package com.gilgamesh.maildirt

import java.io.File
import java.net.HttpURLConnection
import java.net.URL
import java.net.URLEncoder
import java.nio.file.Files
import java.nio.file.StandardCopyOption
import java.nio.file.attribute.PosixFilePermission
import java.time.LocalDateTime
import java.time.ZoneOffset
import java.time.format.DateTimeFormatter
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.jsonObject

class GmailTokenProvider(private val identity: SenderIdentity, private val passphrase: String) {
    private val file = File(identity.tokenFilePath)

    fun accessToken(): String {
        if (!file.exists()) throw IllegalStateException("Token file not found: ${file.path}")
        val root = Json.parseToJsonElement(GpgToken.decrypt(file, passphrase)).jsonObject
        val token = root.string("access_token")
        val expires = parseExpiration(root.string("access_token_expiration"))
        val now = LocalDateTime.now(ZoneOffset.UTC)
        if (!token.isNullOrBlank() && expires != null && expires.isAfter(now.plusSeconds(60))) {
            return token
        }

        val clientId = root.string("client_id")
            ?: throw IllegalStateException("Token file has no client_id")
        val clientSecret = root.string("client_secret")
            ?: throw IllegalStateException("Token file has no client_secret")
        val refreshToken = root.string("refresh_token")
            ?: throw IllegalStateException("Token file has no refresh_token")

        val response = refresh(clientId, clientSecret, refreshToken)
        val newToken = response.string("access_token")
            ?: throw IllegalStateException("Google did not return an access token")
        val expiresIn = response.string("expires_in")?.toLongOrNull() ?: 3600L

        val updated = JsonObject(
            root.toMutableMap().apply {
                put("access_token", JsonPrimitive(newToken))
                put(
                    "access_token_expiration",
                    JsonPrimitive(
                        LocalDateTime.now(ZoneOffset.UTC)
                            .plusSeconds(expiresIn)
                            .format(EXPIRATION_FORMAT),
                    ),
                )
                response.string("refresh_token")?.let { put("refresh_token", JsonPrimitive(it)) }
            },
        )
        replaceFile(Json.encodeToString(JsonObject.serializer(), updated))
        return newToken
    }

    private fun refresh(clientId: String, clientSecret: String, refreshToken: String): JsonObject {
        val body = listOf(
            "client_id" to clientId,
            "client_secret" to clientSecret,
            "refresh_token" to refreshToken,
            "grant_type" to "refresh_token",
        ).joinToString("&") { (key, value) ->
            "${URLEncoder.encode(key, "UTF-8")}=${URLEncoder.encode(value, "UTF-8")}"
        }

        val connection = (URL("https://oauth2.googleapis.com/token").openConnection() as HttpURLConnection)
        try {
            connection.requestMethod = "POST"
            connection.doOutput = true
            connection.connectTimeout = 20000
            connection.readTimeout = 30000
            connection.setRequestProperty("Content-Type", "application/x-www-form-urlencoded")
            connection.outputStream.use { it.write(body.toByteArray(Charsets.UTF_8)) }

            val code = connection.responseCode
            val stream = if (code in 200..299) connection.inputStream else connection.errorStream
            val text = stream?.bufferedReader(Charsets.UTF_8)?.use { it.readText() }.orEmpty()
            if (code !in 200..299) {
                val error = runCatching {
                    val obj = Json.parseToJsonElement(text).jsonObject
                    obj.string("error_description") ?: obj.string("error")
                }.getOrNull()
                throw IllegalStateException("Google token refresh failed ($code): ${error ?: text.take(200)}")
            }
            return Json.parseToJsonElement(text).jsonObject
        } finally {
            connection.disconnect()
        }
    }

    private fun replaceFile(content: String) {
        val dir = file.parentFile ?: throw IllegalStateException("Invalid token file path")
        val permissions = runCatching {
            Files.getPosixFilePermissions(file.toPath())
        }.getOrNull()
        val tmp = File(dir, "${file.name}.tmp")
        tmp.writeBytes(GpgToken.encrypt(content, passphrase))
        if (permissions != null) {
            runCatching { Files.setPosixFilePermissions(tmp.toPath(), permissions) }
        }
        runCatching {
            Files.move(
                tmp.toPath(),
                file.toPath(),
                StandardCopyOption.REPLACE_EXISTING,
                StandardCopyOption.ATOMIC_MOVE,
            )
        }.onFailure {
            tmp.delete()
            throw it
        }
    }

    private fun parseExpiration(value: String?): LocalDateTime? {
        if (value.isNullOrBlank()) return null
        val cut = value.trim().replace(' ', 'T').take(19)
        return runCatching { LocalDateTime.parse(cut) }.getOrNull()
    }

    private fun JsonObject.string(key: String): String? =
        this[key]?.let { (it as? JsonPrimitive)?.contentOrNull }

    private companion object {
        val EXPIRATION_FORMAT: DateTimeFormatter =
            DateTimeFormatter.ofPattern("yyyy-MM-dd'T'HH:mm:ss.SSSSSS")
    }
}

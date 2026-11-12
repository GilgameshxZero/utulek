package com.gilgamesh.maildirt

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import java.io.File
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec
import kotlinx.serialization.builtins.MapSerializer
import kotlinx.serialization.builtins.serializer
import kotlinx.serialization.json.Json

/**
 * Stores per-identity secrets (SMTP passwords and token-file passphrases) in a
 * single app-private file encrypted with an Android Keystore key.
 */
class CredentialStore(context: Context) {
    private val file = File(context.filesDir, "credentials.enc")
    private val json = Json
    private val entrySerializer = MapSerializer(String.serializer(), String.serializer())

    fun has(id: String): Boolean = readAll().containsKey(id)

    fun read(id: String): String? = readAll()[id]

    fun write(id: String, secret: String) {
        val all = readAll().toMutableMap()
        all[id] = secret
        saveAll(all)
    }

    fun remove(id: String) {
        val all = readAll()
        if (!all.containsKey(id)) return
        saveAll(all - id)
    }

    private fun readAll(): Map<String, String> {
        if (!file.exists()) return emptyMap()
        return runCatching {
            val bytes = file.readBytes()
            val iv = bytes.copyOf(IV_SIZE)
            val body = bytes.copyOfRange(IV_SIZE, bytes.size)
            val cipher = Cipher.getInstance(TRANSFORMATION)
            cipher.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, iv))
            val plain = cipher.doFinal(body)
            json.decodeFromString(entrySerializer, String(plain, Charsets.UTF_8))
        }.getOrDefault(emptyMap())
    }

    private fun saveAll(all: Map<String, String>) {
        val cipher = Cipher.getInstance(TRANSFORMATION)
        cipher.init(Cipher.ENCRYPT_MODE, key())
        val plain = json.encodeToString(entrySerializer, all).toByteArray(Charsets.UTF_8)
        val body = cipher.doFinal(plain)
        val bytes = cipher.iv + body
        val tmp = File(file.parentFile, "${file.name}.tmp")
        tmp.writeBytes(bytes)
        if (!tmp.renameTo(file)) {
            file.delete()
            check(tmp.renameTo(file)) { "Could not write credential store" }
        }
    }

    private fun key(): SecretKey = synchronized(KEY_LOCK) {
        val store = KeyStore.getInstance(KEYSTORE).apply { load(null) }
        val existing = store.getEntry(KEY_ALIAS, null) as? KeyStore.SecretKeyEntry
        if (existing != null) return existing.secretKey

        val generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, KEYSTORE)
        generator.init(
            KeyGenParameterSpec.Builder(
                KEY_ALIAS,
                KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT,
            )
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(256)
                .build(),
        )
        generator.generateKey()
    }

    private companion object {
        const val KEYSTORE = "AndroidKeyStore"
        const val KEY_ALIAS = "maildirt-credentials"
        const val TRANSFORMATION = "AES/GCM/NoPadding"
        const val IV_SIZE = 12
        val KEY_LOCK = Any()
    }
}

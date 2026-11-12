package com.gilgamesh.smsgatec

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import kotlinx.serialization.encodeToString
import java.io.File
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/**
 * Gateway passwords are kept out of config.json and encrypted with a
 * device-bound Android Keystore key. Nothing is copied between devices.
 */
class CredentialStore(context: Context) {
    private val file = File(context.filesDir, "credentials.enc")
    private val alias = "smsgatec-credentials"

    private fun key(): SecretKey {
        val store = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        (store.getEntry(alias, null) as? KeyStore.SecretKeyEntry)?.let { return it.secretKey }
        val generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore")
        generator.init(
            KeyGenParameterSpec.Builder(
                alias,
                KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT,
            )
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(256)
                .build()
        )
        return generator.generateKey()
    }

    private fun loadAll(): MutableMap<String, String> {
        if (!file.exists()) return mutableMapOf()
        return try {
            json.decodeFromString<Map<String, String>>(file.readText()).toMutableMap()
        } catch (e: Exception) {
            mutableMapOf()
        }
    }

    private fun saveAll(values: Map<String, String>) {
        file.parentFile?.mkdirs()
        val temp = File(file.parentFile, file.name + ".tmp")
        temp.writeText(json.encodeToString(values))
        if (!temp.renameTo(file)) {
            file.writeText(temp.readText())
            temp.delete()
        }
    }

    @Synchronized
    fun get(id: String): String? {
        val stored = loadAll()[id] ?: return null
        return try {
            val raw = Base64.decode(stored, Base64.NO_WRAP)
            val iv = raw.copyOfRange(0, 12)
            val body = raw.copyOfRange(12, raw.size)
            val cipher = Cipher.getInstance("AES/GCM/NoPadding")
            cipher.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, iv))
            String(cipher.doFinal(body), Charsets.UTF_8)
        } catch (e: Exception) {
            null
        }
    }

    @Synchronized
    fun set(id: String, secret: String) {
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.ENCRYPT_MODE, key())
        val body = cipher.doFinal(secret.toByteArray(Charsets.UTF_8))
        val raw = cipher.iv + body
        val values = loadAll()
        values[id] = Base64.encodeToString(raw, Base64.NO_WRAP)
        saveAll(values)
    }

    @Synchronized
    fun remove(id: String) {
        val values = loadAll()
        if (values.remove(id) != null) saveAll(values)
    }
}

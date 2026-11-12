package com.gilgamesh.smsgatec

import android.content.Context
import kotlinx.serialization.encodeToString
import java.io.File
import java.util.UUID

class ConfigStore(context: Context) {
    private val file = File(context.filesDir, "config.json")
    private val temp = File(context.filesDir, "config.json.tmp")

    fun load(): Config {
        val config = if (file.exists()) {
            try {
                json.decodeFromString<Config>(file.readText())
            } catch (e: Exception) {
                Config()
            }
        } else {
            Config()
        }
        val repaired = repair(config)
        if (repaired != config) save(repaired)
        return repaired
    }

    fun save(config: Config) {
        val text = json.encodeToString(config)
        temp.writeText(text)
        if (!temp.renameTo(file)) {
            file.writeText(text)
            temp.delete()
        }
    }

    fun newAccount(name: String, server: String, username: String): Account =
        Account(id = UUID.randomUUID().toString().take(8), name = name, server = server, username = username)

    private fun repair(config: Config): Config {
        val accounts = config.accounts
            .filter { it.id.isNotBlank() && it.name.isNotBlank() }
            .map { it.copy(server = it.server.trim().trimEnd('/')) }
        val active = accounts.firstOrNull { it.id == config.activeAccount }?.id
            ?: accounts.firstOrNull()?.id
            ?: ""
        return config.copy(
            accounts = accounts,
            activeAccount = active,
            updateMode = if (config.updateMode in setOf("push", "poll")) config.updateMode else "poll",
            pollSeconds = config.pollSeconds.coerceIn(15, 3600),
            reconcileSeconds = config.reconcileSeconds.coerceIn(60, 86400),
            timeoutSeconds = config.timeoutSeconds.coerceIn(3, 120),
            theme = if (config.theme in setOf("auto", "light", "dark")) config.theme else "auto",
        )
    }
}

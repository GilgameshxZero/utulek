package com.gilgamesh.maildirt

import android.content.Context
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.io.File

@Serializable
enum class FolderRole { INBOX, SENT, DRAFTS, ARCHIVE, OTHER }

@Serializable
data class MaildirFolder(
    val id: String,
    val path: String,
    val role: FolderRole = FolderRole.OTHER,
)

val MaildirFolder.displayName: String
    get() {
        val base = File(path).name
        return base.removeSuffix(".maildir").ifBlank { base }.ifBlank { path }
    }

@Serializable
data class AppConfig(
    val folders: List<MaildirFolder> = emptyList(),
    val selectedFolderId: String? = null,
)

class ConfigStore(context: Context) {
    private val file = File(context.filesDir, "config.json")
    private val json = Json { ignoreUnknownKeys = true; prettyPrint = true }

    fun load(): AppConfig = runCatching {
        if (file.exists()) json.decodeFromString(AppConfig.serializer(), file.readText())
        else AppConfig()
    }.getOrDefault(AppConfig())

    fun save(config: AppConfig) {
        file.parentFile?.mkdirs()
        file.writeText(json.encodeToString(AppConfig.serializer(), config))
    }
}

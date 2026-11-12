package com.gilgamesh.maildirt

import android.content.Context
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.io.File

@Serializable
enum class FolderRole { INBOX, POSTPONED, ARCHIVE, OTHER }

@Serializable
enum class AuthMode { STANDARD, XOAUTH2 }

@Serializable
enum class SmtpSecurity { STARTTLS, SSL, NONE }

@Serializable
enum class MimePreference { PLAIN, HTML }

@Serializable
data class SenderIdentity(
    val id: String,
    val email: String,
    val displayName: String = "",
    val signature: String = "",
    val authMode: AuthMode = AuthMode.STANDARD,
    val smtpHost: String = "",
    val smtpPort: Int = 587,
    val security: SmtpSecurity = SmtpSecurity.STARTTLS,
    val username: String = "",
    val tokenFilePath: String = "",
)

val SenderIdentity.displayLabel: String
    get() = if (displayName.isBlank()) email else "$displayName <$email>"

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
    val pagedScrolling: Boolean = true,
    val searchBody: Boolean = false,
    val identities: List<SenderIdentity> = emptyList(),
    val defaultIdentityId: String? = null,
    val defaultReplyAll: Boolean = true,
    val configVersion: Int = 0,
    val notifySound: Boolean = true,
    val notifyShowContent: Boolean = true,
    val notifyPollSeconds: Int = 3600,
    val defaultMime: MimePreference = MimePreference.PLAIN,
)

class ConfigStore(context: Context) {
    private val file = File(context.filesDir, "config.json")
    private val json = Json { ignoreUnknownKeys = true; prettyPrint = true }

    fun load(): AppConfig {
        val loaded = runCatching {
            if (file.exists()) json.decodeFromString(AppConfig.serializer(), file.readText())
            else AppConfig()
        }.getOrDefault(AppConfig())
        if (loaded.configVersion < 1) {
            val migrated = loaded.copy(defaultReplyAll = true, configVersion = 1)
            runCatching { save(migrated) }
            return migrated
        }
        return loaded
    }

    fun save(config: AppConfig) {
        file.parentFile?.mkdirs()
        file.writeText(json.encodeToString(AppConfig.serializer(), config))
    }
}

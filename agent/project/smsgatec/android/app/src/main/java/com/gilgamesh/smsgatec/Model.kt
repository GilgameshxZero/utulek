package com.gilgamesh.smsgatec

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.security.MessageDigest
import java.security.SecureRandom
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.util.Base64

const val APP_VERSION = "0.3.1"

val json = Json {
    ignoreUnknownKeys = true
    encodeDefaults = true
    explicitNulls = false
    isLenient = true
}

@Serializable
data class Account(
    val id: String = "",
    val name: String = "",
    val server: String = "",
    val username: String = "",
)

@Serializable
data class Config(
    val accounts: List<Account> = emptyList(),
    val activeAccount: String = "",
    val updateMode: String = "push",
    val reconcileSeconds: Int = 3600,
    val pollSeconds: Int = 60,
    val timeoutSeconds: Int = 20,
    val cacheEnabled: Boolean = true,
    val theme: String = "auto",
    val pagedScrolling: Boolean = true,
) {
    fun active(): Account? = accounts.firstOrNull { it.id == activeAccount } ?: accounts.firstOrNull()
    fun validate(): String? {
        if (accounts.isEmpty()) return "Add a gateway account"
        val ids = mutableSetOf<String>()
        for (a in accounts) {
            if (a.id.isBlank() || a.name.isBlank()) return "Accounts need a name"
            if (!ids.add(a.id)) return "Duplicate account"
            if (a.server.isBlank() || a.username.isBlank()) return "Server and username are required"
            val url = try { java.net.URI(a.server) } catch (e: Exception) { null }
                ?: return "Use an HTTP(S) server address"
            if (url.host.isNullOrBlank() || (url.scheme != "http" && url.scheme != "https") ||
                url.userInfo != null || url.query != null || url.fragment != null
            ) return "Use an HTTP(S) server address with credentials in their own fields"
        }
        if (updateMode !in setOf("push", "poll")) return "Update mode must be push or poll"
        if (reconcileSeconds !in 60..86400) return "Reconcile must be 60 to 86400 seconds"
        if (pollSeconds !in 15..3600) return "Poll must be 15 to 3600 seconds"
        if (timeoutSeconds !in 3..120) return "Timeout must be 3 to 120 seconds"
        if (theme !in setOf("auto", "light", "dark")) return "Theme must be auto, light, or dark"
        return null
    }
}

@Serializable
data class Attachment(
    val partId: Long = 0,
    val name: String = "",
    val contentType: String = "",
    val size: Long = 0,
    val localPath: String = "",
)

@Serializable
data class Message(
    val id: String = "",
    val number: String = "",
    val direction: String = "",
    val text: String = "",
    val timestamp: String = "",
    val state: String = "",
    val type: String = "",
    val media: List<Attachment> = emptyList(),
    val subject: String = "",
    val groupId: String = "",
    val groupName: String = "",
    val participants: List<String> = emptyList(),
    val sender: String = "",
) {
    val key: String get() = "$direction:$id:$number"
    val conversationKey: String get() = groupId.ifEmpty { number }
    val isGroup: Boolean get() = groupId.isNotEmpty()
    fun stamp(): Instant? = try { Instant.parse(timestamp) } catch (e: Exception) { null }
    fun dateLabel(): String {
        val s = stamp() ?: return "date unknown"
        return DATE_FORMAT.format(s.atZone(ZoneId.systemDefault()))
    }
}

private val DATE_FORMAT = DateTimeFormatter.ofPattern("yyyy/MM/dd HH:mm")

data class Contact(val name: String, val kind: String = "")

private val PHONE = Regex("^\\+[1-9][0-9]{6,14}$")

fun digits(value: String): String = value.filter { it in '0'..'9' }

fun phoneKey(value: String): String {
    var d = digits(value)
    if (value.startsWith("00")) d = d.removePrefix("00")
    if (d.length >= 7) return "+" + d
    return if (d.isNotEmpty()) d else value
}

fun recipient(value: String): String? {
    var v = value.filterNot { it.isWhitespace() || it in "().-" }
    if (v.startsWith("00")) v = "+" + v.substring(2)
    if (!PHONE.matches(v)) return null
    return v
}

fun cleanText(value: String): String = value.filter { it == '\n' || it == '\t' || !it.isISOControl() }

fun safeName(name: String, contentType: String): String {
    var n = name.replace('\\', '_').substringAfterLast('/').substringAfterLast('\\')
    n = cleanText(n).trim()
    if (n.isEmpty() || n == "." || n.startsWith(".")) n = "attachment"
    if (!n.contains('.')) {
        val ext = when {
            contentType.contains("jpeg") -> ".jpg"
            contentType.contains("png") -> ".png"
            contentType.contains("gif") -> ".gif"
            contentType.contains("webp") -> ".webp"
            contentType.contains("pdf") -> ".pdf"
            contentType.contains("text/plain") -> ".txt"
            contentType.contains("mp4") -> ".mp4"
            contentType.contains("3gpp") -> ".3gp"
            contentType.contains("audio/") -> ".m4a"
            else -> ""
        }
        n += ext
    }
    return n
}

fun messageID(): String {
    val data = ByteArray(15)
    SecureRandom().nextBytes(data)
    return Base64.getUrlEncoder().withoutPadding().encodeToString(data)
}

fun stateRank(state: String): Int = when (state) {
    "Pending" -> 1
    "Processed" -> 2
    "Sent" -> 3
    "Delivered" -> 4
    else -> 0
}

/** Same merge rules as the terminal client: content is immutable, metadata may refresh. */
fun mergeMessage(old: Message, next: Message): Message {
    var m = next
    if (next.direction == "out") {
        if (stateRank(old.state) > stateRank(next.state) && stateRank(next.state) > 0) {
            m = m.copy(state = old.state)
        }
        if (next.text == "[Text unavailable]") m = m.copy(text = old.text)
        if (old.type.startsWith("MMS") && next.type == "SMS") {
            m = m.copy(type = old.type, media = old.media)
        }
    }
    if (next.subject.isEmpty()) m = m.copy(subject = old.subject)
    if (next.groupId.isEmpty()) m = m.copy(groupId = old.groupId)
    if (next.groupName.isEmpty()) m = m.copy(groupName = old.groupName)
    if (next.participants.isEmpty()) m = m.copy(participants = old.participants)
    if (next.sender.isEmpty()) m = m.copy(sender = old.sender)
    if (m.media.isEmpty()) return m.copy(media = emptyList())
    val media = m.media.map { part ->
        val match = old.media.firstOrNull {
            it.partId == part.partId && it.name == part.name && it.contentType == part.contentType
        }
        if (match != null && part.localPath.isEmpty()) part.copy(localPath = match.localPath) else part
    }
    return m.copy(media = media)
}

// ---- Gateway payloads ----

@Serializable
data class ApiRecipient(val phoneNumber: String = "", val state: String = "")

@Serializable
data class ApiAttachment(
    val partId: Long = 0,
    val name: String = "",
    val contentType: String = "",
    val size: Long = 0,
)

@Serializable
data class WireAttachment(
    val contentType: String = "",
    val name: String = "",
    val data: String = "",
)

@Serializable
data class ApiMms(
    val subject: String = "",
    val text: String = "",
    val attachments: List<WireAttachment> = emptyList(),
)

@Serializable
data class ApiText(val text: String = "")

@Serializable
data class ApiMessage(
    val id: String = "",
    val createdAt: String = "",
    val sender: String = "",
    val contentPreview: String = "",
    val state: String = "",
    val isEncrypted: Boolean = false,
    val recipients: List<ApiRecipient> = emptyList(),
    val type: String = "",
    val attachments: List<ApiAttachment> = emptyList(),
    val mmsMessage: ApiMms? = null,
    val textMessage: ApiText? = null,
    val groupId: String = "",
    val participants: List<String> = emptyList(),
) {
    fun messages(): List<Message> {
        val group = groupId.isNotEmpty()
        val subject = mmsMessage?.subject.orEmpty()
        if (sender.isNotEmpty()) {
            val incomingText = if (group && !mmsMessage?.text.isNullOrEmpty()) {
                mmsMessage?.text.orEmpty()
            } else {
                contentPreview
            }
            return listOf(
                Message(
                    id = id,
                    number = if (group) groupId else phoneKey(sender),
                    direction = "in",
                    text = incomingText,
                    timestamp = createdAt,
                    state = "Received",
                    type = if (group) "MMS" else type,
                    media = attachments.toMedia(),
                    subject = subject,
                    groupId = groupId,
                    participants = participants,
                    sender = if (group) phoneKey(sender) else "",
                )
            )
        }
        var text = textMessage?.text ?: "[Text unavailable]"
        var kind = "SMS"
        var media = attachments.toMedia()
        if (mmsMessage != null) {
            kind = "MMS"
            text = mmsMessage.text
            if (media.isEmpty()) {
                media = mmsMessage.attachments.map {
                    Attachment(name = it.name, contentType = it.contentType)
                }
            }
        }
        if (isEncrypted) text = "[Encrypted message]"
        val recipientState = state.ifEmpty {
            recipients.firstOrNull { it.state.isNotEmpty() }?.state.orEmpty()
        }
        if (group) {
            return listOf(
                Message(
                    id = id, number = groupId, direction = "out", text = text,
                    timestamp = createdAt, state = recipientState.ifEmpty { "Pending" },
                    type = "MMS", media = media, subject = subject,
                    groupId = groupId, participants = participants,
                )
            )
        }
        if (recipients.size > 1) {
            val numbers = recipients.map { phoneKey(it.phoneNumber) }
            val key = localGroupKey(numbers)
            return listOf(
                Message(
                    id = id, number = key, direction = "out", text = text,
                    timestamp = createdAt, state = recipientState.ifEmpty { "Pending" },
                    type = if (mmsMessage != null) "MMS" else kind, media = media,
                    subject = subject, groupId = key,
                    participants = numbers,
                )
            )
        }
        return recipients.map { r ->
            Message(
                id = id, number = phoneKey(r.phoneNumber), direction = "out",
                text = text, timestamp = createdAt,
                state = r.state.ifEmpty { state }, type = kind, media = media,
                subject = subject,
            )
        }
    }
}

fun List<ApiAttachment>.toMedia(): List<Attachment> = map {
    Attachment(partId = it.partId, name = it.name, contentType = it.contentType, size = it.size)
}

@Serializable
data class PushEvent(
    val messages: List<ApiMessage> = emptyList(),
    val id: String = "",
    val event: String = "",
    val payload: PushPayload = PushPayload(),
)

@Serializable
data class PushPayload(
    val messageId: String = "",
    val receivedAt: String = "",
    val sender: String = "",
    val message: String = "",
    val body: String = "",
    val subject: String = "",
    val transactionId: String = "",
    val attachments: List<PushPart> = emptyList(),
)

@Serializable
data class PushPart(
    val partId: Long = 0,
    val name: String = "",
    val contentType: String = "",
    val size: Long = 0,
    val data: String = "",
)

@Serializable
data class Health(
    val backend: String = "",
    val capabilities: List<String> = emptyList(),
    val authentication: String = "",
    val ready: Boolean = false,
    val number: String = "",
)

fun List<Message>.sortedForThread(): List<Message> = sortedWith(
    compareBy<Message> { it.stamp() ?: Instant.EPOCH }.thenBy { it.key }
)

fun latestByConversation(records: Collection<Message>): Map<String, Message> {
    val latest = mutableMapOf<String, Message>()
    for (m in records) {
        val key = m.conversationKey
        val old = latest[key]
        val stamp = m.stamp() ?: Instant.EPOCH
        val oldStamp = old?.stamp() ?: Instant.EPOCH
        if (old == null || stamp.isAfter(oldStamp) || (stamp == oldStamp && m.key > old.key)) {
            latest[key] = m
        }
    }
    return latest
}

/** Stable key for a group before the gateway assigns a group ID. */
fun localGroupKey(numbers: List<String>): String {
    val normalized = participantKey(numbers)
    val digest = MessageDigest.getInstance("SHA-256")
        .digest(normalized.toByteArray())
        .take(8)
        .joinToString("") { "%02x".format(it) }
    return "local:$digest"
}

fun participantKey(numbers: List<String>): String =
    numbers.map { digits(it) }.filter { it.isNotEmpty() }.distinct().sorted().joinToString(",")

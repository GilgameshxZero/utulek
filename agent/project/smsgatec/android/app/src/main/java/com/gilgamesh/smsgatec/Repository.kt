package com.gilgamesh.smsgatec

import android.content.Context
import kotlinx.serialization.Serializable
import java.io.File
import java.security.MessageDigest
import java.time.Instant

@Serializable
private data class SyncCursor(val since: String = "")

data class PendingAttachment(
    val name: String,
    val contentType: String,
    val size: Long,
    val bytes: ByteArray,
    val localPath: String = "",
) {
    override fun equals(other: Any?): Boolean = this === other
    override fun hashCode(): Int = System.identityHashCode(this)
}

class SendFailure(message: String) : Exception(message)

/**
 * Owns the active account's gateway, message cache, contacts and sync cursor.
 * Account switches reuse the same terminal-client semantics: only the selected
 * account connects, and each account has its own cache and cursor.
 */
class Repository(private val context: Context) {
    val configStore = ConfigStore(context)
    private val credentials = CredentialStore(context)
    val contactBook = ContactBook(context)

    var config: Config = configStore.load()
        private set
    private var store: MessageStore = openStore()
    var gateway: Gateway? = null
        private set
    var lastSync: Instant? = null
        private set
    var connected: Boolean = false

    init {
        gateway = buildGateway()
    }

    private fun accountHash(account: Account): String {
        val digest = MessageDigest.getInstance("SHA-256")
            .digest((account.server + "\u0000" + account.username).toByteArray())
        return digest.take(6).joinToString("") { "%02x".format(it) }
    }

    private fun openStore(): MessageStore {
        val account = config.active()
        val name = if (account == null) "messages.db" else "messages-${accountHash(account)}.db"
        return MessageStore(context, config.cacheEnabled, name)
    }

    private fun buildGateway(): Gateway? {
        val account = config.active() ?: return null
        val password = credentials.get(account.id) ?: return null
        return Gateway(account.server, account.username, password, config.timeoutSeconds)
    }

    fun activeAccount(): Account? = config.active()

    fun password(id: String): String? = credentials.get(id)

    fun storePassword(id: String, password: String) = credentials.set(id, password)

    fun removePassword(id: String) = credentials.remove(id)

    fun replaceConfig(next: Config) {
        val activeChanged = next.activeAccount != config.activeAccount
        config = next
        configStore.save(next)
        if (activeChanged) reloadActive()
    }

    fun reloadActive() {
        gateway?.close()
        store.close()
        store = openStore()
        gateway = buildGateway()
        lastSync = null
        connected = false
    }

    fun messages(): List<Message> = store.all()

    fun messageCount(): Int = store.size()

    fun contacts(query: String = ""): Map<String, Contact> = contactBook.read(query)

    fun close() {
        gateway?.close()
        gateway = null
        store.close()
    }

    private fun cursorFile(): File? {
        val account = config.active() ?: return null
        return File(context.filesDir, "sync-${accountHash(account)}.json")
    }

    private fun readCursor(): Instant? {
        val file = cursorFile() ?: return null
        if (!file.exists() || store.size() == 0) return null
        return try {
            val cursor = json.decodeFromString<SyncCursor>(file.readText())
            Instant.parse(cursor.since).takeIf { !it.isAfter(Instant.now()) }
        } catch (e: Exception) {
            null
        }
    }

    private fun saveCursor(since: Instant) {
        val file = cursorFile() ?: return
        try {
            file.writeText(json.encodeToString(SyncCursor.serializer(), SyncCursor(since.toString())))
        } catch (e: Exception) {
            // The next start reconciles history again.
        }
    }

    private fun knownOutgoing(): Map<String, Message> {
        val result = mutableMapOf<String, Message>()
        for (m in store.all()) {
            if (m.direction == "out" && !result.containsKey(m.id)) result[m.id] = m
        }
        return result
    }

    data class SyncOutcome(
        val added: Int,
        val total: Int,
        val contacts: Map<String, Contact>,
        val adopted: Map<String, String> = emptyMap(),
    )

    fun sync(full: Boolean): SyncOutcome {
        val gateway = gateway ?: throw SendFailure("Configure a gateway account first")
        val since = if (full) null else readCursor()
        val started = Instant.now()
        val messages = gateway.syncSince(knownOutgoing(), since)
        val adopted = adoptLocalGroups(messages)
        val migrated = migrateLegacyGroups(messages)
        val added = store.put(migrated.filterNot { it.conversationKey in adopted.keys })
        store.trim()
        lastSync = started
        saveCursor(started)
        connected = true
        return SyncOutcome(added, store.size(), contacts(), adopted)
    }

    fun refreshContacts(): Map<String, Contact> = contacts()

    /** Downloads and caches one attachment; returns its local file path. */
    fun ensureMedia(message: Message, part: Attachment): String {
        val existing = part.localPath
        if (existing.isNotEmpty() && File(existing).exists()) return existing
        if (message.direction != "in") {
            throw SendFailure("Outgoing attachment was not cached on this device")
        }
        val gateway = gateway ?: throw SendFailure("Configure a gateway account first")
        val data = gateway.download(message.id, part)
        val path = store.storeMedia(part, data)
        val updated = message.copy(
            media = message.media.map { if (it == part) it.copy(localPath = path, size = data.size.toLong()) else it }
        )
        store.put(listOf(updated))
        connected = true
        return path
    }

    fun send(
        numbers: List<String>,
        text: String,
        subject: String,
        attachments: List<PendingAttachment>,
    ): Message {
        val gateway = gateway ?: throw SendFailure("Configure a gateway account first")
        val targets = numbers.map {
            recipient(it) ?: throw SendFailure("Include the country code, e.g. +12025550123")
        }.distinct()
        if (targets.isEmpty()) throw SendFailure("Add a recipient")
        if (targets.size > 10) throw SendFailure("A group can have at most 10 recipients")
        if (text.isBlank() && attachments.isEmpty()) throw SendFailure("Write a message first")
        var total = 0L
        for (file in attachments) {
            total += file.size
            if (total > 20L * 1024 * 1024) throw SendFailure("Combined attachments exceed 20 MiB")
        }
        val group = targets.size > 1
        val key = if (group) localGroupKey(targets) else targets[0]
        val id = messageID()
        val kind = if (group || attachments.isNotEmpty() || subject.isNotBlank()) "MMS" else "SMS"
        val media = attachments.map { file ->
            Attachment(
                name = safeName(file.name, file.contentType),
                contentType = file.contentType,
                size = file.size,
                localPath = store.storeMedia(
                    Attachment(name = file.name, contentType = file.contentType), file.bytes,
                ),
            )
        }
        val optimistic = Message(
            id = id, number = key, direction = "out", text = text,
            timestamp = Instant.now().toString(), state = "Uncertain",
            type = kind, media = media, subject = subject,
            groupId = if (group) key else "",
            participants = if (group) targets else emptyList(),
        )
        store.put(listOf(optimistic))
        try {
            val outcome = gateway.send(
                targets, id, text, subject,
                attachments.map {
                    WireAttachment(
                        contentType = it.contentType,
                        name = safeName(it.name, it.contentType),
                        data = java.util.Base64.getEncoder().encodeToString(it.bytes),
                    )
                },
            )
            val state = outcome.state.ifEmpty { "Pending" }
            if (group && outcome.groupId.isNotEmpty()) {
                val migrated = optimistic.copy(
                    state = state,
                    number = outcome.groupId,
                    groupId = outcome.groupId,
                )
                store.delete(optimistic.key)
                store.put(listOf(migrated))
                lastSync = Instant.now()
                connected = true
                return migrated
            }
            val sent = optimistic.copy(state = state)
            store.put(listOf(sent))
            lastSync = Instant.now()
            connected = true
            return sent
        } catch (e: Exception) {
            val uncertain = (e as? ApiError)?.uncertain ?: true
            val failed = optimistic.copy(state = if (uncertain) "Uncertain" else "Rejected")
            store.put(listOf(failed))
            connected = false
            throw SendFailure(e.message ?: "Send failed")
        }
    }

    /**
     * A freshly sent group is keyed locally until the gateway returns its group
     * ID. Adopt the local thread into the real group once history reports it.
     */
    private fun adoptLocalGroups(incoming: List<Message>): Map<String, String> {
        val adopted = mutableMapOf<String, String>()
        val locals = store.all()
            .filter { it.direction == "out" && it.groupId.startsWith("local:") }
            .toMutableList()
        for (real in incoming) {
            if (real.direction != "out" || real.groupId.isEmpty() || real.groupId.startsWith("local:")) continue
            val match = locals.firstOrNull { local ->
                participantKey(local.participants) == participantKey(real.participants) &&
                    (local.text == real.text || (local.subject.isNotEmpty() && local.subject == real.subject)) &&
                    closeInTime(local, real)
            } ?: continue
            store.delete(match.key)
            adopted[match.groupId] = real.groupId
            locals.remove(match)
        }
        return adopted
    }

    /** Removes pre-0.3.0 1:1 copies and duplicate records of gateway group messages. */
    private fun migrateLegacyGroups(incoming: List<Message>): List<Message> {
        val snapshot = store.all()
        val deleted = mutableSetOf<String>()
        return incoming.map { message ->
            if (!message.isGroup) return@map message
            var merged = message
            for (old in snapshot) {
                if (old.key == message.key || old.direction != message.direction) continue
                val sameId = old.id == message.id
                if (!sameId && !sameLogicalGroupSend(old, message)) continue
                if (deleted.add(old.key)) {
                    store.delete(old.key)
                    merged = mergeMessage(old, merged)
                }
            }
            merged
        }
    }

    private fun sameLogicalGroupSend(old: Message, incoming: Message): Boolean {
        if (old.text != incoming.text && (old.subject.isEmpty() || old.subject != incoming.subject)) {
            return false
        }
        if (!closeInTime(old, incoming, windowSeconds = 120)) return false
        if (old.isGroup) {
            return participantKey(old.participants) == participantKey(incoming.participants)
        }
        return participantKey(incoming.participants).split(",").contains(digits(old.number))
    }

    private fun closeInTime(a: Message, b: Message, windowSeconds: Long = 30 * 60): Boolean {
        val left = a.stamp() ?: return true
        val right = b.stamp() ?: return true
        return kotlin.math.abs(java.time.Duration.between(left, right).seconds) <= windowSeconds
    }

    data class EventOutcome(
        val messages: List<Message>,
        val fullSync: Boolean,
        val adopted: Map<String, String> = emptyMap(),
    )

    /** Converts authenticated bridge events into message records. */
    fun eventMessages(events: List<PushEvent>): EventOutcome {
        val gateway = gateway ?: throw SendFailure("Configure a gateway account first")
        val result = mutableListOf<Message>()
        var fullSync = false
        val ids = mutableSetOf<String>()
        var since: Instant? = null
        for (event in events) {
            when (event.event) {
                "bridge:reset" -> fullSync = true
                "bridge:messages" -> event.messages.forEach { result.addAll(it.messages()) }
                else -> {
                    val incoming = event.incoming()
                    if (incoming != null) {
                        var message = incoming
                        if (message.media.isNotEmpty()) {
                            message = message.copy(
                                media = message.media.map { part ->
                                    val wire = event.payload.attachments.firstOrNull { it.partId == part.partId }
                                    val bytes = wire?.let { decodeWireAttachment(it) }
                                    if (bytes != null) part.copy(localPath = store.storeMedia(part, bytes), size = bytes.size.toLong())
                                    else part
                                }
                            )
                        }
                        result.add(message)
                    }
                    if (event.event == "sms:received" || event.event.startsWith("mms:")) {
                        val parsed = try { Instant.parse(event.payload.receivedAt) } catch (e: Exception) { null }
                        if (parsed != null) {
                            val from = parsed.minusSeconds(120)
                            if (since == null || from.isBefore(since)) since = from
                        }
                    } else if (event.event.startsWith("sms:") && event.payload.messageId.isNotEmpty()) {
                        ids.add(event.payload.messageId)
                    }
                }
            }
        }
        if (since != null) {
            result.addAll(gateway.syncSince(emptyMap(), since))
        }
        for (id in ids) {
            try {
                result.addAll(gateway.message(id).messages())
            } catch (e: ApiError) {
                if (e.status != 404) throw e
            }
        }
        val adopted = adoptLocalGroups(result)
        val migrated = migrateLegacyGroups(result)
        store.put(migrated.filterNot { it.conversationKey in adopted.keys })
        connected = true
        return EventOutcome(result, fullSync, adopted)
    }

    fun checkConnection(account: Account, password: String): String {
        val gateway = Gateway(account.server, account.username, password, config.timeoutSeconds)
        return try {
            val health = gateway.health()
            val parts = mutableListOf("Connected")
            if (health.backend.isNotEmpty()) parts.add(health.backend)
            if (health.number.isNotEmpty()) parts.add(health.number)
            if (health.capabilities.isNotEmpty()) parts.add(health.capabilities.joinToString(","))
            parts.joinToString(" · ")
        } finally {
            gateway.close()
        }
    }

    fun supportsEventStream(): Boolean {
        val gateway = gateway ?: return false
        return try {
            val health = gateway.health()
            connected = true
            health.backend == "google_voice" && health.capabilities.contains("event-stream")
        } catch (e: Exception) {
            connected = false
            false
        }
    }
}

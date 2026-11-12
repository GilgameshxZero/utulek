package com.gilgamesh.smsgatec

import android.app.Application
import android.net.Uri
import android.provider.OpenableColumns
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.File
import java.time.Instant
import java.util.Locale

sealed class Screen {
    data object Conversations : Screen()
    data class Thread(val number: String, val newThread: Boolean = false) : Screen()
    data class AttachmentView(val messageKey: String, val partIndex: Int) : Screen()
    data object Settings : Screen()
    data class AccountEditor(val id: String?) : Screen()
}

data class ComposeState(
    val number: String = "",
    val recipients: List<String> = emptyList(),
    val text: String = "",
    val subject: String = "",
    val attachments: List<PendingAttachment> = emptyList(),
    val mms: Boolean = false,
    val error: String = "",
    val sending: Boolean = false,
)

data class ThreadRow(
    val number: String,
    val name: String,
    val snippet: String,
    val time: String,
    val state: String,
    val hasMedia: Boolean,
)

data class ContactRow(val number: String, val name: String, val kind: String)

data class AttachmentUi(
    val message: Message,
    val part: Attachment,
    val path: String? = null,
    val busy: Boolean = false,
    val error: String = "",
)

data class UiState(
    val screen: Screen = Screen.Conversations,
    val threads: List<ThreadRow> = emptyList(),
    val contactRows: List<ContactRow> = emptyList(),
    val threadMessages: List<Message> = emptyList(),
    val currentNumber: String = "",
    val currentName: String = "",
    val query: String = "",
    val searching: Boolean = false,
    val status: String = "",
    val connected: Boolean = false,
    val lastSync: Instant? = null,
    val accountName: String = "",
    val accounts: List<Account> = emptyList(),
    val config: Config = Config(),
    val compose: ComposeState = ComposeState(),
    val attachment: AttachmentUi? = null,
    val openPath: String? = null,
    val notice: String = "",
    val busy: Boolean = false,
    val messageCount: Int = 0,
    val contactCount: Int = 0,
    val hasAccounts: Boolean = false,
)

class AppViewModel(application: Application) : AndroidViewModel(application) {
    private val repo = Repository(application)
    private var contacts: Map<String, Contact> = emptyMap()
    private var aliases: Set<String> = emptySet()
    private val drafts = mutableMapOf<String, ComposeState>()

    var state by mutableStateOf(UiState())
        private set

    private var updatesJob: Job? = null
    private var queryJob: Job? = null
    private var syncing = false
    private var sending = false
    private var eventChannel: Channel<PushEvent>? = null

    init {
        repo.config.accounts.forEach { account ->
            if (repo.password(account.id).isNullOrEmpty()) {
                notice("Set a password for ${account.name}")
            }
        }
        publish()
        refreshContacts()
        if (repo.activeAccount() != null && repo.password(repo.activeAccount()!!.id) != null) {
            startUpdates()
            syncNow(full = false)
        }
    }

    fun notice(message: String) {
        state = state.copy(status = message, notice = message)
        viewModelScope.launch {
            delay(8000)
            if (state.notice == message) state = state.copy(notice = "", status = "")
        }
    }

    private fun publish(extra: (UiState) -> UiState = { it }) {
        val base = snapshot()
        state = extra(base)
    }

    private fun snapshot(): UiState {
        val records = repo.messages()
        val account = repo.activeAccount()
        val query = state.query
        val searching = state.searching
        val latest = latestByConversation(records)
        val threadRows = latest.map { (key, m) ->
            ThreadRow(
                number = key,
                name = conversationName(key, m),
                snippet = snippet(m),
                time = shortTime(m),
                state = if (m.direction == "out") m.state else "",
                hasMedia = m.media.isNotEmpty(),
            )
        }.sortedWith(compareByDescending<ThreadRow> { latest[it.number]?.stamp() ?: Instant.EPOCH }
            .thenByDescending { it.number })
            .filter { row ->
                !searching || query.isBlank() ||
                    row.name.lowercase(Locale.ROOT).contains(query.lowercase(Locale.ROOT)) ||
                    row.number.contains(query) || aliases.contains(row.number)
            }
        val threadMessages = records.filter { it.conversationKey == state.currentNumber }
            .filter { !searching || query.isBlank() || it.text.lowercase(Locale.ROOT).contains(query.lowercase(Locale.ROOT)) }
            .sortedForThread()
        val current = state.currentNumber
        return state.copy(
            threads = threadRows,
            contactRows = contacts.map { (number, c) -> ContactRow(number, c.name, c.kind) }
                .sortedWith(compareBy({ it.name.lowercase(Locale.ROOT) }, { it.number })),
            threadMessages = threadMessages,
            currentName = if (current.isNotEmpty()) {
                val sample = records.firstOrNull { it.conversationKey == current }
                conversationName(current, sample)
            } else {
                ""
            },
            accountName = account?.name ?: "No account",
            accounts = repo.config.accounts,
            config = repo.config,
            connected = repo.connected,
            lastSync = repo.lastSync,
            messageCount = repo.messageCount(),
            contactCount = contacts.size,
            hasAccounts = repo.config.accounts.isNotEmpty(),
        )
    }

    private fun contactFor(number: String): Contact? {
        contacts[number]?.let { return it }
        val d = digits(number)
        if (d.length >= 10) {
            val tail = d.takeLast(10)
            val matches = contacts.entries.filter { digits(it.key).endsWith(tail) }
            if (matches.size == 1) return matches[0].value
        }
        return null
    }

    private fun conversationName(key: String, sample: Message?): String {
        if (sample?.isGroup == true) {
            if (sample.groupName.isNotBlank()) return sample.groupName
            val names = sample.participants.map { contactFor(it)?.name ?: it }.distinct()
            if (names.isEmpty()) return "Group"
            if (names.size == 1) {
                return if (sample.participants.size <= 1) "Group"
                else "Group · ${sample.participants.size} people"
            }
            val shown = names.take(3).joinToString(", ")
            return if (names.size > 3) "$shown +${names.size - 3}" else shown
        }
        return contactFor(key)?.name ?: key
    }

    private fun snippet(m: Message): String {
        val prefix = when {
            m.direction == "out" -> "You: "
            m.isGroup && m.sender.isNotEmpty() -> (contactFor(m.sender)?.name ?: m.sender) + ": "
            else -> ""
        }
        val body = m.text.replace('\n', ' ').ifBlank { if (m.media.isNotEmpty()) "[attachment]" else "" }
        return prefix + body
    }

    private fun shortTime(m: Message): String {
        val stamp = m.stamp() ?: return ""
        val now = Instant.now()
        val minutes = java.time.Duration.between(stamp, now).toMinutes()
        return when {
            minutes < 1 -> "now"
            minutes < 60 -> "${minutes}m"
            minutes < 60 * 24 -> "${minutes / 60}h"
            minutes < 60 * 24 * 7 -> "${minutes / (60 * 24)}d"
            else -> java.time.format.DateTimeFormatter.ofPattern("yyyy/MM/dd HH:mm")
                .withZone(java.time.ZoneId.systemDefault()).format(stamp)
        }
    }

    fun refreshContacts() {
        viewModelScope.launch {
            val loaded = withContext(Dispatchers.IO) { runCatching { repo.contacts() }.getOrDefault(emptyMap()) }
            contacts = loaded
            publish()
        }
    }

    private fun startUpdates() {
        updatesJob?.cancel()
        eventChannel?.close()
        eventChannel = null
        val mode = repo.config.updateMode
        updatesJob = viewModelScope.launch {
            if (mode == "push") {
                launchReconcile()
                if (withContext(Dispatchers.IO) { repo.supportsEventStream() }) {
                    launchEventStream()
                } else {
                    launchPoll()
                }
            } else {
                launchPoll()
            }
        }
    }

    private fun kotlinx.coroutines.CoroutineScope.launchPoll() {
        launch {
            while (isActive) {
                delay(repo.config.pollSeconds.coerceAtLeast(15) * 1000L)
                if (!syncing && !sending) syncNow(full = false)
            }
        }
    }

    private fun kotlinx.coroutines.CoroutineScope.launchReconcile() {
        launch {
            while (isActive) {
                delay(repo.config.reconcileSeconds.coerceAtLeast(60) * 1000L)
                if (!syncing && !sending) syncNow(full = false)
            }
        }
    }

    private fun kotlinx.coroutines.CoroutineScope.launchEventStream() {
        val channel = Channel<PushEvent>(Channel.UNLIMITED)
        eventChannel = channel
        launch {
            while (isActive) {
                val gateway = repo.gateway
                if (gateway == null) break
                try {
                    withContext(Dispatchers.IO) {
                        gateway.streamEvents(repo.config.pollSeconds.coerceAtLeast(30)) { event ->
                            channel.trySend(event)
                        }
                    }
                } catch (e: kotlinx.coroutines.CancellationException) {
                    throw e
                } catch (e: Exception) {
                    repo.connected = false
                    publish()
                }
                delay(5000)
            }
        }
        launch {
            val batch = mutableListOf<PushEvent>()
            while (isActive) {
                batch.add(channel.receive())
                delay(250)
                while (batch.size < 256) {
                    val next = channel.tryReceive().getOrNull() ?: break
                    batch.add(next)
                }
                val events = batch.toList()
                batch.clear()
                processEvents(events)
            }
        }
    }

    private suspend fun processEvents(events: List<PushEvent>) {
        val outcome = withContext(Dispatchers.IO) {
            runCatching { repo.eventMessages(events) }
        }
        outcome.onSuccess { result ->
            repo.connected = true
            contacts = withContext(Dispatchers.IO) { runCatching { repo.contacts() }.getOrDefault(contacts) }
            applyAdoptions(result.adopted)
            publish()
            if (result.fullSync) syncNow(full = true)
        }.onFailure {
            repo.connected = false
            publish()
            notice(it.message ?: "Update failed")
        }
    }

    private fun applyAdoptions(adopted: Map<String, String>) {
        if (adopted.isEmpty()) return
        val current = state.currentNumber
        val real = adopted[current] ?: return
        state = state.copy(screen = Screen.Thread(real), currentNumber = real)
    }

    fun syncNow(full: Boolean) {
        if (syncing) {
            notice("Already refreshing")
            return
        }
        val account = repo.activeAccount() ?: return
        if (repo.password(account.id).isNullOrEmpty()) {
            notice("Set the account password in Settings")
            return
        }
        syncing = true
        state = state.copy(busy = true)
        viewModelScope.launch {
            val result = withContext(Dispatchers.IO) { runCatching { repo.sync(full) } }
            syncing = false
            result.onSuccess { outcome ->
                contacts = outcome.contacts
                applyAdoptions(outcome.adopted)
                publish { it.copy(busy = false) }
            }.onFailure {
                repo.connected = false
                publish { it.copy(busy = false) }
                notice(it.message ?: "Refresh failed")
            }
        }
    }

    // ---- Navigation ----

    fun openThread(number: String) {
        val sample = repo.messages().firstOrNull { it.conversationKey == number }
        val group = sample?.isGroup == true
        state = state.copy(
            screen = Screen.Thread(number, newThread = false),
            currentNumber = number,
            compose = drafts[number]?.copy(sending = false) ?: ComposeState(
                number = if (group) "" else number,
                recipients = if (group) sample?.participants.orEmpty() else listOf(number),
            ),
            query = "", searching = false,
        )
        publish()
    }

    fun openNewThread() {
        state = state.copy(
            screen = Screen.Thread("", newThread = true),
            currentNumber = "",
            compose = drafts[""]?.copy(sending = false, error = "") ?: ComposeState(),
            query = "", searching = false,
        )
        publish()
    }

    fun openAttachment(messageKey: String, partIndex: Int) {
        val message = repo.messages().firstOrNull { it.key == messageKey } ?: return
        val part = message.media.getOrNull(partIndex) ?: return
        state = state.copy(
            screen = Screen.AttachmentView(messageKey, partIndex),
            attachment = AttachmentUi(message, part),
        )
        val existing = part.localPath
        if (existing.isNotEmpty() && File(existing).exists()) {
            state = state.copy(attachment = AttachmentUi(message, part, path = existing))
        } else if (message.direction == "in") {
            state = state.copy(attachment = AttachmentUi(message, part, busy = true))
            viewModelScope.launch {
                val result = withContext(Dispatchers.IO) { runCatching { repo.ensureMedia(message, part) } }
                result.onSuccess { path ->
                    publish {
                        it.copy(attachment = it.attachment?.copy(path = path, busy = false))
                    }
                }.onFailure { failure ->
                    publish {
                        it.copy(attachment = it.attachment?.copy(busy = false, error = failure.message ?: "Download failed"))
                    }
                }
            }
        }
    }

    fun openScreen(screen: Screen) {
        state = state.copy(screen = screen, query = "", searching = false)
        publish()
    }

    fun back() {
        when (val screen = state.screen) {
            is Screen.Thread -> {
                val key = screen.number
                if (state.compose.text.isNotBlank() || state.compose.attachments.isNotEmpty() ||
                    state.compose.subject.isNotBlank() || state.compose.recipients.isNotEmpty() ||
                    state.compose.number.isNotBlank()
                ) {
                    drafts[key] = state.compose.copy(sending = false, error = "")
                } else {
                    drafts.remove(key)
                }
                state = state.copy(screen = Screen.Conversations, currentNumber = "", compose = ComposeState(), query = "", searching = false)
                publish()
            }
            is Screen.AttachmentView -> {
                val number = state.attachment?.message?.number.orEmpty()
                state = state.copy(
                    screen = Screen.Thread(number), currentNumber = number,
                    attachment = null, query = "", searching = false,
                )
                publish()
            }
            is Screen.AccountEditor -> openScreen(Screen.Settings)
            Screen.Settings -> openScreen(Screen.Conversations)
            Screen.Conversations -> Unit
        }
    }

    fun setQuery(query: String) {
        state = state.copy(query = query)
        queryJob?.cancel()
        queryJob = viewModelScope.launch {
            delay(250)
            val filtered = withContext(Dispatchers.IO) {
                runCatching { if (state.query.isBlank()) repo.contacts() else repo.contacts(state.query) }.getOrDefault(contacts)
            }
            aliases = filtered.keys
            if (state.query.isBlank()) contacts = filtered
            publish()
        }
    }

    fun toggleSearch() {
        state = state.copy(searching = !state.searching, query = "")
        aliases = emptySet()
        publish()
    }

    fun clearSearch() {
        state = state.copy(searching = false, query = "")
        aliases = emptySet()
        publish()
    }

    // ---- Inline compose ----

    private fun updateCompose(block: (ComposeState) -> ComposeState) {
        state = state.copy(compose = block(state.compose))
    }

    fun setComposeNumber(value: String) = updateCompose { it.copy(number = value, error = "") }

    fun addComposeRecipient(value: String) {
        val trimmed = value.trim()
        if (trimmed.isEmpty()) return
        updateCompose {
            if (it.recipients.any { r -> phoneKey(r) == phoneKey(trimmed) }) {
                it.copy(number = "", error = "")
            } else {
                it.copy(recipients = it.recipients + trimmed, number = "", error = "")
            }
        }
    }

    fun removeComposeRecipient(index: Int) = updateCompose {
        it.copy(recipients = it.recipients.filterIndexed { i, _ -> i != index })
    }
    fun setComposeText(value: String) = updateCompose { it.copy(text = value, error = "") }
    fun setComposeSubject(value: String) = updateCompose {
        it.copy(subject = value, mms = true, error = "")
    }

    fun toggleMms() = updateCompose {
        if (it.mms) it.copy(mms = false, subject = "", attachments = emptyList(), error = "")
        else it.copy(mms = true, error = "")
    }

    fun removeAttachment(index: Int) = updateCompose {
        it.copy(attachments = it.attachments.filterIndexed { i, _ -> i != index })
    }

    fun addAttachment(uri: Uri) {
        val context = getApplication<Application>()
        viewModelScope.launch {
            val result = withContext(Dispatchers.IO) {
                runCatching {
                    var name = "attachment"
                    var size = 0L
                    context.contentResolver.query(uri, null, null, null, null)?.use { cursor ->
                        if (cursor.moveToFirst()) {
                            val nameIndex = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
                            val sizeIndex = cursor.getColumnIndex(OpenableColumns.SIZE)
                            if (nameIndex >= 0) cursor.getString(nameIndex)?.let { name = it }
                            if (sizeIndex >= 0 && !cursor.isNull(sizeIndex)) size = cursor.getLong(sizeIndex)
                        }
                    }
                    if (size > 20L * 1024 * 1024) throw SendFailure("Attachment exceeds 20 MiB")
                    val bytes = context.contentResolver.openInputStream(uri)?.use { input ->
                        val output = java.io.ByteArrayOutputStream()
                        val buffer = ByteArray(64 * 1024)
                        var total = 0
                        while (true) {
                            val count = input.read(buffer)
                            if (count < 0) break
                            total += count
                            if (total > 20 * 1024 * 1024) throw SendFailure("Attachment exceeds 20 MiB")
                            output.write(buffer, 0, count)
                        }
                        output.toByteArray()
                    } ?: throw SendFailure("Cannot read the selected file")
                    val type = context.contentResolver.getType(uri) ?: "application/octet-stream"
                    PendingAttachment(name = name, contentType = type, size = bytes.size.toLong(), bytes = bytes)
                }
            }
            result.onSuccess { file ->
                updateCompose {
                    it.copy(attachments = it.attachments + file, mms = true, error = "")
                }
            }.onFailure { failure ->
                updateCompose { it.copy(error = failure.message ?: "Cannot attach that file") }
            }
        }
    }

    fun send() {
        val compose = state.compose
        if (sending) return
        val typed = recipient(compose.number)
        val candidates = (compose.recipients + listOfNotNull(typed))
            .map { it.trim() }
            .filter { it.isNotEmpty() }
            .distinctBy { phoneKey(it) }
        if (candidates.isEmpty()) {
            updateCompose { it.copy(error = "Add a recipient") }
            return
        }
        val numbers = mutableListOf<String>()
        for (candidate in candidates) {
            val number = recipient(candidate)
            if (number == null) {
                updateCompose { it.copy(error = "Include the country code, e.g. +12025550123") }
                return
            }
            numbers.add(number)
        }
        if (numbers.size > 10) {
            updateCompose { it.copy(error = "A group can have at most 10 recipients") }
            return
        }
        if (compose.text.isBlank() && compose.attachments.isEmpty()) {
            updateCompose { it.copy(error = "Write a message first") }
            return
        }
        sending = true
        updateCompose { it.copy(sending = true, error = "") }
        viewModelScope.launch {
            val result = withContext(Dispatchers.IO) {
                runCatching { repo.send(numbers, compose.text, compose.subject, compose.attachments) }
            }
            sending = false
            result.onSuccess { sent ->
                val key = sent.conversationKey
                drafts.remove(key)
                drafts.remove("")
                numbers.forEach { drafts.remove(it) }
                state = state.copy(
                    compose = if (sent.isGroup) {
                        ComposeState(recipients = sent.participants)
                    } else {
                        ComposeState(number = key, recipients = listOf(key))
                    },
                    screen = Screen.Thread(key),
                    currentNumber = key,
                )
                publish()
                notice(if (sent.isGroup) "Group message sent" else "Message sent")
                if (sent.isGroup) syncNow(full = false)
            }.onFailure {
                updateCompose { c -> c.copy(sending = false, error = it.message ?: "Send failed") }
            }
        }
    }

    fun saveAttachment(uri: Uri) {
        val attachment = state.attachment ?: return
        val path = attachment.path
        if (path == null) {
            notice("Attachment is not downloaded yet")
            return
        }
        viewModelScope.launch {
            val result = withContext(Dispatchers.IO) {
                runCatching {
                    getApplication<Application>().contentResolver.openOutputStream(uri)?.use { out ->
                        File(path).inputStream().use { it.copyTo(out) }
                    } ?: throw SendFailure("Cannot write the selected file")
                }
            }
            result.onSuccess { notice("Attachment saved") }
                .onFailure { notice(it.message ?: "Save failed") }
        }
    }

    fun markOpen(path: String) {
        state = state.copy(openPath = path)
    }

    fun openedFile() {
        state = state.copy(openPath = null)
    }

    // ---- Accounts and settings ----

    fun selectAccount(id: String) {
        if (id == repo.config.activeAccount) return
        if (sending) {
            notice("Wait for the current send")
            return
        }
        val next = repo.config.copy(activeAccount = id)
        repo.replaceConfig(next)
        contacts = emptyMap()
        aliases = emptySet()
        state = state.copy(
            screen = Screen.Conversations, currentNumber = "", compose = ComposeState(),
            query = "", searching = false, lastSync = null,
        )
        drafts.clear()
        publish()
        startUpdates()
        syncNow(full = false)
    }

    fun saveAccount(
        id: String?,
        name: String,
        server: String,
        username: String,
        password: String,
    ): String? {
        val cleanedServer = server.trim().trimEnd('/')
        if (name.isBlank()) return "Account needs a name"
        val candidate = Account(
            id = id ?: repo.configStore.newAccount(name, cleanedServer, username).id,
            name = name.trim(),
            server = cleanedServer,
            username = username.trim(),
        )
        if (candidate.server.isBlank() || candidate.username.isBlank()) return "Server and username are required"
        val url = runCatching { java.net.URI(candidate.server) }.getOrNull()
        if (url?.host.isNullOrBlank() || (url?.scheme != "http" && url?.scheme != "https")) {
            return "Use an HTTP(S) server address"
        }
        val existingPassword = id?.let { repo.password(it) }
        if (password.isBlank() && existingPassword.isNullOrEmpty()) return "Password is required"
        if (password.isNotBlank()) repo.storePassword(candidate.id, password)
        val accounts = repo.config.accounts.map { if (it.id == candidate.id) candidate else it } +
            (if (repo.config.accounts.none { it.id == candidate.id }) listOf(candidate) else emptyList())
        val active = repo.config.activeAccount.ifEmpty { candidate.id }
        val next = repo.config.copy(accounts = accounts, activeAccount = active)
        val error = next.validate()
        if (error != null) return error
        repo.replaceConfig(next)
        if (active == candidate.id) repo.reloadActive()
        refreshContacts()
        startUpdates()
        if (active == candidate.id) syncNow(full = false)
        return null
    }

    fun moveAccount(id: String, delta: Int) {
        val accounts = repo.config.accounts.toMutableList()
        val index = accounts.indexOfFirst { it.id == id }
        val target = index + delta
        if (index < 0 || target !in accounts.indices) return
        val item = accounts.removeAt(index)
        accounts.add(target, item)
        repo.replaceConfig(repo.config.copy(accounts = accounts))
        publish()
    }

    fun deleteAccount(id: String): String? {
        if (repo.config.accounts.size <= 1) return "Keep at least one account"
        val accounts = repo.config.accounts.filterNot { it.id == id }
        val active = if (repo.config.activeAccount == id) accounts.first().id else repo.config.activeAccount
        repo.removePassword(id)
        repo.replaceConfig(repo.config.copy(accounts = accounts, activeAccount = active))
        repo.reloadActive()
        state = state.copy(screen = Screen.Settings)
        startUpdates()
        syncNow(full = false)
        return null
    }

    fun testAccount(account: Account, password: String, done: (String) -> Unit) {
        val effective = password.ifBlank { repo.password(account.id).orEmpty() }
        if (effective.isEmpty()) {
            done("Password is required")
            return
        }
        viewModelScope.launch {
            val result = withContext(Dispatchers.IO) { runCatching { repo.checkConnection(account, effective) } }
            done(result.getOrElse { it.message ?: "Connection failed" })
        }
    }

    fun saveSettings(next: Config): String? {
        val error = next.validate()
        if (error != null) return error
        repo.replaceConfig(next)
        repo.reloadActive()
        state = state.copy(screen = Screen.Conversations)
        startUpdates()
        syncNow(full = false)
        publish()
        return null
    }

    fun setTheme(theme: String) {
        val next = repo.config.copy(theme = theme)
        repo.replaceConfig(next)
        publish()
    }

    fun setUpdateMode(mode: String) {
        val next = repo.config.copy(updateMode = mode)
        repo.replaceConfig(next)
        startUpdates()
        publish()
    }

    fun togglePagedScrolling() {
        repo.replaceConfig(repo.config.copy(pagedScrolling = !repo.config.pagedScrolling))
        publish()
    }

    fun onContactsPermissionChanged() {
        repo.contactBook.invalidateSystem()
        refreshContacts()
    }

    override fun onCleared() {
        updatesJob?.cancel()
        eventChannel?.close()
        repo.close()
        super.onCleared()
    }
}

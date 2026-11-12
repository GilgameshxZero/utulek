package com.gilgamesh.maildirt

import android.app.Application
import android.net.Uri
import android.os.FileObserver
import androidx.core.content.FileProvider
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.File
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.Properties
import java.util.TimeZone
import java.util.UUID
import javax.mail.Session
import javax.mail.internet.InternetAddress
import javax.mail.internet.MimeMessage
import javax.mail.internet.MimeUtility

sealed interface Screen {
    data class Messages(val folderId: String) : Screen
    data class Message(val folderId: String, val relativePath: String) : Screen
    data object Settings : Screen
    data class FolderEditor(val folderId: String?) : Screen
    data class IdentityEditor(val identityId: String?) : Screen
    data object Compose : Screen
    data object PickIdentity : Screen
    data class CalendarReply(val folderId: String, val relativePath: String) : Screen
    data class GoToItem(val folderId: String) : Screen
    data class PickMaildir(
        val folderId: String,
        val relativePath: String?,
        val forSelection: Boolean,
    ) : Screen
}

data class ComposeSeed(
    val identityId: String,
    val to: String = "",
    val cc: String = "",
    val bcc: String = "",
    val subject: String = "",
    val body: String = "",
    val attachments: List<OutgoingAttachment> = emptyList(),
    val inReplyTo: String? = null,
    val references: String? = null,
    val replyFolderId: String? = null,
    val replyPath: String? = null,
    val draftFolderId: String? = null,
    val draftPath: String? = null,
)

data class ComposeFields(
    val identityId: String,
    val to: String,
    val cc: String,
    val bcc: String,
    val subject: String,
    val body: String,
)

class MainViewModel(application: Application) : AndroidViewModel(application) {
    private val configStore = ConfigStore(application)
    private val mimeReader = MimeReader(application)
    private val indexStore = FolderIndexStore(application)
    private val credentialStore = CredentialStore(application)
    private val smtpClient = SmtpClient(credentialStore)
    private val addressBook = AddressBook(application)
    private val searchIndex = SearchIndex(application)

    var composeSeed by mutableStateOf<ComposeSeed?>(null)
        private set
    var composeError by mutableStateOf<String?>(null)
        private set
    var sending by mutableStateOf(false)
        private set
    var posting by mutableStateOf(false)
        private set
    var testingIdentity by mutableStateOf<String?>(null)
        private set
    var identityTestResult by mutableStateOf<String?>(null)
        private set

    var config by mutableStateOf(configStore.load())
        private set
    var screen by mutableStateOf<Screen>(Screen.Messages(""))
        private set
    var messages by mutableStateOf<List<MaildirEntry>>(emptyList())
        private set
    var listLoading by mutableStateOf(false)
        private set
    var listError by mutableStateOf<String?>(null)
        private set
    var listState = LazyListState()
        private set
    var parsed by mutableStateOf<ParsedMessage?>(null)
        private set
    var messageLoading by mutableStateOf(false)
        private set
    var messageError by mutableStateOf<String?>(null)
        private set
    var remoteImages by mutableStateOf(true)
        private set
    var viewAlternative by mutableStateOf<String?>(null)
        private set
    var showSource by mutableStateOf(false)
        private set
    var rawSource by mutableStateOf<String?>(null)
        private set
    var currentEntry by mutableStateOf<MaildirEntry?>(null)
        private set
    var actionInProgress by mutableStateOf(false)
        private set
    var toast by mutableStateOf<String?>(null)
        private set
    var calendarReplySending by mutableStateOf(false)
        private set
    var calendarReplyError by mutableStateOf<String?>(null)
        private set
    var selection by mutableStateOf<Set<String>>(emptySet())
        private set
    var searchQuery by mutableStateOf("")
        private set
    var searchActive by mutableStateOf(false)
        private set
    val searchBody: Boolean
        get() = config.searchBody
    var searchMatchIndex by mutableStateOf(-1)
        private set
    var searchLoading by mutableStateOf(false)
        private set
    private var searchJob: Job? = null

    val currentSearchMatch: String?
        get() = messages.getOrNull(searchMatchIndex)?.relativePath

    private var listJob: Job? = null
    private var refreshInFlight = false
    private var refreshQueued = false
    private var refreshFolderId: String? = null
    private var activeFolderId: String? = null
    private var messageJob: Job? = null
    private var watchRefreshJob: Job? = null
    private var pollJob: Job? = null
    private val folderObservers = mutableListOf<FileObserver>()
    private val navPrefs = application.getSharedPreferences("nav", 0)
    private var pendingMessagePath: String? = null

    init {
        val initialId = config.selectedFolderId?.takeIf { folder(it) != null }
            ?: config.folders.firstOrNull { it.role == FolderRole.INBOX }?.id
            ?: config.folders.firstOrNull()?.id
            ?: ""
        if (initialId.isNotEmpty()) {
            activeFolderId = initialId
            screen = Screen.Messages(initialId)
            folder(initialId)?.let { startWatching(it) }
        }
        restoreNavigation()
        if (screen is Screen.Messages) refreshFolder()
        viewModelScope.launch(Dispatchers.IO) {
            searchIndex.pruneFolders(config.folders.map { it.path }.toSet())
        }
    }

    fun folders(): List<MaildirFolder> = config.folders

    fun folder(id: String): MaildirFolder? = config.folders.firstOrNull { it.id == id }

    fun archiveFolder(): MaildirFolder? =
        config.folders.firstOrNull { it.role == FolderRole.ARCHIVE }

    private fun storeFor(folder: MaildirFolder) = MaildirFolderStore(folder, indexStore)

    private fun messagesScreen(): Screen {
        val id = activeFolderId?.takeIf { folder(it) != null }
            ?: config.selectedFolderId?.takeIf { folder(it) != null }
            ?: config.folders.firstOrNull { it.role == FolderRole.INBOX }?.id
            ?: config.folders.firstOrNull()?.id
            ?: ""
        return Screen.Messages(id)
    }

    fun openFolder(id: String) {
        listState = LazyListState()
        selection = emptySet()
        clearSearch()
        if (activeFolderId != id) messages = emptyList()
        activeFolderId = id
        screen = Screen.Messages(id)
        if (config.selectedFolderId != id) save(config.copy(selectedFolderId = id))
        folder(id)?.let { startWatching(it) }
        saveNavigation()
        refreshFolder()
    }

    private fun startWatching(folder: MaildirFolder) {
        stopWatching()
        val mask = FileObserver.CREATE or FileObserver.MOVED_TO or FileObserver.CLOSE_WRITE or
            FileObserver.DELETE or FileObserver.MOVED_FROM or FileObserver.ATTRIB
        for (sub in listOf("cur", "new")) {
            val dir = File(folder.path, sub)
            if (!dir.isDirectory) continue
            val observer = object : FileObserver(dir.absolutePath, mask) {
                override fun onEvent(event: Int, path: String?) {
                    watchRefreshJob?.cancel()
                    watchRefreshJob = viewModelScope.launch {
                        delay(1000)
                        if ((screen as? Screen.Messages)?.folderId == folder.id) refreshFolder()
                    }
                }
            }
            observer.startWatching()
            folderObservers.add(observer)
        }
        // FileObserver is unreliable on emulated/FUSE storage; poll directory mtimes.
        pollJob = viewModelScope.launch {
            var lastCur = File(folder.path, "cur").lastModified()
            var lastNew = File(folder.path, "new").lastModified()
            while (isActive) {
                delay(5000)
                if ((screen as? Screen.Messages)?.folderId != folder.id) continue
                val cur = File(folder.path, "cur").lastModified()
                val new = File(folder.path, "new").lastModified()
                if (cur != lastCur || new != lastNew) {
                    lastCur = cur
                    lastNew = new
                    refreshFolder()
                }
            }
        }
    }

    private fun stopWatching() {
        watchRefreshJob?.cancel()
        pollJob?.cancel()
        folderObservers.forEach { runCatching { it.stopWatching() } }
        folderObservers.clear()
    }

    override fun onCleared() {
        stopWatching()
        super.onCleared()
    }

    fun onAppResume() {
        if (screen is Screen.Messages) refreshFolder()
    }

    fun openFromNotification(folderId: String?, folderRole: String?, relativePath: String) {
        val folder = folderId?.let { folder(it) }
            ?: folderRole?.let { role -> config.folders.firstOrNull { it.role.name == role } }
            ?: return
        clearSearch()
        selection = emptySet()
        if (activeFolderId != folder.id) messages = emptyList()
        activeFolderId = folder.id
        screen = Screen.Messages(folder.id)
        pendingMessagePath = relativePath
        startWatching(folder)
        saveNavigation()
        refreshFolder()
    }

    fun refreshFolder() {
        val folderId = (screen as? Screen.Messages)?.folderId ?: return
        val folder = folder(folderId) ?: return
        if (refreshInFlight) {
            if (refreshFolderId == folderId) {
                refreshQueued = true
                return
            }
            listJob?.cancel()
            refreshInFlight = false
        }
        val anchor = messages.getOrNull(listState.firstVisibleItemIndex)?.relativePath
        refreshInFlight = true
        refreshFolderId = folderId
        listLoading = true
        listError = null
        listJob = viewModelScope.launch {
            val started = System.currentTimeMillis()
            val result = withContext(Dispatchers.IO) {
                runCatching { storeFor(folder).list() }
            }
            val elapsed = System.currentTimeMillis() - started
            if (elapsed > 500) {
                android.util.Log.w("maildirt", "slow list ${folder.path}: ${elapsed}ms")
            }
            refreshInFlight = false
            result.onSuccess {
                messages = it
                if (anchor != null) {
                    val index = it.indexOfFirst { entry -> entry.relativePath == anchor }
                    if (index >= 0) listState.scrollToItem(index)
                }
                if (selection.isNotEmpty()) {
                    val valid = it.mapTo(HashSet()) { entry -> entry.relativePath }
                    selection = selection.filterTo(HashSet()) { path -> path in valid }
                }
                pendingMessagePath?.let { path ->
                    pendingMessagePath = null
                    if (messages.any { it.relativePath == path }) {
                        openMessage(path)
                    } else {
                        saveNavigation()
                    }
                }
            }.onFailure {
                android.util.Log.e("maildirt", "listing failed for ${folder.path}", it)
                listError = it.message ?: it.toString()
            }
            listLoading = false
            if (refreshQueued) {
                refreshQueued = false
                refreshFolder()
            }
        }
    }

    fun openMessage(relativePath: String) {
        val folderId = (screen as? Screen.Messages)?.folderId ?: return
        val folder = folder(folderId) ?: return
        val entry = messages.firstOrNull { it.relativePath == relativePath } ?: run {
            return
        }

        if (folder.role == FolderRole.POSTPONED) {
            openDraft(folderId, relativePath)
            return
        }

        screen = Screen.Message(folderId, relativePath)
        currentEntry = entry
        saveNavigation()
        parsed = null
        messageError = null
        messageLoading = true
        remoteImages = true
        viewAlternative = null
        showSource = false
        rawSource = null
        messageJob?.cancel()
        messageJob = viewModelScope.launch {
            val result = withContext(Dispatchers.IO) {
                runCatching {
                    val store = storeFor(folder)
                    val cacheKey = "${folderId.hashCode()}_${relativePath.hashCode()}"
                    val message = mimeReader.parse(store.fileFor(entry.relativePath), cacheKey)
                    val updated = if (entry.unread) {
                        runCatching { store.setSeen(entry, true) }.getOrNull()
                    } else {
                        null
                    }
                    message to updated
                }
            }
            result.onSuccess { (message, updated) ->
                parsed = message
                if (updated != null) {
                    replaceEntry(entry.relativePath, updated)
                    currentEntry = updated
                    screen = Screen.Message(folderId, updated.relativePath)
                    saveNavigation()
                }
            }.onFailure {
                messageError = it.message ?: it.toString()
            }
            messageLoading = false
        }
    }

    fun openCalendarReply() {
        val current = screen as? Screen.Message ?: return
        if (parsed?.calendar?.canRespond != true) return
        calendarReplyError = null
        val path = currentEntry?.relativePath ?: current.relativePath
        screen = Screen.CalendarReply(current.folderId, path)
        saveNavigation()
    }

    fun sendCalendarReply(identityId: String, state: CalendarResponseState) {
        if (calendarReplySending) return
        val current = screen as? Screen.CalendarReply ?: return
        val invite = parsed?.calendar
        if (invite == null || !invite.canRespond) {
            calendarReplyError = "This message has no replyable invitation"
            return
        }
        val identity = identity(identityId) ?: run {
            calendarReplyError = "Choose a sender identity"
            return
        }
        val recordFolder = config.folders.firstOrNull { it.role == FolderRole.ARCHIVE }
        if (recordFolder == null) {
            calendarReplyError = "No archive maildir configured for sent copies"
            return
        }

        calendarReplySending = true
        calendarReplyError = null
        viewModelScope.launch {
            val outcome = withContext(Dispatchers.IO) {
                runCatching {
                    val calendar = CalendarMessaging.buildReply(invite, identity.email, state)
                    val message = MailComposer.buildCalendar(
                        identity = identity,
                        to = invite.organizerEmail,
                        subject = CalendarMessaging.subjectFor(invite, state),
                        calendar = calendar,
                    )
                    smtpClient.send(identity, message)
                    storeFor(recordFolder).append(MailComposer.toBytes(message), "S")
                        ?: throw IllegalStateException(
                            "Sent, but could not save a copy to ${recordFolder.displayName}",
                        )
                }
            }
            calendarReplySending = false
            outcome.onSuccess {
                toast = "Response sent"
                screen = Screen.Message(current.folderId, current.relativePath)
                saveNavigation()
            }.onFailure {
                calendarReplyError = it.message ?: it.toString()
            }
        }
    }

    fun updateRemoteImages(enabled: Boolean) {        remoteImages = enabled
    }

    fun setPagedScrolling(enabled: Boolean) {
        save(config.copy(pagedScrolling = enabled))
    }

    fun chooseAlternative(mimeType: String?) {
        viewAlternative = mimeType
    }

    fun toggleSource() {
        if (showSource) {
            showSource = false
            return
        }
        val folderId = (screen as? Screen.Message)?.folderId ?: return
        val folder = folder(folderId) ?: return
        val entry = currentEntry ?: return
        viewModelScope.launch {
            val text = withContext(Dispatchers.IO) {
                runCatching {
                    storeFor(folder).fileFor(entry.relativePath).readText(Charsets.ISO_8859_1)
                }.getOrNull()
            }
            if (text != null) {
                rawSource = text
                showSource = true
            } else {
                toast = "Could not read message file"
            }
        }
    }

    fun markCurrentUnread() {
        val folderId = (screen as? Screen.Message)?.folderId ?: return
        val folder = folder(folderId) ?: return
        val entry = currentEntry ?: return
        if (entry.unread || actionInProgress) return

        actionInProgress = true
        viewModelScope.launch {
            val updated = withContext(Dispatchers.IO) {
                runCatching { storeFor(folder).setSeen(entry, false) }.getOrNull()
            }
            actionInProgress = false
            if (updated != null) {
                replaceEntry(entry.relativePath, updated)
                currentEntry = updated
                saveNavigation()
                toast = "Marked unread"
            } else {
                toast = "Could not mark unread"
            }
        }
    }

    fun archiveCurrent() {
        val folderId = (screen as? Screen.Message)?.folderId ?: return
        val sourceFolder = folder(folderId) ?: return
        val entry = currentEntry ?: return
        if (sourceFolder.role == FolderRole.ARCHIVE || actionInProgress) return

        val archive = archiveFolder()
        if (archive == null) {
            toast = "No archive maildir configured"
            return
        }

        actionInProgress = true
        viewModelScope.launch {
            val moved = withContext(Dispatchers.IO) {
                runCatching {
                    storeFor(sourceFolder).moveTo(storeFor(archive), entry, addSeen = true)
                }.getOrDefault(false)
            }
            actionInProgress = false
            if (moved) {
                toast = "Archived"
                screen = Screen.Messages(folderId)
                saveNavigation()
                refreshFolder()
            } else {
                toast = "Could not archive message"
            }
        }
    }

    fun deleteCurrent() {
        val folderId = (screen as? Screen.Message)?.folderId ?: return
        val folder = folder(folderId) ?: return
        if (folder.role != FolderRole.ARCHIVE || actionInProgress) return
        val entry = currentEntry ?: return

        actionInProgress = true
        viewModelScope.launch {
            val deleted = withContext(Dispatchers.IO) {
                runCatching { storeFor(folder).delete(entry) }.getOrDefault(false)
            }
            actionInProgress = false
            if (deleted) {
                toast = "Deleted"
                screen = Screen.Messages(folderId)
                saveNavigation()
                refreshFolder()
            } else {
                toast = "Could not delete message"
            }
        }
    }

    fun moveCurrent(folderId: String, targetId: String) {
        val sourceFolder = folder(folderId) ?: return
        val entry = currentEntry ?: return
        if (targetId == folderId || actionInProgress) return
        val targetFolder = folder(targetId) ?: return

        actionInProgress = true
        viewModelScope.launch {
            val moved = withContext(Dispatchers.IO) {
                runCatching {
                    storeFor(sourceFolder).moveTo(storeFor(targetFolder), entry, addSeen = false)
                }.getOrDefault(false)
            }
            actionInProgress = false
            if (moved) {
                toast = "Moved to ${targetFolder.displayName}"
                screen = Screen.Messages(folderId)
                saveNavigation()
                refreshFolder()
            } else {
                toast = "Could not move message"
            }
        }
    }

    fun startSelection(path: String) {
        selection = selection + path
    }

    fun toggleSelection(path: String) {
        selection = if (path in selection) selection - path else selection + path
    }

    fun clearSelection() {
        selection = emptySet()
    }

    fun selectAll() {
        if (actionInProgress) return
        selection = messages.mapTo(HashSet()) { it.relativePath }
    }

    private fun selectedEntries(): List<MaildirEntry> =
        messages.filter { it.relativePath in selection }

    fun markSelected(seen: Boolean) {
        val folderId = (screen as? Screen.Messages)?.folderId ?: return
        val folder = folder(folderId) ?: return
        val entries = selectedEntries()
        if (entries.isEmpty() || actionInProgress) return
        actionInProgress = true
        viewModelScope.launch {
            val count = withContext(Dispatchers.IO) {
                val store = storeFor(folder)
                entries.count { store.setSeen(it, seen) != null }
            }
            actionInProgress = false
            selection = emptySet()
            clearSearch()
            toast = when {
                count == 0 -> "No messages changed"
                seen -> "$count marked read"
                else -> "$count marked unread"
            }
            refreshFolder()
        }
    }

    fun archiveSelected() {
        val folderId = (screen as? Screen.Messages)?.folderId ?: return
        val sourceFolder = folder(folderId) ?: return
        if (sourceFolder.role == FolderRole.ARCHIVE || actionInProgress) return
        val archive = archiveFolder()
        if (archive == null) {
            toast = "No archive maildir configured"
            return
        }
        val entries = selectedEntries()
        if (entries.isEmpty()) return
        actionInProgress = true
        viewModelScope.launch {
            val count = withContext(Dispatchers.IO) {
                val source = storeFor(sourceFolder)
                val target = storeFor(archive)
                entries.count { source.moveTo(target, it, addSeen = true) }
            }
            actionInProgress = false
            selection = emptySet()
            clearSearch()
            toast = if (count > 0) "$count archived" else "Could not archive messages"
            refreshFolder()
        }
    }

    fun deleteSelected() {
        val folderId = (screen as? Screen.Messages)?.folderId ?: return
        val folder = folder(folderId) ?: return
        if (folder.role != FolderRole.ARCHIVE || actionInProgress) return
        val entries = selectedEntries()
        if (entries.isEmpty()) return
        actionInProgress = true
        viewModelScope.launch {
            val count = withContext(Dispatchers.IO) {
                val store = storeFor(folder)
                entries.count { store.delete(it) }
            }
            actionInProgress = false
            selection = emptySet()
            clearSearch()
            toast = if (count > 0) "$count deleted" else "Could not delete messages"
            refreshFolder()
        }
    }

    fun moveSelected(folderId: String, targetId: String) {
        val sourceFolder = folder(folderId) ?: return
        if (targetId == folderId || actionInProgress) return
        val targetFolder = folder(targetId) ?: return
        val entries = selectedEntries()
        if (entries.isEmpty()) return
        actionInProgress = true
        viewModelScope.launch {
            val count = withContext(Dispatchers.IO) {
                val source = storeFor(sourceFolder)
                val target = storeFor(targetFolder)
                entries.count { source.moveTo(target, it, addSeen = false) }
            }
            actionInProgress = false
            selection = emptySet()
            clearSearch()
            toast = if (count > 0) {
                "$count moved to ${targetFolder.displayName}"
            } else {
                "Could not move messages"
            }
            refreshFolder()
        }
    }

    fun beginSearch() {
        searchActive = true
        searchQuery = ""
        searchMatchIndex = -1
    }

    fun updateSearchQuery(query: String) {
        searchQuery = query
        searchActive = true
        searchJob?.cancel()
        val trimmed = query.trim()
        if (trimmed.isEmpty()) {
            searchMatchIndex = -1
            searchLoading = false
            return
        }
        searchLoading = true
        searchJob = viewModelScope.launch {
            delay(350)
            val index = findMatch(0, 1, wrap = false)
            searchMatchIndex = index
            searchLoading = false
            if (index >= 0) scrollToSearchMatch() else toast = "No match"
        }
    }

    fun toggleSearchScope() {
        save(config.copy(searchBody = !config.searchBody))
        toast = if (config.searchBody) "Searching headers and body" else "Searching headers only"
        if (searchQuery.isNotBlank()) {
            updateSearchQuery(searchQuery)
        }
    }

    fun nextSearchMatch() {
        if (searchQuery.isBlank() || searchLoading) return
        searchJob?.cancel()
        searchLoading = true
        searchJob = viewModelScope.launch {
            val start = if (searchMatchIndex < 0) 0 else searchMatchIndex + 1
            val index = findMatch(start, 1, wrap = true)
            searchMatchIndex = index
            searchLoading = false
            if (index >= 0) scrollToSearchMatch() else toast = "No match"
        }
    }

    fun previousSearchMatch() {
        if (searchQuery.isBlank() || searchLoading) return
        searchJob?.cancel()
        searchLoading = true
        searchJob = viewModelScope.launch {
            val start = if (searchMatchIndex < 0) messages.lastIndex else searchMatchIndex - 1
            val index = findMatch(start, -1, wrap = true)
            searchMatchIndex = index
            searchLoading = false
            if (index >= 0) scrollToSearchMatch() else toast = "No match"
        }
    }

    fun clearSearch() {
        searchJob?.cancel()
        searchQuery = ""
        searchActive = false
        searchMatchIndex = -1
        searchLoading = false
    }

    fun refreshAll() {
        clearSearch()
        refreshFolder()
    }

    private fun scrollToSearchMatch() {
        if (searchMatchIndex in messages.indices) {
            viewModelScope.launch { listState.scrollToItem(searchMatchIndex) }
        }
    }

    private suspend fun findMatch(fromIndex: Int, step: Int, wrap: Boolean): Int {
        val total = messages.size
        if (total == 0) return -1
        val folderId = (screen as? Screen.Messages)?.folderId ?: return -1
        val folder = folder(folderId) ?: return -1
        val needle = searchQuery.trim()
        if (needle.isEmpty()) return -1
        val includeBody = searchBody
        val snapshot = messages
        val matches = withContext(Dispatchers.IO) {
            searchIndex.sync(folder, snapshot)
            searchIndex.search(folder, needle, includeBody)
        }
        if (matches.isEmpty()) return -1

        var index = fromIndex.coerceIn(0, total - 1)
        var scanned = 0
        while (scanned < total) {
            val entry = snapshot.getOrNull(index) ?: break
            if (entry.relativePath in matches) {
                return index
            }
            scanned++
            index += step
            if (index !in snapshot.indices) {
                if (!wrap) break
                index = if (step > 0) 0 else snapshot.lastIndex
            }
        }
        return -1
    }

    fun openGoToItem() {
        val folderId = (screen as? Screen.Messages)?.folderId ?: return
        clearSearch()
        screen = Screen.GoToItem(folderId)
        saveNavigation()
    }

    fun goToItem(folderId: String, number: Int): String? {
        val total = messages.size
        if (total == 0) return "No messages in this folder"
        if (number !in 1..total) return "Enter a number between 1 and $total"
        screen = Screen.Messages(folderId)
        saveNavigation()
        viewModelScope.launch { listState.scrollToItem(number - 1) }
        return null
    }

    fun consumeToast() {
        toast = null
    }

    fun openSettings() {
        selection = emptySet()
        screen = Screen.Settings
        saveNavigation()
    }

    fun openFolderEditor(folderId: String?) {
        selection = emptySet()
        screen = Screen.FolderEditor(folderId)
        saveNavigation()
    }

    fun identities(): List<SenderIdentity> = config.identities

    fun identity(id: String): SenderIdentity? = config.identities.firstOrNull { it.id == id }

    fun hasStoredSecret(id: String): Boolean = credentialStore.has(id)

    fun openIdentityEditor(identityId: String?) {
        selection = emptySet()
        screen = Screen.IdentityEditor(identityId)
        saveNavigation()
    }

    fun saveIdentity(identity: SenderIdentity, secret: String): String? {
        val email = identity.email.trim()
        if (email.isEmpty()) return "Email address is required"
        if (!email.contains('@') || email.any { it.isWhitespace() }) {
            return "Enter a valid email address"
        }
        val host = identity.smtpHost.trim()
        if (host.isEmpty()) return "SMTP host is required"
        if (identity.smtpPort !in 1..65535) return "SMTP port must be 1-65535"
        if (identity.authMode == AuthMode.STANDARD && identity.username.trim().isEmpty()) {
            return "Username is required"
        }
        if (identity.authMode == AuthMode.XOAUTH2 && identity.tokenFilePath.trim().isEmpty()) {
            return "Token file path is required"
        }
        if (secret.isBlank() && !credentialStore.has(identity.id)) {
            return if (identity.authMode == AuthMode.STANDARD) {
                "Password is required"
            } else {
                "Token file passphrase is required"
            }
        }

        val username = identity.username.trim().ifEmpty {
            if (identity.authMode == AuthMode.XOAUTH2) email else ""
        }
        val cleaned = identity.copy(
            email = email,
            displayName = identity.displayName.trim(),
            smtpHost = host,
            username = username,
            tokenFilePath = identity.tokenFilePath.trim(),
        )
        val updated = if (config.identities.any { it.id == cleaned.id }) {
            config.identities.map { if (it.id == cleaned.id) cleaned else it }
        } else {
            config.identities + cleaned
        }
        if (secret.isNotBlank()) {
            runCatching { credentialStore.write(cleaned.id, secret) }
                .onFailure { return "Could not store credential: ${it.message ?: it}" }
        }
        save(config.copy(identities = updated))
        return null
    }

    fun removeIdentity(id: String) {
        credentialStore.remove(id)
        save(config.copy(identities = config.identities.filterNot { it.id == id }))
    }

    fun moveIdentity(id: String, delta: Int) {
        val list = config.identities.toMutableList()
        val index = list.indexOfFirst { it.id == id }
        val target = index + delta
        if (index < 0 || target !in list.indices) return
        val item = list.removeAt(index)
        list.add(target, item)
        save(config.copy(identities = list))
    }

    fun setDefaultReplyAll(enabled: Boolean) {
        save(config.copy(defaultReplyAll = enabled))
    }

    fun setNotifySound(enabled: Boolean) {
        save(config.copy(notifySound = enabled))
    }

    fun setNotifyShowContent(enabled: Boolean) {
        save(config.copy(notifyShowContent = enabled))
    }

    fun setNotifyPollSeconds(seconds: Int) {
        val clamped = seconds.coerceIn(30, 86_400)
        save(config.copy(notifyPollSeconds = clamped))
        MailNotificationService.start(getApplication())
    }

    fun setDefaultMime(preference: MimePreference) {
        save(config.copy(defaultMime = preference))
    }

    fun defaultIdentity(): SenderIdentity? = config.identities.firstOrNull()

    fun addressSuggestions(query: String): List<AddressEntry> = addressBook.suggestions(query)

    fun openComposeNew() {
        val identity = defaultIdentity()
        if (identity == null) {
            toast = "Add a sender identity in Settings first"
            return
        }
        composeSeed = ComposeSeed(
            identityId = identity.id,
            body = MailText.signatureBlock(identity.signature),
        )
        composeError = null
        identityTestResult = null
        screen = Screen.Compose
        saveNavigation()
        refreshAddressBook()
    }

    fun replyCurrent(all: Boolean) {
        val folderId = (screen as? Screen.Message)?.folderId ?: return
        val folder = folder(folderId) ?: return
        val entry = currentEntry ?: return
        val message = parsed ?: return
        openReply(folder, entry, message, all)
    }

    fun forwardCurrent() {
        val folderId = (screen as? Screen.Message)?.folderId ?: return
        val folder = folder(folderId) ?: return
        val entry = currentEntry ?: return
        val message = parsed ?: return
        val identity = defaultIdentity()
        if (identity == null) {
            toast = "Add a sender identity in Settings first"
            return
        }
        composeSeed = ComposeSeed(
            identityId = identity.id,
            subject = MailText.prefixedSubject(message.subject, "Fwd:"),
            body = MailText.quotedBody(message) + MailText.signatureBlock(identity.signature),
            replyFolderId = folder.id,
            replyPath = entry.relativePath,
        )
        composeError = null
        identityTestResult = null
        screen = Screen.Compose
        saveNavigation()
        refreshAddressBook()
    }

    private fun openReply(
        folder: MaildirFolder,
        entry: MaildirEntry,
        message: ParsedMessage,
        all: Boolean,
    ) {
        val own = config.identities.map { it.email.lowercase() }.toSet()
        val originalRecipients = MailText.addressesOf(message.to) + MailText.addressesOf(message.cc)
        val identity = config.identities.firstOrNull { it.email.lowercase() in originalRecipients }
            ?: defaultIdentity()
        if (identity == null) {
            toast = "Add a sender identity in Settings first"
            return
        }

        val fromAddresses = MailText.addressesOf(message.from).toSet()
        val cc = if (all) {
            (MailText.recipientLines(message.to) + MailText.recipientLines(message.cc))
                .filter { line -> MailText.addressesOf(line).none { it in own || it in fromAddresses } }
                .distinct()
                .joinToString(", ")
        } else {
            ""
        }

        composeSeed = ComposeSeed(
            identityId = identity.id,
            to = MailText.recipientLines(message.from).joinToString(", "),
            cc = cc,
            subject = MailText.prefixedSubject(message.subject, "Re:"),
            body = MailText.quotedBody(message) + MailText.signatureBlock(identity.signature),
            inReplyTo = message.messageId,
            references = MailText.referencesFor(message),
            replyFolderId = folder.id,
            replyPath = entry.relativePath,
        )
        composeError = null
        identityTestResult = null
        screen = Screen.Compose
        saveNavigation()
        refreshAddressBook()
    }

    fun openDraft(folderId: String, relativePath: String) {
        val folder = folder(folderId) ?: return
        viewModelScope.launch {
            val seed = withContext(Dispatchers.IO) {
                runCatching { readDraft(folder, relativePath) }.getOrNull()
            }
            if (seed == null) {
                toast = "Could not open draft"
                return@launch
            }
            composeSeed = seed
            composeError = null
            identityTestResult = null
            screen = Screen.Compose
            saveNavigation()
            refreshAddressBook()
        }
    }

    fun openIdentityPicker(fields: ComposeFields, attachments: List<OutgoingAttachment>) {
        composeSeed = composeSeed?.copy(
            identityId = fields.identityId,
            to = fields.to,
            cc = fields.cc,
            bcc = fields.bcc,
            subject = fields.subject,
            body = fields.body,
            attachments = attachments,
        )
        screen = Screen.PickIdentity
        saveNavigation()
    }

    fun chooseIdentity(id: String) {
        val seed = composeSeed ?: return
        val next = identity(id)
        composeSeed = if (next == null) {
            seed.copy(identityId = id)
        } else {
            seed.copy(
                identityId = id,
                body = MailText.replaceSignature(seed.body, next.signature),
            )
        }
        screen = Screen.Compose
        saveNavigation()
    }

    fun sendCompose(fields: ComposeFields, attachments: List<OutgoingAttachment>) {
        if (sending) return
        val seed = composeSeed ?: return
        val identity = identity(fields.identityId)
        if (identity == null) {
            composeError = "Choose a sender identity"
            return
        }
        if (!MailComposer.isValidAddressList(fields.to) ||
            !MailComposer.isValidAddressList(fields.cc) ||
            !MailComposer.isValidAddressList(fields.bcc)
        ) {
            composeError = "Check the recipient addresses"
            return
        }
        if (fields.to.isBlank() && fields.cc.isBlank() && fields.bcc.isBlank()) {
            composeError = "Add at least one recipient"
            return
        }
        val recordFolder = config.folders.firstOrNull { it.role == FolderRole.ARCHIVE }
        if (recordFolder == null) {
            composeError = "No archive maildir configured for sent copies"
            return
        }

        sending = true
        composeError = null
        viewModelScope.launch {
            val outcome = withContext(Dispatchers.IO) {
                runCatching {
                    val body = fields.body
                    val mail = OutgoingMail(
                        identity = identity,
                        to = fields.to,
                        cc = fields.cc,
                        bcc = fields.bcc,
                        subject = fields.subject,
                        body = body,
                        attachments = attachments,
                        inReplyTo = seed.inReplyTo,
                        references = seed.references,
                    )
                    val message = MailComposer.build(getApplication(), mail)
                    smtpClient.send(identity, message)
                    val copied = storeFor(recordFolder).append(MailComposer.toBytes(message), "S")
                    if (copied == null) {
                        throw IllegalStateException(
                            "Sent, but could not save a copy to ${recordFolder.displayName}",
                        )
                    }
                    seed.draftPath?.let { path ->
                        seed.draftFolderId?.let { id ->
                            folder(id)?.let { storeFor(it).deletePath(path) }
                        }
                    }
                    seed.replyPath?.let { path ->
                        seed.replyFolderId?.let { id ->
                            folder(id)?.let { storeFor(it).addFlag(path, 'R') }
                        }
                    }
                }
            }
            sending = false
            outcome.onSuccess {
                toast = "Message sent"
                composeSeed = null
                screen = screenAfterCompose(seed)
                saveNavigation()
                (screen as? Screen.Messages)?.let { refreshFolderFor(it.folderId) }
            }.onFailure {
                composeError = it.message ?: it.toString()
            }
        }
    }

    fun postponeCompose(fields: ComposeFields, attachments: List<OutgoingAttachment>) {
        if (posting || sending) return
        val seed = composeSeed ?: return
        val identity = identity(fields.identityId)
        if (identity == null) {
            composeError = "Choose a sender identity"
            return
        }
        val draftsFolder = config.folders.firstOrNull { it.role == FolderRole.POSTPONED }
        if (draftsFolder == null) {
            composeError = "No Postponed maildir configured"
            return
        }
        if (fields.to.isBlank() && fields.cc.isBlank() && fields.bcc.isBlank() &&
            fields.subject.isBlank() && fields.body.isBlank()
        ) {
            composeError = "Nothing to save"
            return
        }

        posting = true
        composeError = null
        viewModelScope.launch {
            val outcome = withContext(Dispatchers.IO) {
                runCatching {
                    val mail = OutgoingMail(
                        identity = identity,
                        to = fields.to,
                        cc = fields.cc,
                        bcc = fields.bcc,
                        subject = fields.subject,
                        body = fields.body,
                        attachments = attachments,
                        inReplyTo = seed.inReplyTo,
                        references = seed.references,
                    )
                    val message = MailComposer.build(getApplication(), mail)
                    message.setHeader("X-Maildirt-Identity", identity.id)
                    message.saveChanges()
                    val stamped = MimeMessage(message)
                    val path = storeFor(draftsFolder).append(MailComposer.toBytes(stamped), "D")
                        ?: throw IllegalStateException("Could not save draft")
                    seed.draftPath?.let { old ->
                        seed.draftFolderId?.let { id ->
                            folder(id)?.let { storeFor(it).deletePath(old) }
                        }
                    }
                    path
                }
            }
            posting = false
            outcome.onSuccess {
                toast = "Draft saved"
                composeSeed = null
                screen = Screen.Messages(draftsFolder.id)
                saveNavigation()
                refreshFolder()
            }.onFailure {
                composeError = it.message ?: it.toString()
            }
        }
    }

    fun discardCompose() {
        val seed = composeSeed
        if (seed == null) {
            screen = messagesScreen()
            saveNavigation()
            return
        }
        composeSeed = null
        composeError = null
        val target = screenAfterCompose(seed)
        screen = target
        saveNavigation()
        if (target is Screen.Messages) refreshFolder()
        viewModelScope.launch(Dispatchers.IO) {
            seed.draftPath?.let { path ->
                seed.draftFolderId?.let { id ->
                    folder(id)?.let { storeFor(it).deletePath(path) }
                }
            }
        }
    }

    fun testIdentityForm(identity: SenderIdentity, secret: String) {
        if (testingIdentity != null) return
        val effective = secret.ifBlank { credentialStore.read(identity.id) }
        if (effective.isNullOrBlank()) {
            identityTestResult = "Failed: enter the password or passphrase first"
            return
        }
        testingIdentity = identity.id
        identityTestResult = null
        viewModelScope.launch {
            val result = withContext(Dispatchers.IO) {
                runCatching { smtpClient.test(identity, effective) }
            }
            testingIdentity = null
            identityTestResult = result.getOrElse { "Failed: ${it.message ?: it}" }
        }
    }

    fun clearIdentityTest() {
        identityTestResult = null
    }

    private fun screenAfterCompose(seed: ComposeSeed): Screen = when {
        seed.draftFolderId != null -> Screen.Messages(seed.draftFolderId)
        seed.replyFolderId != null -> Screen.Messages(seed.replyFolderId)
        else -> messagesScreen()
    }

    private fun refreshFolderFor(folderId: String) {
        if ((screen as? Screen.Messages)?.folderId == folderId) refreshFolder()
    }

    private fun refreshAddressBook() {
        viewModelScope.launch(Dispatchers.IO) {
            runCatching { addressBook.refresh(config.folders) }
        }
    }

    private fun readDraft(folder: MaildirFolder, relativePath: String): ComposeSeed {
        val file = storeFor(folder).fileFor(relativePath)
        val message = file.inputStream().use { input ->
            MimeMessage(MailComposer.newSession(), input)
        }
        val storedIdentity = message.getHeader("X-Maildirt-Identity")?.firstOrNull()
        val identityId = storedIdentity?.takeIf { identity(it) != null }
            ?: defaultIdentity()?.id
            ?: throw IllegalStateException("No sender identity configured")
        val app = getApplication<Application>()
        val attachments = runCatching {
            mimeReader.parse(
                file,
                "draft_${folder.id.hashCode()}_${relativePath.hashCode()}",
            ).attachments.mapNotNull { attachment ->
                runCatching {
                    OutgoingAttachment(
                        uri = FileProvider.getUriForFile(
                            app,
                            "${app.packageName}.fileprovider",
                            attachment.file,
                        ),
                        name = attachment.name,
                        mimeType = attachment.mimeType,
                    )
                }.getOrNull()
            }
        }.getOrDefault(emptyList())
        return ComposeSeed(
            identityId = identityId,
            to = recipientsOf(message, javax.mail.Message.RecipientType.TO),
            cc = recipientsOf(message, javax.mail.Message.RecipientType.CC),
            bcc = recipientsOf(message, javax.mail.Message.RecipientType.BCC),
            subject = message.subject.orEmpty(),
            body = draftBody(message),
            attachments = attachments,
            inReplyTo = message.getHeader("In-Reply-To")?.firstOrNull(),
            references = message.getHeader("References")?.firstOrNull(),
            draftFolderId = folder.id,
            draftPath = relativePath,
        )
    }

    private fun recipientsOf(message: MimeMessage, type: javax.mail.Message.RecipientType): String =
        runCatching {
            message.getRecipients(type)?.joinToString(", ") { address ->
                if (address is InternetAddress) HeaderParser.format(address) else address.toString()
            }.orEmpty()
        }.getOrDefault("")

    private fun draftBody(message: MimeMessage): String = runCatching {
        if (message.isMimeType("text/plain")) return message.content as? String ?: ""
        if (message.isMimeType("text/html")) {
            return MailText.stripHtml(message.content as? String ?: "")
        }
        val content = message.content
        if (content is javax.mail.Multipart) {
            for (index in 0 until content.count) {
                val part = content.getBodyPart(index)
                if (part.isMimeType("text/plain")) return part.content as? String ?: ""
            }
            for (index in 0 until content.count) {
                val part = content.getBodyPart(index)
                if (part.isMimeType("text/html")) return MailText.stripHtml(part.content as? String ?: "")
            }
        }
        ""
    }.getOrDefault("")

    fun openMovePicker(forSelection: Boolean) {
        val folderId = when (val current = screen) {
            is Screen.Messages -> current.folderId
            is Screen.Message -> current.folderId
            else -> return
        }
        val relativePath = if (forSelection) null else currentEntry?.relativePath
        screen = Screen.PickMaildir(folderId, relativePath, forSelection)
        saveNavigation()
    }

    fun back(): Boolean {
        if (selection.isNotEmpty()) {
            selection = emptySet()
            return true
        }
        if (searchActive) {
            clearSearch()
            return true
        }
        return when (val current = screen) {
            Screen.Settings -> {
                screen = messagesScreen()
                saveNavigation()
                true
            }
            is Screen.Messages -> false
            is Screen.Message -> {
                screen = Screen.Messages(current.folderId)
                saveNavigation()
                refreshFolder()
                true
            }
            is Screen.CalendarReply -> {
                screen = Screen.Message(current.folderId, current.relativePath)
                saveNavigation()
                true
            }
            is Screen.FolderEditor -> {
                screen = Screen.Settings
                saveNavigation()
                true
            }
            is Screen.IdentityEditor -> {
                screen = Screen.Settings
                saveNavigation()
                true
            }
            is Screen.Compose -> {
                val seed = composeSeed
                composeSeed = null
                screen = if (seed == null) messagesScreen() else screenAfterCompose(seed)
                saveNavigation()
                true
            }
            Screen.PickIdentity -> {
                screen = Screen.Compose
                saveNavigation()
                true
            }
            is Screen.GoToItem -> {
                screen = Screen.Messages(current.folderId)
                saveNavigation()
                true
            }
            is Screen.PickMaildir -> {
                screen = if (current.forSelection || current.relativePath == null) {
                    Screen.Messages(current.folderId)
                } else {
                    Screen.Message(current.folderId, current.relativePath)
                }
                saveNavigation()
                true
            }
        }
    }

    fun addFolder(path: String, role: FolderRole): String? {
        val validation = validatePath(path, null)
        if (validation != null) return validation
        val root = canonical(path)

        val id = UUID.randomUUID().toString()
        var folders = config.folders
        if (role == FolderRole.INBOX || role == FolderRole.ARCHIVE) {
            folders = folders.map {
                if (it.role == role) it.copy(role = FolderRole.OTHER) else it
            }
        }
        folders = folders + MaildirFolder(id = id, path = root.path, role = role)
        save(config.copy(folders = folders))
        return null
    }

    fun updateFolder(id: String, path: String, role: FolderRole): String? {
        if (config.folders.none { it.id == id }) return "Maildir not found"
        val validation = validatePath(path, id)
        if (validation != null) return validation
        val root = canonical(path)

        var folders = config.folders.map {
            if (it.id == id) it.copy(path = root.path, role = role) else it
        }
        if (role == FolderRole.INBOX || role == FolderRole.ARCHIVE) {
            folders = folders.map {
                if (it.id != id && it.role == role) it.copy(role = FolderRole.OTHER) else it
            }
        }
        save(config.copy(folders = folders))
        return null
    }

    fun removeFolder(id: String) {
        val updated = config.copy(folders = config.folders.filterNot { it.id == id })
        if (updated.selectedFolderId == id) {
            save(updated.copy(selectedFolderId = null))
        } else {
            save(updated)
        }
        viewModelScope.launch(Dispatchers.IO) {
            searchIndex.pruneFolders(config.folders.map { it.path }.toSet())
        }
    }

    fun moveFolder(id: String, delta: Int) {
        val list = config.folders.toMutableList()
        val index = list.indexOfFirst { it.id == id }
        val target = index + delta
        if (index < 0 || target !in list.indices) return
        val item = list.removeAt(index)
        list.add(target, item)
        save(config.copy(folders = list))
    }

    private fun validatePath(path: String, ignoreId: String?): String? {
        val trimmed = path.trim().trimEnd('/')
        if (trimmed.isEmpty()) return "Path is required"
        val root = File(trimmed)
        if (!File(root, "cur").isDirectory ||
            !File(root, "new").isDirectory ||
            !File(root, "tmp").isDirectory
        ) {
            return "Not a maildir: expected cur/, new/ and tmp/ under this path"
        }
        if (!root.canRead()) return "Cannot read this path; check storage access"
        val canonical = canonical(trimmed).path
        if (config.folders.any { it.path == canonical && it.id != ignoreId }) {
            return "This path is already added"
        }
        return null
    }

    private fun canonical(path: String): File =
        runCatching { File(path.trim().trimEnd('/')).canonicalFile }
            .getOrDefault(File(path.trim().trimEnd('/')))

    private fun replaceEntry(oldPath: String, updated: MaildirEntry) {
        messages = messages.map { if (it.relativePath == oldPath) updated else it }
    }

    private fun save(updated: AppConfig) {
        config = updated
        runCatching { configStore.save(updated) }
    }

    private fun saveNavigation() {
        val folderId = when (val current = screen) {
            is Screen.Messages -> current.folderId
            is Screen.Message -> current.folderId
            is Screen.CalendarReply -> current.folderId
            is Screen.PickMaildir -> current.folderId
            is Screen.GoToItem -> current.folderId
            else -> null
        }
        val relativePath = when (val current = screen) {
            is Screen.Message -> currentEntry?.relativePath ?: current.relativePath
            is Screen.CalendarReply -> current.relativePath
            is Screen.PickMaildir -> current.relativePath
            else -> null
        }
        val type = when (val current = screen) {
            Screen.Settings -> "settings"
            is Screen.Messages -> "messages"
            is Screen.Message -> "message"
            is Screen.CalendarReply -> "message"
            is Screen.FolderEditor -> "settings"
            is Screen.IdentityEditor -> "settings"
            is Screen.Compose -> "messages"
            Screen.PickIdentity -> "messages"
            is Screen.GoToItem -> "messages"
            is Screen.PickMaildir -> if (current.relativePath != null) "message" else "messages"
        }
        navPrefs.edit()
            .putString("type", type)
            .putString("folderId", folderId)
            .putString("relativePath", relativePath)
            .apply()
    }

    private fun restoreNavigation() {
        val type = navPrefs.getString("type", null) ?: return
        val folderId = navPrefs.getString("folderId", null)
        val relativePath = navPrefs.getString("relativePath", null)
        when (type) {
            "settings" -> screen = Screen.Settings
            "messages", "folders" -> {
                if (folderId != null && folder(folderId) != null) {
                    activeFolderId = folderId
                    screen = Screen.Messages(folderId)
                    folder(folderId)?.let { startWatching(it) }
                    refreshFolder()
                }
            }
            "message" -> {
                if (folderId != null && relativePath != null && folder(folderId) != null) {
                    activeFolderId = folderId
                    screen = Screen.Messages(folderId)
                    pendingMessagePath = relativePath
                    refreshFolder()
                }
            }
        }
    }
}

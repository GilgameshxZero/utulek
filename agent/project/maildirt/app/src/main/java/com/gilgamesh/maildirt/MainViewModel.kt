package com.gilgamesh.maildirt

import android.app.Application
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.File
import java.util.UUID

sealed interface Screen {
    data object Folders : Screen
    data class Messages(val folderId: String) : Screen
    data class Message(val folderId: String, val relativePath: String) : Screen
    data object Settings : Screen
}

class MainViewModel(application: Application) : AndroidViewModel(application) {
    private val configStore = ConfigStore(application)
    private val mimeReader = MimeReader(application)
    private val indexStore = FolderIndexStore(application)

    var config by mutableStateOf(configStore.load())
        private set
    var screen by mutableStateOf<Screen>(Screen.Folders)
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
    var remoteImages by mutableStateOf(false)
        private set
    var viewAlternative by mutableStateOf<String?>(null)
        private set
    var currentEntry by mutableStateOf<MaildirEntry?>(null)
        private set
    var actionInProgress by mutableStateOf(false)
        private set
    var toast by mutableStateOf<String?>(null)
        private set

    private var listJob: Job? = null
    private var messageJob: Job? = null

    fun folders(): List<MaildirFolder> = config.folders

    fun folder(id: String): MaildirFolder? = config.folders.firstOrNull { it.id == id }

    fun archiveFolder(): MaildirFolder? =
        config.folders.firstOrNull { it.role == FolderRole.ARCHIVE }

    private fun storeFor(folder: MaildirFolder) = MaildirFolderStore(folder, indexStore)

    fun openFolder(id: String) {
        listState = LazyListState()
        screen = Screen.Messages(id)
        refreshFolder()
    }

    fun refreshFolder() {
        val folderId = (screen as? Screen.Messages)?.folderId ?: return
        val folder = folder(folderId) ?: return
        listJob?.cancel()
        listLoading = true
        listError = null
        listJob = viewModelScope.launch {
            val result = withContext(Dispatchers.IO) {
                runCatching { storeFor(folder).list() }
            }
            result.onSuccess {
                messages = it
            }.onFailure {
                android.util.Log.e("maildirt", "listing failed for ${folder.path}", it)
                listError = it.message ?: it.toString()
            }
            listLoading = false
        }
    }

    fun openMessage(relativePath: String) {
        val folderId = (screen as? Screen.Messages)?.folderId ?: return
        val folder = folder(folderId) ?: return
        val entry = messages.firstOrNull { it.relativePath == relativePath } ?: return

        screen = Screen.Message(folderId, relativePath)
        currentEntry = entry
        parsed = null
        messageError = null
        messageLoading = true
        remoteImages = false
        viewAlternative = null
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
                }
            }.onFailure {
                messageError = it.message ?: it.toString()
            }
            messageLoading = false
        }
    }

    fun updateRemoteImages(enabled: Boolean) {
        remoteImages = enabled
    }

    fun chooseAlternative(mimeType: String?) {
        viewAlternative = mimeType
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
                refreshFolder()
            } else {
                toast = "Could not delete message"
            }
        }
    }

    fun moveCurrent(targetId: String) {
        val folderId = (screen as? Screen.Message)?.folderId ?: return
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
                refreshFolder()
            } else {
                toast = "Could not move message"
            }
        }
    }

    fun consumeToast() {
        toast = null
    }

    fun openSettings() {
        screen = Screen.Settings
    }

    fun back(): Boolean = when (val current = screen) {
        Screen.Folders -> false
        Screen.Settings -> {
            screen = Screen.Folders
            true
        }
        is Screen.Messages -> {
            screen = Screen.Folders
            true
        }
        is Screen.Message -> {
            screen = Screen.Messages(current.folderId)
            refreshFolder()
            true
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
}

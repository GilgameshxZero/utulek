package com.gilgamesh.maildirt

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.graphics.Color as AndroidColor
import android.net.Uri
import android.os.Build
import android.provider.OpenableColumns
import android.view.MotionEvent
import android.view.ViewConfiguration
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.Toast
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.ScrollState
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.gestures.detectVerticalDragGestures
import androidx.compose.foundation.gestures.scrollBy
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListState
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.content.FileProvider
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.lifecycle.viewmodel.compose.viewModel
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.TimeZone
import java.util.UUID
import kotlin.math.abs
import kotlinx.coroutines.launch

@Composable
fun MaildirtApp(vm: MainViewModel = viewModel()) {
    val context = LocalContext.current
    val granted = rememberStorageGranted()
    val permissionLauncher = rememberLauncherForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { result -> granted.value = result }
    val contactsLauncher = rememberLauncherForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { }

    LaunchedEffect(granted.value) {
        if (!granted.value) return@LaunchedEffect
        if (context.checkSelfPermission(Manifest.permission.READ_CONTACTS) !=
            PackageManager.PERMISSION_GRANTED
        ) {
            contactsLauncher.launch(Manifest.permission.READ_CONTACTS)
        }
    }

    val notificationsLauncher = rememberLauncherForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { }
    LaunchedEffect(Unit) {
        if (Build.VERSION.SDK_INT >= 33 &&
            context.checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) !=
            PackageManager.PERMISSION_GRANTED
        ) {
            notificationsLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
        }
    }

    LaunchedEffect(OpenRequests.pending) {
        OpenRequests.pending?.let { request ->
            vm.openFromNotification(request.folderId, request.folderRole, request.path)
            OpenRequests.pending = null
        }
    }

    val appLifecycle = LocalLifecycleOwner.current
    DisposableEffect(appLifecycle) {
        val observer = LifecycleEventObserver { _, event ->
            if (event == Lifecycle.Event.ON_RESUME) vm.onAppResume()
        }
        appLifecycle.lifecycle.addObserver(observer)
        onDispose { appLifecycle.lifecycle.removeObserver(observer) }
    }

    LaunchedEffect(vm.toast) {
        val message = vm.toast
        if (message != null) {
            Toast.makeText(context, message, Toast.LENGTH_SHORT).show()
            vm.consumeToast()
        }
    }

    if (!granted.value) {
        PermissionScreen(
            needsRuntimePermission = StorageAccess.needsRuntimePermission(),
            onGrant = {
                if (StorageAccess.needsRuntimePermission()) {
                    permissionLauncher.launch(Manifest.permission.WRITE_EXTERNAL_STORAGE)
                } else {
                    StorageAccess.openSystemSettings(context)
                }
            },
        )
        return
    }

    val screen = vm.screen
    BackHandler(enabled = screen !is Screen.Messages) { vm.back() }

    when (screen) {
        Screen.Settings -> SettingsScreen(vm)
        is Screen.Messages -> MessageListScreen(vm, screen.folderId)
        is Screen.Message -> MessageScreen(vm, screen.folderId)
        is Screen.CalendarReply -> CalendarReplyScreen(vm)
        is Screen.FolderEditor -> FolderEditorScreen(vm, screen)
        is Screen.IdentityEditor -> IdentityEditorScreen(vm, screen)
        Screen.Compose -> ComposeScreen(vm)
        Screen.PickIdentity -> IdentityPickerScreen(vm)
        is Screen.PickMaildir -> PickMaildirScreen(vm, screen)
        is Screen.GoToItem -> GoToItemScreen(vm, screen)
    }
}

@Composable
internal fun rememberStorageGranted(): MutableState<Boolean> {
    val context = LocalContext.current
    val state = remember { mutableStateOf(StorageAccess.hasAccess(context)) }
    val lifecycleOwner = LocalLifecycleOwner.current
    DisposableEffect(lifecycleOwner) {
        val observer = LifecycleEventObserver { _, event ->
            if (event == Lifecycle.Event.ON_RESUME) {
                state.value = StorageAccess.hasAccess(context)
            }
        }
        lifecycleOwner.lifecycle.addObserver(observer)
        onDispose { lifecycleOwner.lifecycle.removeObserver(observer) }
    }
    return state
}

@Composable
private fun PermissionScreen(needsRuntimePermission: Boolean, onGrant: () -> Unit) {
    Column(
        modifier = Modifier.fillMaxSize().padding(24.dp),
        verticalArrangement = Arrangement.Center,
    ) {
        Text("Storage access needed", style = MaterialTheme.typography.titleLarge)
        Spacer(Modifier.height(12.dp))
        Text(
            if (needsRuntimePermission) {
                "maildirt reads maildirs from shared storage. Grant storage access to continue."
            } else {
                "maildirt reads maildirs from shared storage. Allow \"All files access\" for maildirt in the system settings, then come back."
            },
            style = MaterialTheme.typography.bodyLarge,
        )
        Spacer(Modifier.height(20.dp))
        Button(onClick = onGrant) { Text("Grant access") }
    }
}

@Composable
internal fun AppTopBar(
    title: String,
    onBack: (() -> Unit)? = null,
    onTitleClick: (() -> Unit)? = null,
    actions: @Composable RowScope.() -> Unit = {},
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .height(46.dp)
            .background(Color.White)
            .padding(horizontal = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (onBack != null) {
            IconButton(onClick = onBack, modifier = Modifier.size(44.dp)) {
                Icon(
                    painter = painterResource(R.drawable.ic_back),
                    contentDescription = "Back",
                    modifier = Modifier.size(22.dp),
                )
            }
        } else {
            Spacer(Modifier.width(8.dp))
        }
        Row(
            modifier = Modifier
                .weight(1f)
                .clickable(enabled = onTitleClick != null) { onTitleClick?.invoke() },
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                title,
                fontSize = 17.sp,
                fontWeight = FontWeight.SemiBold,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.weight(1f, fill = false),
            )
            if (onTitleClick != null) {
                Icon(
                    painter = painterResource(R.drawable.ic_arrow_down),
                    contentDescription = "Switch",
                    modifier = Modifier.size(18.dp),
                )
            }
        }
        actions()
    }
}

@Composable
internal fun BarIcon(
    icon: Int,
    description: String,
    enabled: Boolean = true,
    onClick: () -> Unit,
) {
    IconButton(
        onClick = onClick,
        enabled = enabled,
        modifier = Modifier.size(44.dp),
    ) {
        Icon(
            painter = painterResource(icon),
            contentDescription = description,
            tint = if (enabled) Color.Black else InkDim,
            modifier = Modifier.size(22.dp),
        )
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun MessageListScreen(vm: MainViewModel, folderId: String) {
    val folder = vm.folder(folderId)
    val messages = vm.messages
    val loading = vm.listLoading
    val error = vm.listError
    val selection = vm.selection
    val selectionMode = selection.isNotEmpty()
    val ownAddresses = remember(vm.config.identities) {
        vm.config.identities
            .map { it.email.trim().lowercase() }
            .filter { it.isNotBlank() }
            .toSet()
    }
    var selectionMenu by remember { mutableStateOf(false) }
    var listMenu by remember { mutableStateOf(false) }
    var folderMenu by remember { mutableStateOf(false) }
    val folders = vm.folders()
    val searchActive = vm.searchActive
    val searchFocusRequester = remember { FocusRequester() }
    LaunchedEffect(searchActive) {
        if (searchActive) searchFocusRequester.requestFocus()
    }

    Scaffold(
        topBar = {
            if (selectionMode) {
                AppTopBar(
                    title = "${selection.size} selected",
                    onBack = { vm.clearSelection() },
                    actions = {
                        if (folder?.role == FolderRole.POSTPONED) {
                            // Drafts are edited individually; no archive/delete from the list.
                        } else if (folder?.role == FolderRole.ARCHIVE) {
                            IconButton(
                                onClick = { vm.deleteSelected() },
                                enabled = !vm.actionInProgress,
                            ) {
                                Icon(
                                    painter = painterResource(R.drawable.ic_delete),
                                    contentDescription = "Delete selected",
                                )
                            }
                        } else {
                            IconButton(
                                onClick = { vm.archiveSelected() },
                                enabled = !vm.actionInProgress,
                            ) {
                                Icon(
                                    painter = painterResource(R.drawable.ic_archive),
                                    contentDescription = "Archive selected",
                                )
                            }
                        }
                        Box {
                            IconButton(onClick = { selectionMenu = true }) {
                                Icon(
                                    painter = painterResource(R.drawable.ic_more_vert),
                                    contentDescription = "Selection actions",
                                )
                            }
                            DropdownMenu(
                                expanded = selectionMenu,
                                onDismissRequest = { selectionMenu = false },
                            ) {
                                DropdownMenuItem(
                                    text = { Text("Mark read") },
                                    enabled = !vm.actionInProgress,
                                    onClick = {
                                        selectionMenu = false
                                        vm.markSelected(true)
                                    },
                                )
                                DropdownMenuItem(
                                    text = { Text("Mark unread") },
                                    enabled = !vm.actionInProgress,
                                    onClick = {
                                        selectionMenu = false
                                        vm.markSelected(false)
                                    },
                                )
                                DropdownMenuItem(
                                    text = { Text("Move to maildir...") },
                                    enabled = folder?.role != FolderRole.POSTPONED &&
                                        vm.folders().any { it.id != folderId } &&
                                        !vm.actionInProgress,
                                    onClick = {
                                        selectionMenu = false
                                        vm.openMovePicker(true)
                                    },
                                )
                                DropdownMenuItem(
                                    text = { Text("Select all") },
                                    enabled = !vm.actionInProgress,
                                    onClick = {
                                        selectionMenu = false
                                        vm.selectAll()
                                    },
                                )
                            }
                        }
                    },
                )
            } else {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(46.dp)
                        .background(Color.White)
                        .padding(horizontal = 4.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    if (searchActive) {
                        BasicTextField(
                            value = vm.searchQuery,
                            onValueChange = { vm.updateSearchQuery(it) },
                            singleLine = true,
                            textStyle = MaterialTheme.typography.bodyLarge.copy(color = Color.Black),
                            decorationBox = { inner ->
                                Box(Modifier.fillMaxWidth()) {
                                    if (vm.searchQuery.isEmpty()) {
                                        Text(
                                            "Search maildir",
                                            fontSize = 16.sp,
                                            color = InkDim,
                                        )
                                    }
                                    inner()
                                }
                            },
                            modifier = Modifier
                                .weight(1f)
                                .focusRequester(searchFocusRequester),
                        )
                        BarIcon(
                            if (vm.searchBody) R.drawable.ic_search_all else R.drawable.ic_search_headers,
                            if (vm.searchBody) "Searching headers and body" else "Searching headers only",
                        ) { vm.toggleSearchScope() }
                        BarIcon(
                            R.drawable.ic_chevron_left,
                            "Previous match",
                            enabled = !vm.searchLoading,
                        ) { vm.previousSearchMatch() }
                        BarIcon(
                            R.drawable.ic_chevron_right,
                            "Next match",
                            enabled = !vm.searchLoading,
                        ) { vm.nextSearchMatch() }
                        BarIcon(R.drawable.ic_close, "Close search") { vm.clearSearch() }
                    } else {
                        Box {
                            AppTopBar(
                                title = folder?.displayName ?: "maildirt",
                                onTitleClick = if (folders.size > 1) {
                                    { folderMenu = true }
                                } else {
                                    null
                                },
                                actions = {
                                    BarIcon(R.drawable.ic_search, "Search") { vm.beginSearch() }
                                    BarIcon(R.drawable.ic_add, "New message") {
                                        vm.openComposeNew()
                                    }
                                    Box {
                                        BarIcon(R.drawable.ic_more_vert, "More actions") {
                                            listMenu = true
                                        }
                                        DropdownMenu(
                                            expanded = listMenu,
                                            onDismissRequest = { listMenu = false },
                                        ) {
                                            DropdownMenuItem(
                                                text = { Text("Refresh") },
                                                onClick = {
                                                    listMenu = false
                                                    vm.refreshAll()
                                                },
                                            )
                                            DropdownMenuItem(
                                                text = { Text("Settings") },
                                                onClick = {
                                                    listMenu = false
                                                    vm.openSettings()
                                                },
                                            )
                                            DropdownMenuItem(
                                                text = { Text("Go to item...") },
                                                onClick = {
                                                    listMenu = false
                                                    vm.openGoToItem()
                                                },
                                            )
                                        }
                                    }
                                },
                            )
                            DropdownMenu(
                                expanded = folderMenu,
                                onDismissRequest = { folderMenu = false },
                            ) {
                                for (candidate in folders) {
                                    DropdownMenuItem(
                                        text = {
                                            Text(
                                                candidate.displayName,
                                                fontSize = 16.sp,
                                                fontWeight = if (candidate.id == folderId) {
                                                    FontWeight.SemiBold
                                                } else {
                                                    FontWeight.Normal
                                                },
                                            )
                                        },
                                        trailingIcon = if (candidate.id == folderId) {
                                            {
                                                Icon(
                                                    painter = painterResource(R.drawable.ic_check),
                                                    contentDescription = "Current maildir",
                                                    modifier = Modifier.size(18.dp),
                                                )
                                            }
                                        } else {
                                            null
                                        },
                                        onClick = {
                                            folderMenu = false
                                            vm.openFolder(candidate.id)
                                        },
                                    )
                                }
                            }
                        }
                    }
                }
            }
        },
    ) { padding ->
        Column(modifier = Modifier.fillMaxSize().padding(padding)) {
            when {
                folder == null -> Column(Modifier.fillMaxSize().padding(24.dp)) {
                    Text("No maildirs configured.", fontSize = 14.sp, color = InkDim)
                    Spacer(Modifier.height(12.dp))
                    Text(
                        "Open settings",
                        fontSize = 15.sp,
                        fontWeight = FontWeight.SemiBold,
                        modifier = Modifier
                            .clickable { vm.openSettings() }
                            .padding(vertical = 8.dp),
                    )
                }
                loading && messages.isEmpty() -> Text(
                    "Loading...",
                    fontSize = 14.sp,
                    color = InkDim,
                    modifier = Modifier.padding(16.dp),
                )
                error != null -> Text(
                    "Could not read this folder:\n$error",
                    fontSize = 14.sp,
                    color = InkDim,
                    modifier = Modifier.padding(16.dp),
                )
                messages.isEmpty() -> Text(
                    "No messages.",
                    fontSize = 14.sp,
                    color = InkDim,
                    modifier = Modifier.padding(16.dp),
                )
                else -> {
                    val paged = vm.config.pagedScrolling
                    val base = Modifier.fillMaxSize()
                    LazyColumn(
                        state = vm.listState,
                        userScrollEnabled = !paged,
                        modifier = if (paged) base.pagedScroll(vm.listState) else base,
                    ) {
                        items(messages, key = { it.relativePath }) { entry ->
                            MessageRow(
                                entry = entry,
                                ownAddresses = ownAddresses,
                                selected = entry.relativePath in selection,
                                selectionMode = selectionMode,
                                searchMatch = entry.relativePath == vm.currentSearchMatch,
                                onClick = {
                                    if (selectionMode) {
                                        vm.toggleSelection(entry.relativePath)
                                    } else {
                                        vm.openMessage(entry.relativePath)
                                    }
                                },
                                onLongClick = { vm.startSelection(entry.relativePath) },
                            )
                        }
                    }
                }
            }
        }
    }

}

@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun MessageRow(
    entry: MaildirEntry,
    ownAddresses: Set<String>,
    selected: Boolean,
    selectionMode: Boolean,
    searchMatch: Boolean,
    onClick: () -> Unit,
    onLongClick: () -> Unit,
) {
    val fromSelf = remember(entry.fromAddress, ownAddresses) {
        ownAddresses.isNotEmpty() &&
            MailText.addressesOf(entry.fromAddress).any { it in ownAddresses }
    }
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .combinedClickable(onClick = onClick, onLongClick = onLongClick)
            .padding(horizontal = 12.dp, vertical = 8.dp),
        verticalAlignment = Alignment.Top,
    ) {
        Column(Modifier.weight(1f)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                if (selectionMode && selected) {
                    Icon(
                        painter = painterResource(R.drawable.ic_check),
                        contentDescription = "Selected",
                        modifier = Modifier.padding(end = 5.dp).size(18.dp),
                    )
                } else if (searchMatch) {
                    Text(
                        "\u25B6",
                        fontSize = 10.sp,
                        modifier = Modifier.padding(end = 5.dp),
                    )
                }
                if (fromSelf) {
                    Icon(
                        painter = painterResource(R.drawable.ic_send),
                        contentDescription = "From your address",
                        tint = InkDim,
                        modifier = Modifier.padding(end = 4.dp).size(13.dp),
                    )
                }
                Text(
                    entry.from,
                    fontSize = 16.sp,
                    fontWeight = FontWeight.Normal,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f),
                )
                if (entry.unread) {
                    Text(
                        "\u25CF",
                        fontSize = 7.sp,
                        modifier = Modifier.padding(start = 5.dp),
                    )
                }
                if (entry.hasAttachments) {
                    Icon(
                        painter = painterResource(R.drawable.ic_attachment),
                        contentDescription = "Attachment",
                        tint = InkDim,
                        modifier = Modifier.padding(start = 5.dp).size(13.dp),
                    )
                }
                if (entry.replied) {
                    Icon(
                        painter = painterResource(R.drawable.ic_reply),
                        contentDescription = "Replied",
                        tint = InkDim,
                        modifier = Modifier.padding(start = 4.dp).size(13.dp),
                    )
                }
                Spacer(Modifier.width(6.dp))
                Text(formatListDate(entry.date), fontSize = 11.sp, color = InkDim)
            }
            Spacer(Modifier.height(1.dp))
            Text(
                entry.subject,
                fontSize = 13.sp,
                color = InkDim,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun MessageScreen(vm: MainViewModel, folderId: String) {
    val context = LocalContext.current
    val folder = vm.folder(folderId)
    val entry = vm.currentEntry
    val parsed = vm.parsed
    val loading = vm.messageLoading
    val error = vm.messageError
    val currentMime = vm.viewAlternative ?: parsed?.let {
        defaultMime(it, vm.config.defaultMime == MimePreference.HTML)
    }
    val showSource = vm.showSource
    val html = remember(parsed, currentMime, showSource, vm.rawSource) {
        if (showSource) {
            vm.rawSource?.let { renderSourceHtml(it) }
        } else {
            parsed?.let { renderMessageHtml(it, currentMime) }
        }
    }
    var menuExpanded by remember { mutableStateOf(false) }
    val pagedScrolling = vm.config.pagedScrolling

    Scaffold(
        topBar = {
            AppTopBar(
                    title = if (showSource) "Source" else parsed?.subject ?: "Message",
                    onBack = { vm.back() },
                    actions = {
                        if (folder?.role == FolderRole.POSTPONED) {
                            IconButton(
                                onClick = {
                                    val path = entry?.relativePath ?: return@IconButton
                                    vm.openDraft(folderId, path)
                                },
                            ) {
                                Icon(
                                    painter = painterResource(R.drawable.ic_edit),
                                    contentDescription = "Edit draft",
                                )
                            }
                        } else {
                            IconButton(
                                onClick = {
                                    vm.replyCurrent(vm.config.defaultReplyAll)
                                },
                                enabled = parsed != null,
                            ) {
                                Icon(
                                    painter = painterResource(R.drawable.ic_reply),
                                    contentDescription = "Reply",
                                )
                            }
                            if (folder?.role == FolderRole.ARCHIVE) {
                                IconButton(
                                    onClick = { vm.deleteCurrent() },
                                    enabled = !vm.actionInProgress,
                                ) {
                                    Icon(
                                        painter = painterResource(R.drawable.ic_delete),
                                        contentDescription = "Delete",
                                    )
                                }
                            } else {
                                IconButton(
                                    onClick = { vm.archiveCurrent() },
                                    enabled = !vm.actionInProgress,
                                ) {
                                    Icon(
                                        painter = painterResource(R.drawable.ic_archive),
                                        contentDescription = "Archive",
                                    )
                                }
                            }
                        }
                        Box {
                            IconButton(onClick = { menuExpanded = true }) {
                                Icon(
                                    painter = painterResource(R.drawable.ic_more_vert),
                                    contentDescription = "More actions",
                                )
                            }
                            DropdownMenu(
                                expanded = menuExpanded,
                                onDismissRequest = { menuExpanded = false },
                            ) {
                                if (!showSource && parsed?.calendar?.canRespond == true) {
                                    DropdownMenuItem(
                                        text = { Text("Respond to invite") },
                                        enabled = !vm.calendarReplySending,
                                        onClick = {
                                            menuExpanded = false
                                            vm.openCalendarReply()
                                        },
                                    )
                                }
                                DropdownMenuItem(
                                    text = { Text("Reply") },
                                    enabled = parsed != null,
                                    onClick = {
                                        menuExpanded = false
                                        vm.replyCurrent(false)
                                    },
                                )
                                DropdownMenuItem(
                                    text = { Text("Reply all") },
                                    enabled = parsed != null,
                                    onClick = {
                                        menuExpanded = false
                                        vm.replyCurrent(true)
                                    },
                                )
                                DropdownMenuItem(
                                    text = { Text("Forward") },
                                    enabled = parsed != null,
                                    onClick = {
                                        menuExpanded = false
                                        vm.forwardCurrent()
                                    },
                                )
                                DropdownMenuItem(
                                    text = { Text("Mark as unread") },
                                    enabled = entry?.unread == false && !vm.actionInProgress,
                                    onClick = {
                                        menuExpanded = false
                                        vm.markCurrentUnread()
                                    },
                                )
                                DropdownMenuItem(
                                    text = {
                                        Text(if (showSource) "View formatted message" else "View source")
                                    },
                                    onClick = {
                                        menuExpanded = false
                                        vm.toggleSource()
                                    },
                                )
                                if (!showSource) {
                                    for (alternative in parsed?.alternatives.orEmpty()) {
                                        if (alternative.mimeType.equals(currentMime, ignoreCase = true)) {
                                            continue
                                        }
                                        DropdownMenuItem(
                                            text = { Text("View as ${alternative.mimeType}") },
                                            onClick = {
                                                menuExpanded = false
                                                vm.chooseAlternative(alternative.mimeType)
                                            },
                                        )
                                    }
                                }
                                DropdownMenuItem(
                                    text = { Text("Move to maildir...") },
                                    enabled = vm.folders().any { it.id != folderId } &&
                                        !vm.actionInProgress,
                                    onClick = {
                                        menuExpanded = false
                                        vm.openMovePicker(false)
                                    },
                                )
                                if (!showSource) {
                                    DropdownMenuItem(
                                        text = {
                                            Text(
                                                if (vm.remoteImages) "Hide remote images"
                                                else "Load remote images",
                                            )
                                        },
                                        onClick = {
                                            menuExpanded = false
                                            vm.updateRemoteImages(!vm.remoteImages)
                                        },
                                    )
                                }
                            }
                        }
                    },
            )
        },
    ) { padding ->
        Box(modifier = Modifier.fillMaxSize().padding(padding)) {
            when {
                loading -> Text("Loading message...", modifier = Modifier.padding(20.dp))
                error != null -> Text(
                    "Could not open message:\n$error",
                    modifier = Modifier.padding(20.dp),
                )
                html != null -> MessageWebView(
                    html = html,
                    attachments = parsed?.attachments.orEmpty(),
                    remoteImages = vm.remoteImages,
                    context = context,
                    pagedScrolling = pagedScrolling,
                )
                else -> Text("Nothing to show.", modifier = Modifier.padding(20.dp))
            }
        }
    }

}

@Composable
private fun MessageWebView(
    html: String,
    attachments: List<Attachment>,
    remoteImages: Boolean,
    context: Context,
    pagedScrolling: Boolean,
) {
    AndroidView(
        modifier = Modifier.fillMaxSize(),
        factory = { ctx ->
            ScrollAwareWebView(ctx).apply {
                setBackgroundColor(AndroidColor.WHITE)
                settings.javaScriptEnabled = false
                settings.allowFileAccess = false
                settings.allowContentAccess = false
                settings.domStorageEnabled = false
                settings.loadsImagesAutomatically = true
                settings.useWideViewPort = false
                settings.loadWithOverviewMode = false
                settings.blockNetworkLoads = !remoteImages
                this.paged = pagedScrolling
                webViewClient = object : WebViewClient() {
                    override fun shouldOverrideUrlLoading(
                        view: WebView?,
                        request: WebResourceRequest?,
                    ): Boolean {
                        val url = request?.url?.toString() ?: return false
                        return handleMessageUrl(url, context, attachments)
                    }
                }
                tag = html.hashCode() to remoteImages
                loadDataWithBaseURL(null, html, "text/html", "utf-8", null)
            }
        },
        update = { webView ->
            webView.settings.blockNetworkLoads = !remoteImages
            (webView as? ScrollAwareWebView)?.paged = pagedScrolling
            val key = html.hashCode() to remoteImages
            if (webView.tag != key) {
                webView.tag = key
                webView.loadDataWithBaseURL(null, html, "text/html", "utf-8", null)
            }
        },
    )
}

private class ScrollAwareWebView(context: Context) : WebView(context) {
    var paged = true

    private var touchStartX = 0f
    private var touchStartY = 0f
    private var swiped = false
    private val touchSlop = ViewConfiguration.get(context).scaledTouchSlop

    override fun onTouchEvent(event: MotionEvent): Boolean {
        if (!paged) return super.onTouchEvent(event)
        when (event.actionMasked) {
            MotionEvent.ACTION_DOWN -> {
                touchStartX = event.x
                touchStartY = event.y
                swiped = false
                return super.onTouchEvent(event)
            }
            MotionEvent.ACTION_MOVE -> {
                val dx = event.x - touchStartX
                val dy = event.y - touchStartY
                if (abs(dy) > touchSlop && abs(dy) > abs(dx)) {
                    if (!swiped) {
                        swiped = true
                        val cancel = MotionEvent.obtain(event)
                        cancel.action = MotionEvent.ACTION_CANCEL
                        super.onTouchEvent(cancel)
                        cancel.recycle()
                    }
                    return true
                }
                return super.onTouchEvent(event)
            }
            MotionEvent.ACTION_UP -> {
                if (swiped) {
                    val dy = event.y - touchStartY
                    if (abs(dy) > 40f) {
                        scrollByPage(if (dy < 0) 1 else -1)
                    }
                    return true
                }
                return super.onTouchEvent(event)
            }
            MotionEvent.ACTION_CANCEL -> {
                swiped = false
                return super.onTouchEvent(event)
            }
        }
        return super.onTouchEvent(event)
    }

    private fun scrollByPage(direction: Int) {
        val page = (height - 48).coerceAtLeast(1)
        scrollBy(0, direction * page)
    }
}

@Composable
internal fun Modifier.pagedScroll(state: LazyListState): Modifier {
    val scope = rememberCoroutineScope()
    return this.pointerInput(state) {
        var total = 0f
        detectVerticalDragGestures(
            onDragStart = { total = 0f },
            onDragEnd = {
                if (abs(total) > 40f) {
                    val info = state.layoutInfo
                    val visible = info.visibleItemsInfo
                    if (visible.isNotEmpty() && info.totalItemsCount > 0) {
                        val first = visible.first()
                        val last = visible.last()
                        val count = (last.index - first.index + 1).coerceAtLeast(1)
                        val viewport = info.viewportSize.height
                        val lastPartial = last.offset + last.size > viewport
                        val firstPartial = first.offset < 0
                        val target = if (total < 0) {
                            when {
                                lastPartial -> last.index
                                last.index >= info.totalItemsCount - 1 -> null
                                else -> last.index + 1
                            }
                        } else {
                            when {
                                firstPartial -> first.index
                                first.index <= 0 -> null
                                else -> (first.index - count).coerceAtLeast(0)
                            }
                        }
                        if (target != null) {
                            scope.launch { state.scrollToItem(target) }
                        }
                    }
                }
                total = 0f
            },
            onDragCancel = { total = 0f },
            onVerticalDrag = { change, dragAmount ->
                change.consume()
                total += dragAmount
            },
        )
    }
}

@Composable
internal fun Modifier.pagedScroll(state: ScrollState): Modifier {
    val scope = rememberCoroutineScope()
    return this.pointerInput(state) {
        var total = 0f
        detectVerticalDragGestures(
            onDragStart = { total = 0f },
            onDragEnd = {
                val page = (state.viewportSize - 48).coerceAtLeast(1)
                if (abs(total) > 40f && state.maxValue > 0) {
                    val direction = if (total < 0) 1 else -1
                    scope.launch { state.scrollBy(direction * page.toFloat()) }
                }
                total = 0f
            },
            onDragCancel = { total = 0f },
            onVerticalDrag = { change, dragAmount ->
                change.consume()
                total += dragAmount
            },
        )
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun GoToItemScreen(vm: MainViewModel, target: Screen.GoToItem) {
    var text by remember { mutableStateOf("") }
    var error by remember { mutableStateOf<String?>(null) }
    val total = vm.messages.size

    Scaffold(
        topBar = {
            AppTopBar(
                title = "Go to item",
                onBack = { vm.back() },
            )
        },
    ) { padding ->
        Column(modifier = Modifier.fillMaxSize().padding(padding).padding(12.dp)) {
            Text(
                "Message number (1 to $total)",
                style = MaterialTheme.typography.bodyLarge,
            )
            Spacer(Modifier.height(10.dp))
            OutlinedTextField(
                value = text,
                onValueChange = { value -> text = value.filter { it.isDigit() } },
                label = { Text("Item") },
                singleLine = true,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                modifier = Modifier.fillMaxWidth(),
            )
            if (error != null) {
                Spacer(Modifier.height(8.dp))
                Text(error!!, fontWeight = FontWeight.Bold, fontSize = 14.sp)
            }
            Spacer(Modifier.height(16.dp))
            Button(
                onClick = {
                    val number = text.toIntOrNull()
                    if (number == null) {
                        error = "Enter a number"
                    } else {
                        val result = vm.goToItem(target.folderId, number)
                        if (result != null) error = result
                    }
                },
                modifier = Modifier.fillMaxWidth(),
            ) {
                Text("Go")
            }
        }
    }
}

private fun defaultMime(message: ParsedMessage, preferHtml: Boolean): String? = when {
    preferHtml && !message.html.isNullOrBlank() -> "text/html"
    !message.text.isNullOrBlank() -> "text/plain"
    !message.html.isNullOrBlank() -> "text/html"
    else -> message.alternatives.firstOrNull()?.mimeType
}

private fun handleMessageUrl(
    url: String,
    context: Context,
    attachments: List<Attachment>,
): Boolean {
    if (url.startsWith("maildirt://attachment/")) {
        val index = url.removePrefix("maildirt://attachment/").substringBefore('/').toIntOrNull()
        if (index != null) openAttachment(context, attachments.getOrNull(index), share = false)
        return true
    }
    if (url.startsWith("maildirt://share/")) {
        val index = url.removePrefix("maildirt://share/").substringBefore('/').toIntOrNull()
        if (index != null) openAttachment(context, attachments.getOrNull(index), share = true)
        return true
    }
    if (url.startsWith("http://", true) ||
        url.startsWith("https://", true) ||
        url.startsWith("mailto:", true)
    ) {
        runCatching { context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(url))) }
        return true
    }
    return false
}

private fun openAttachment(context: Context, attachment: Attachment?, share: Boolean) {
    if (attachment == null) return
    val uri = runCatching {
        FileProvider.getUriForFile(
            context,
            "${context.packageName}.fileprovider",
            attachment.file,
        )
    }.getOrNull() ?: return

    val intent = if (share) {
        Intent(Intent.ACTION_SEND).apply {
            type = attachment.mimeType
            putExtra(Intent.EXTRA_STREAM, uri)
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
        }
    } else {
        Intent(Intent.ACTION_VIEW).apply {
            setDataAndType(uri, attachment.mimeType)
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
        }
    }

    runCatching {
        context.startActivity(
            if (share) Intent.createChooser(intent, "Share attachment") else intent,
        )
    }.onFailure {
        Toast.makeText(context, "No app can open this attachment", Toast.LENGTH_SHORT).show()
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun PickMaildirScreen(vm: MainViewModel, picker: Screen.PickMaildir) {
    val folders = vm.folders().filter { it.id != picker.folderId }
    Scaffold(
        topBar = {
            AppTopBar(
                title = "Move to maildir",
                onBack = { vm.back() },
            )
        },
    ) { padding ->
        if (folders.isEmpty()) {
            Text(
                "No other maildirs configured.",
                modifier = Modifier.padding(padding).padding(20.dp),
            )
        } else {
            LazyColumn(modifier = Modifier.fillMaxSize().padding(padding)) {
                items(folders, key = { it.id }) { folder ->
                    ListRow(
                        title = folder.displayName,
                        subtitle = folder.path,
                        onClick = {
                            if (picker.forSelection) {
                                vm.moveSelected(picker.folderId, folder.id)
                            } else {
                                vm.moveCurrent(picker.folderId, folder.id)
                            }
                        },
                    )
                }
            }
        }
    }
}

internal fun normalizeRole(role: FolderRole?): FolderRole = role ?: FolderRole.OTHER

internal fun roleLabel(role: FolderRole): String = when (role) {
    FolderRole.INBOX -> "Inbox"
    FolderRole.ARCHIVE -> "Archive"
    FolderRole.POSTPONED -> "Postponed"
    FolderRole.OTHER -> "Other"
}

internal fun formatInterval(seconds: Int): String = when {
    seconds >= 3600 && seconds % 3600 == 0 -> "${seconds / 3600} h"
    seconds >= 60 -> "${seconds / 60} min"
    else -> "$seconds s"
}

@Composable
internal fun ListRow(title: String, subtitle: String?, onClick: () -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick)
            .padding(horizontal = 12.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            Text(title, style = MaterialTheme.typography.titleMedium)
            if (!subtitle.isNullOrBlank()) {
                Spacer(Modifier.height(2.dp))
                Text(
                    subtitle,
                    style = MaterialTheme.typography.bodyMedium,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
            }
        }
        Text("\u203A", fontSize = 30.sp)
    }
}

private fun formatListDate(timestamp: Long): String {
    val minutes = (System.currentTimeMillis() - timestamp) / 60_000L
    return when {
        minutes < 1 -> "now"
        minutes < 60 -> "${minutes}m"
        minutes < 60 * 24 -> "${minutes / 60}h"
        minutes < 60 * 24 * 7 -> "${minutes / (60 * 24)}d"
        else -> SimpleDateFormat("yyyy/MM/dd HH:mm", Locale.getDefault())
            .apply { timeZone = TimeZone.getDefault() }
            .format(Date(timestamp))
    }
}

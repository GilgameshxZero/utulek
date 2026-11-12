package com.gilgamesh.maildirt

import android.Manifest
import android.content.Context
import android.content.Intent
import android.graphics.Color as AndroidColor
import android.net.Uri
import android.webkit.WebResourceRequest
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.Toast
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
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

@Composable
fun MaildirtApp(vm: MainViewModel = viewModel()) {
    val context = LocalContext.current
    val granted = rememberStorageGranted()
    val permissionLauncher = rememberLauncherForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { result -> granted.value = result }

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
    BackHandler(enabled = screen != Screen.Folders) { vm.back() }

    when (screen) {
        Screen.Folders -> FolderScreen(vm)
        Screen.Settings -> SettingsScreen(vm)
        is Screen.Messages -> MessageListScreen(vm, screen.folderId)
        is Screen.Message -> MessageScreen(vm, screen.folderId)
    }
}

@Composable
private fun rememberStorageGranted(): MutableState<Boolean> {
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

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun FolderScreen(vm: MainViewModel) {
    val folders = vm.folders()
    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("maildirt") },
                actions = {
                    IconButton(onClick = { vm.openSettings() }) {
                        Icon(
                            painter = painterResource(R.drawable.ic_settings),
                            contentDescription = "Settings",
                        )
                    }
                },
            )
        },
    ) { padding ->
        if (folders.isEmpty()) {
            Column(
                modifier = Modifier.fillMaxSize().padding(padding).padding(24.dp),
                verticalArrangement = Arrangement.Center,
            ) {
                Text("No maildirs configured.", style = MaterialTheme.typography.titleMedium)
                Spacer(Modifier.height(8.dp))
                Text("Add a maildir path in Settings.", style = MaterialTheme.typography.bodyLarge)
                Spacer(Modifier.height(16.dp))
                Button(onClick = { vm.openSettings() }) { Text("Open settings") }
            }
        } else {
            LazyColumn(modifier = Modifier.fillMaxSize().padding(padding)) {
                items(folders, key = { it.id }) { folder ->
                    ListRow(
                        title = folder.displayName,
                        subtitle = folder.path,
                        onClick = { vm.openFolder(folder.id) },
                    )
                    HorizontalDivider()
                }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun MessageListScreen(vm: MainViewModel, folderId: String) {
    val folder = vm.folder(folderId)
    val messages = vm.messages
    val loading = vm.listLoading
    val error = vm.listError

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        folder?.displayName ?: "Folder",
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                },
                navigationIcon = {
                    IconButton(onClick = { vm.back() }) {
                        Icon(
                            painter = painterResource(R.drawable.ic_back),
                            contentDescription = "Back",
                        )
                    }
                },
                actions = {
                    IconButton(onClick = { vm.refreshFolder() }) {
                        Icon(
                            painter = painterResource(R.drawable.ic_refresh),
                            contentDescription = "Refresh",
                        )
                    }
                },
            )
        },
    ) { padding ->
        Column(modifier = Modifier.fillMaxSize().padding(padding)) {
            when {
                loading -> Text("Loading...", modifier = Modifier.padding(24.dp))
                error != null -> Text(
                    "Could not read this folder:\n$error",
                    modifier = Modifier.padding(24.dp),
                )
                messages.isEmpty() -> Text("No messages.", modifier = Modifier.padding(24.dp))
                else -> LazyColumn(
                    state = vm.listState,
                    modifier = Modifier.fillMaxSize(),
                ) {
                    items(messages, key = { it.relativePath }) { entry ->
                        MessageRow(entry) { vm.openMessage(entry.relativePath) }
                        HorizontalDivider()
                    }
                }
            }
        }
    }
}

@Composable
private fun MessageRow(entry: MaildirEntry, onClick: () -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick)
            .padding(horizontal = 14.dp, vertical = 10.dp),
        verticalAlignment = Alignment.Top,
    ) {
        if (entry.unread) {
            Text("\u25CF", fontSize = 12.sp, modifier = Modifier.padding(top = 5.dp, end = 8.dp))
        } else {
            Spacer(Modifier.width(20.dp))
        }
        Column(Modifier.weight(1f)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    entry.from,
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = if (entry.unread) FontWeight.Bold else FontWeight.Normal,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f),
                )
                Spacer(Modifier.width(8.dp))
                Text(formatListDate(entry.date), fontSize = 13.sp)
            }
            Spacer(Modifier.height(2.dp))
            Text(
                entry.subject,
                style = MaterialTheme.typography.bodyLarge,
                fontWeight = if (entry.unread) FontWeight.Bold else FontWeight.Normal,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
            if (entry.hasAttachments || entry.replied) {
                Row(modifier = Modifier.padding(top = 2.dp)) {
                    if (entry.hasAttachments) {
                        Text("attachment", fontSize = 12.sp)
                    }
                    if (entry.hasAttachments && entry.replied) {
                        Text("  \u00B7  ", fontSize = 12.sp)
                    }
                    if (entry.replied) {
                        Text("replied", fontSize = 12.sp, fontWeight = FontWeight.Bold)
                    }
                }
            }
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
    val currentMime = vm.viewAlternative ?: parsed?.let { defaultMime(it) }
    val html = remember(parsed, currentMime) {
        parsed?.let { renderMessageHtml(it, currentMime) }
    }
    var menuExpanded by remember { mutableStateOf(false) }
    var showMoveDialog by remember { mutableStateOf(false) }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        parsed?.subject ?: "Message",
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                },
                navigationIcon = {
                    IconButton(onClick = { vm.back() }) {
                        Icon(
                            painter = painterResource(R.drawable.ic_back),
                            contentDescription = "Back",
                        )
                    }
                },
                actions = {
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
                            DropdownMenuItem(
                                text = { Text("Mark as unread") },
                                enabled = entry?.unread == false && !vm.actionInProgress,
                                onClick = {
                                    menuExpanded = false
                                    vm.markCurrentUnread()
                                },
                            )
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
                            DropdownMenuItem(
                                text = { Text("Move to maildir...") },
                                enabled = vm.folders().any { it.id != folderId } &&
                                    !vm.actionInProgress,
                                onClick = {
                                    menuExpanded = false
                                    showMoveDialog = true
                                },
                            )
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
                },
            )
        },
    ) { padding ->
        Column(modifier = Modifier.fillMaxSize().padding(padding)) {
            val attachments = parsed?.attachments.orEmpty()
            if (attachments.isNotEmpty()) {
                AttachmentBar(attachments)
                HorizontalDivider()
            }
            Box(modifier = Modifier.weight(1f).fillMaxWidth()) {
                when {
                    loading -> Text("Loading message...", modifier = Modifier.padding(24.dp))
                    error != null -> Text(
                        "Could not open message:\n$error",
                        modifier = Modifier.padding(24.dp),
                    )
                    html != null -> MessageWebView(
                        html = html,
                        attachments = attachments,
                        remoteImages = vm.remoteImages,
                        context = context,
                    )
                    else -> Text("Nothing to show.", modifier = Modifier.padding(24.dp))
                }
            }
        }
    }

    if (showMoveDialog) {
        MoveMaildirDialog(
            folders = vm.folders().filter { it.id != folderId },
            onDismiss = { showMoveDialog = false },
            onSelect = { targetId ->
                showMoveDialog = false
                vm.moveCurrent(targetId)
            },
        )
    }
}

@Composable
private fun AttachmentBar(attachments: List<Attachment>) {
    val context = LocalContext.current
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .heightIn(max = 180.dp)
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 12.dp, vertical = 6.dp),
    ) {
        Text("Attachments", fontWeight = FontWeight.Bold, fontSize = 14.sp)
        for (attachment in attachments) {
            Row(
                modifier = Modifier.fillMaxWidth().padding(vertical = 2.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    attachment.name,
                    fontSize = 15.sp,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier
                        .weight(1f)
                        .clickable { openAttachment(context, attachment, share = false) },
                )
                Text(
                    formatSize(attachment.size),
                    fontSize = 12.sp,
                    modifier = Modifier.padding(horizontal = 8.dp),
                )
                IconButton(onClick = { openAttachment(context, attachment, share = true) }) {
                    Icon(
                        painter = painterResource(R.drawable.ic_share),
                        contentDescription = "Share ${attachment.name}",
                    )
                }
            }
        }
    }
}

@Composable
private fun MoveMaildirDialog(
    folders: List<MaildirFolder>,
    onDismiss: () -> Unit,
    onSelect: (String) -> Unit,
) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("Move to maildir") },
        text = {
            if (folders.isEmpty()) {
                Text("No other maildirs configured.")
            } else {
                Column(
                    modifier = Modifier.heightIn(max = 420.dp).verticalScroll(rememberScrollState()),
                ) {
                    for (folder in folders) {
                        ListRow(
                            title = folder.displayName,
                            subtitle = folder.path,
                            onClick = { onSelect(folder.id) },
                        )
                        HorizontalDivider()
                    }
                }
            }
        },
        confirmButton = {
            TextButton(onClick = onDismiss) { Text("Cancel") }
        },
    )
}

@Composable
private fun MessageWebView(
    html: String,
    attachments: List<Attachment>,
    remoteImages: Boolean,
    context: Context,
) {
    AndroidView(
        modifier = Modifier.fillMaxSize(),
        factory = { ctx ->
            WebView(ctx).apply {
                setBackgroundColor(AndroidColor.WHITE)
                settings.javaScriptEnabled = false
                settings.allowFileAccess = false
                settings.allowContentAccess = false
                settings.domStorageEnabled = false
                settings.loadsImagesAutomatically = true
                settings.useWideViewPort = false
                settings.loadWithOverviewMode = false
                settings.blockNetworkLoads = !remoteImages
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
            val key = html.hashCode() to remoteImages
            if (webView.tag != key) {
                webView.tag = key
                webView.loadDataWithBaseURL(null, html, "text/html", "utf-8", null)
            }
        },
    )
}

private fun defaultMime(message: ParsedMessage): String? = when {
    !message.html.isNullOrBlank() -> "text/html"
    !message.text.isNullOrBlank() -> "text/plain"
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
private fun SettingsScreen(vm: MainViewModel) {
    val context = LocalContext.current
    val granted = rememberStorageGranted()
    val folders = vm.folders()
    var showAddDialog by remember { mutableStateOf(false) }
    var editingFolder by remember { mutableStateOf<MaildirFolder?>(null) }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Settings") },
                navigationIcon = {
                    IconButton(onClick = { vm.back() }) {
                        Icon(
                            painter = painterResource(R.drawable.ic_back),
                            contentDescription = "Back",
                        )
                    }
                },
            )
        },
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .padding(16.dp),
        ) {
            Text("Storage access", style = MaterialTheme.typography.titleMedium)
            Spacer(Modifier.height(4.dp))
            Text(
                if (granted.value) "Granted."
                else "maildirt needs access to shared storage to read maildirs.",
                style = MaterialTheme.typography.bodyLarge,
            )
            if (!granted.value) {
                Spacer(Modifier.height(8.dp))
                Button(onClick = { StorageAccess.openSystemSettings(context) }) {
                    Text("Open system settings")
                }
            }

            Spacer(Modifier.height(24.dp))
            Text("Maildirs", style = MaterialTheme.typography.titleMedium)
            Spacer(Modifier.height(8.dp))

            for (folder in folders) {
                Column(Modifier.fillMaxWidth().padding(vertical = 8.dp)) {
                    Text(
                        folder.displayName,
                        style = MaterialTheme.typography.bodyLarge,
                        fontWeight = FontWeight.Bold,
                    )
                    Text(folder.path, style = MaterialTheme.typography.bodyMedium)
                    Text(roleLabel(folder.role), fontSize = 13.sp)
                    Row {
                        TextButton(onClick = { editingFolder = folder }) { Text("Edit") }
                        TextButton(onClick = { vm.removeFolder(folder.id) }) { Text("Remove") }
                    }
                }
                HorizontalDivider()
            }

            Spacer(Modifier.height(8.dp))
            Button(onClick = { showAddDialog = true }) { Text("Add maildir") }

            Spacer(Modifier.height(32.dp))
            Text("maildirt 0.2.0-m2", fontSize = 13.sp)
        }
    }

    if (showAddDialog) {
        FolderDialog(
            initial = null,
            onDismiss = { showAddDialog = false },
            onSave = { path, role -> vm.addFolder(path, role) },
        )
    }

    editingFolder?.let { folder ->
        FolderDialog(
            initial = folder,
            onDismiss = { editingFolder = null },
            onSave = { path, role -> vm.updateFolder(folder.id, path, role) },
        )
    }
}

@Composable
private fun FolderDialog(
    initial: MaildirFolder?,
    onDismiss: () -> Unit,
    onSave: (String, FolderRole) -> String?,
) {
    var path by remember(initial?.id) { mutableStateOf(initial?.path.orEmpty()) }
    var role by remember(initial?.id) { mutableStateOf(normalizeRole(initial?.role)) }
    var error by remember(initial?.id) { mutableStateOf<String?>(null) }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(if (initial == null) "Add maildir" else "Edit maildir") },
        text = {
            Column {
                OutlinedTextField(
                    value = path,
                    onValueChange = { path = it },
                    label = { Text("Path") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                )
                Spacer(Modifier.height(10.dp))
                Text("Role", style = MaterialTheme.typography.bodyMedium)
                Spacer(Modifier.height(4.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    for (option in listOf(FolderRole.INBOX, FolderRole.ARCHIVE, FolderRole.OTHER)) {
                        FilterChip(
                            selected = role == option,
                            onClick = { role = option },
                            label = { Text(roleLabel(option)) },
                        )
                    }
                }
                Spacer(Modifier.height(6.dp))
                Text("Folder must contain cur/, new/ and tmp/.", fontSize = 13.sp)
                if (error != null) {
                    Spacer(Modifier.height(6.dp))
                    Text(error!!, fontWeight = FontWeight.Bold, fontSize = 14.sp)
                }
            }
        },
        confirmButton = {
            TextButton(
                onClick = {
                    val result = onSave(path, role)
                    if (result == null) onDismiss() else error = result
                },
            ) { Text("Save") }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) { Text("Cancel") }
        },
    )
}

private fun normalizeRole(role: FolderRole?): FolderRole =
    if (role == FolderRole.INBOX || role == FolderRole.ARCHIVE) role else FolderRole.OTHER

private fun roleLabel(role: FolderRole): String = when (role) {
    FolderRole.INBOX -> "Inbox"
    FolderRole.ARCHIVE -> "Archive"
    FolderRole.SENT -> "Sent"
    FolderRole.DRAFTS -> "Drafts"
    FolderRole.OTHER -> "Other"
}

@Composable
private fun ListRow(title: String, subtitle: String?, onClick: () -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick)
            .padding(horizontal = 16.dp, vertical = 14.dp),
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

private fun formatListDate(timestamp: Long): String =
    SimpleDateFormat("yy/MM/dd HH:mm", Locale.getDefault()).format(Date(timestamp))

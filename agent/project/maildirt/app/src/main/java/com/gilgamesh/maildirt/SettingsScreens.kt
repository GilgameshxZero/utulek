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
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.text.TextStyle
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
internal fun SettingsScreen(vm: MainViewModel) {
    val context = LocalContext.current
    val granted = rememberStorageGranted()
    val folders = vm.folders()
    val scrollState = rememberScrollState()
    val paged = vm.config.pagedScrolling
    var pollText by remember { mutableStateOf(vm.config.notifyPollSeconds.toString()) }

    DisposableEffect(Unit) {
        onDispose {
            val parsed = pollText.toIntOrNull()
            if (parsed != null && parsed != vm.config.notifyPollSeconds) {
                vm.setNotifyPollSeconds(parsed)
            }
        }
    }

    Scaffold(
        topBar = {
            AppTopBar(
                title = "Settings",
                onBack = { vm.back() },
            )
        },
    ) { padding ->
        val base = Modifier
            .fillMaxSize()
            .padding(padding)
            .verticalScroll(scrollState, enabled = !paged)
        val columnModifier = if (paged) base.pagedScroll(scrollState) else base
        Column(modifier = columnModifier) {
            SettingsSection("Storage")
            SettingsRow(
                title = "Storage access",
                value = if (granted.value) "Granted" else "Not granted",
                onClick = if (granted.value) null else {
                    { StorageAccess.openSystemSettings(context) }
                },
            )
            if (!granted.value) {
                Text(
                    "maildirt needs access to shared storage to read maildirs.",
                    fontSize = 11.sp,
                    color = InkDim,
                    modifier = Modifier.padding(horizontal = 12.dp, vertical = 4.dp),
                )
            }

            SettingsSection("Appearance")
            SettingsRow(
                title = "Scrolling",
                value = if (paged) "Paged" else "Smooth",
                onClick = { vm.setPagedScrolling(!paged) },
            )
            SettingsRow(
                title = "Reply action",
                value = if (vm.config.defaultReplyAll) "Reply all" else "Reply",
                onClick = { vm.setDefaultReplyAll(!vm.config.defaultReplyAll) },
            )
            SettingsRow(
                title = "Message format",
                value = if (vm.config.defaultMime == MimePreference.PLAIN) "Plain" else "HTML",
                onClick = {
                    vm.setDefaultMime(
                        if (vm.config.defaultMime == MimePreference.PLAIN) {
                            MimePreference.HTML
                        } else {
                            MimePreference.PLAIN
                        },
                    )
                },
            )

            SettingsSection("Notifications")
            SettingsRow(
                title = "Sound",
                value = if (vm.config.notifySound) "Sound" else "Silent",
                onClick = { vm.setNotifySound(!vm.config.notifySound) },
            )
            SettingsRow(
                title = "Lock screen content",
                value = if (vm.config.notifyShowContent) "Show" else "Hide",
                onClick = { vm.setNotifyShowContent(!vm.config.notifyShowContent) },
            )
            SettingsRow(
                title = "Check interval",
                value = "Every ${formatInterval(vm.config.notifyPollSeconds)}",
            )
            SettingsField(
                label = "Seconds (fallback when file watching is unavailable)",
                value = pollText,
                onValueChange = { value -> pollText = value.filter { it.isDigit() } },
                numeric = true,
            )

            SettingsSection("Maildirs", addLabel = "Add maildir", onAdd = { vm.openFolderEditor(null) })
            if (folders.isEmpty()) {
                Text(
                    "No maildirs configured.",
                    fontSize = 13.sp,
                    color = InkDim,
                    modifier = Modifier.padding(horizontal = 12.dp, vertical = 6.dp),
                )
            }
            for ((folderIndex, folder) in folders.withIndex()) {
                var menuOpen by remember(folder.id) { mutableStateOf(false) }
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(start = 12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Column(Modifier.weight(1f)) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(
                                folder.displayName,
                                fontSize = 16.sp,
                                fontWeight = FontWeight.SemiBold,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                            )
                            Spacer(Modifier.width(8.dp))
                            Text(roleLabel(folder.role), fontSize = 11.sp, color = InkDim)
                        }
                        Text(
                            folder.path,
                            fontSize = 12.sp,
                            color = InkDim,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                    Box {
                        BarIcon(R.drawable.ic_more_vert, "Maildir actions") { menuOpen = true }
                        DropdownMenu(
                            expanded = menuOpen,
                            onDismissRequest = { menuOpen = false },
                        ) {
                            DropdownMenuItem(
                                text = { Text("Move up", fontSize = 16.sp) },
                                enabled = folderIndex > 0,
                                onClick = {
                                    menuOpen = false
                                    vm.moveFolder(folder.id, -1)
                                },
                            )
                            DropdownMenuItem(
                                text = { Text("Move down", fontSize = 16.sp) },
                                enabled = folderIndex < folders.lastIndex,
                                onClick = {
                                    menuOpen = false
                                    vm.moveFolder(folder.id, 1)
                                },
                            )
                            DropdownMenuItem(
                                text = { Text("Edit", fontSize = 16.sp) },
                                onClick = {
                                    menuOpen = false
                                    vm.openFolderEditor(folder.id)
                                },
                            )
                            DropdownMenuItem(
                                text = { Text("Remove", fontSize = 16.sp) },
                                onClick = {
                                    menuOpen = false
                                    vm.removeFolder(folder.id)
                                },
                            )
                        }
                    }
                }
            }

            SettingsSection("Sender identities", addLabel = "Add identity", onAdd = { vm.openIdentityEditor(null) })
            Text(
                "Topmost identity is used by default.",
                fontSize = 11.sp,
                color = InkDim,
                modifier = Modifier.padding(horizontal = 12.dp, vertical = 4.dp),
            )
            if (vm.identities().isEmpty()) {
                Text(
                    "No sender identities configured.",
                    fontSize = 13.sp,
                    color = InkDim,
                    modifier = Modifier.padding(horizontal = 12.dp, vertical = 6.dp),
                )
            }
            val identityList = vm.identities()
            for ((identityIndex, identity) in identityList.withIndex()) {
                var menuOpen by remember(identity.id) { mutableStateOf(false) }
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(start = 12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Column(Modifier.weight(1f)) {
                        Text(
                            identity.displayLabel,
                            fontSize = 16.sp,
                            fontWeight = FontWeight.SemiBold,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                        Text(
                            if (identity.authMode == AuthMode.XOAUTH2) {
                                "Gmail XOAUTH2 \u00b7 ${identity.smtpHost}"
                            } else {
                                "SMTP \u00b7 ${identity.smtpHost}"
                            },
                            fontSize = 12.sp,
                            color = InkDim,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                    Box {
                        BarIcon(R.drawable.ic_more_vert, "Identity actions") { menuOpen = true }
                        DropdownMenu(
                            expanded = menuOpen,
                            onDismissRequest = { menuOpen = false },
                        ) {
                            DropdownMenuItem(
                                text = { Text("Move up", fontSize = 16.sp) },
                                enabled = identityIndex > 0,
                                onClick = {
                                    menuOpen = false
                                    vm.moveIdentity(identity.id, -1)
                                },
                            )
                            DropdownMenuItem(
                                text = { Text("Move down", fontSize = 16.sp) },
                                enabled = identityIndex < identityList.lastIndex,
                                onClick = {
                                    menuOpen = false
                                    vm.moveIdentity(identity.id, 1)
                                },
                            )
                            DropdownMenuItem(
                                text = { Text("Edit", fontSize = 16.sp) },
                                onClick = {
                                    menuOpen = false
                                    vm.openIdentityEditor(identity.id)
                                },
                            )
                            DropdownMenuItem(
                                text = { Text("Remove", fontSize = 16.sp) },
                                onClick = {
                                    menuOpen = false
                                    vm.removeIdentity(identity.id)
                                },
                            )
                        }
                    }
                }
            }

            Text(
                "maildirt 0.5.3",
                fontSize = 11.sp,
                color = InkDim,
                modifier = Modifier.padding(horizontal = 12.dp, vertical = 16.dp),
            )
        }
    }
}

@Composable
private fun SettingsSection(
    title: String,
    addLabel: String = "Add",
    onAdd: (() -> Unit)? = null,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(start = 12.dp, top = 12.dp, bottom = 2.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            title.uppercase(),
            fontSize = 10.sp,
            color = InkDim,
            modifier = Modifier.weight(1f),
        )
        if (onAdd != null) {
            BarIcon(R.drawable.ic_add, addLabel) { onAdd() }
        }
    }
}

@Composable
private fun SettingsRow(
    title: String,
    value: String? = null,
    onClick: (() -> Unit)? = null,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(enabled = onClick != null) { onClick?.invoke() }
            .padding(horizontal = 12.dp, vertical = 11.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            title,
            fontSize = 16.sp,
            fontWeight = FontWeight.Normal,
            modifier = Modifier.weight(1f),
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        if (value != null) {
            Text(
                value,
                fontSize = 13.sp,
                color = InkDim,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.padding(start = 12.dp),
            )
        }
    }
}

@Composable
private fun SettingsField(
    label: String,
    value: String,
    onValueChange: (String) -> Unit,
    numeric: Boolean = false,
) {
    Column(
        Modifier
            .fillMaxWidth()
            .padding(horizontal = 12.dp, vertical = 4.dp),
    ) {
        Text(label, fontSize = 11.sp, color = InkDim)
        Box(
            Modifier
                .fillMaxWidth()
                .padding(vertical = 6.dp)
                .drawBehind {
                    drawLine(
                        InkDim,
                        Offset(0f, size.height),
                        Offset(size.width, size.height),
                        strokeWidth = 1f,
                    )
                },
        ) {
            BasicTextField(
                value = value,
                onValueChange = onValueChange,
                singleLine = true,
                textStyle = TextStyle(color = Color.Black, fontSize = 16.sp, lineHeight = 22.sp),
                cursorBrush = SolidColor(Color.Black),
                keyboardOptions = if (numeric) {
                    KeyboardOptions(keyboardType = KeyboardType.Number)
                } else {
                    KeyboardOptions.Default
                },
                modifier = Modifier.fillMaxWidth(),
            )
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun FolderEditorScreen(vm: MainViewModel, editor: Screen.FolderEditor) {
    val initial = editor.folderId?.let { vm.folder(it) }
    var path by remember(editor.folderId) { mutableStateOf(initial?.path.orEmpty()) }
    var role by remember(editor.folderId) { mutableStateOf(normalizeRole(initial?.role)) }
    var error by remember(editor.folderId) { mutableStateOf<String?>(null) }
    val paged = vm.config.pagedScrolling
    val scrollState = rememberScrollState()

    fun save(): Boolean {
        if (editor.folderId == null && path.isBlank()) return true
        val result = if (editor.folderId == null) {
            vm.addFolder(path, role)
        } else {
            vm.updateFolder(editor.folderId, path, role)
        }
        return if (result == null) {
            true
        } else {
            error = result
            false
        }
    }

    BackHandler { if (save()) vm.back() }

    Scaffold(
        topBar = {
            AppTopBar(
                title = if (editor.folderId == null) "Add maildir" else "Edit maildir",
                onBack = { if (save()) vm.back() },
            )
        },
    ) { padding ->
        val base = Modifier
            .fillMaxSize()
            .padding(padding)
            .verticalScroll(scrollState, enabled = !paged)
            .padding(12.dp)
        val modifier = if (paged) base.pagedScroll(scrollState) else base
        Column(modifier = modifier) {
            Text("Path", fontSize = 13.sp)
            val pathScroll = rememberScrollState()
            LaunchedEffect(path) { pathScroll.animateScrollTo(pathScroll.maxValue) }
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .border(1.dp, Color.Black)
                    .horizontalScroll(pathScroll)
                    .padding(horizontal = 12.dp, vertical = 14.dp),
            ) {
                BasicTextField(
                    value = path,
                    onValueChange = { path = it },
                    singleLine = true,
                    textStyle = MaterialTheme.typography.bodyLarge.copy(color = Color.Black),
                    modifier = Modifier.width(IntrinsicSize.Max),
                )
            }
            Spacer(Modifier.height(16.dp))
            Text("Role", style = MaterialTheme.typography.titleMedium)
            Spacer(Modifier.height(4.dp))
            Column(Modifier.fillMaxWidth()) {
                for (option in FolderRole.values()) {
                    val selected = role == option
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .clickable { role = option }
                            .padding(vertical = 6.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Box(Modifier.width(24.dp), contentAlignment = Alignment.CenterStart) {
                            if (selected) {
                                Icon(
                                    painter = painterResource(R.drawable.ic_check),
                                    contentDescription = "Selected",
                                    modifier = Modifier.size(18.dp),
                                )
                            }
                        }
                        Text(
                            roleLabel(option),
                            style = MaterialTheme.typography.bodyLarge,
                            fontWeight = if (selected) FontWeight.Bold else FontWeight.Normal,
                        )
                    }
                }
            }
            if (error != null) {
                Spacer(Modifier.height(8.dp))
                Text(error!!, fontWeight = FontWeight.Bold, fontSize = 14.sp)
            }
            Spacer(Modifier.height(20.dp))
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun IdentityEditorScreen(vm: MainViewModel, editor: Screen.IdentityEditor) {
    val initial = editor.identityId?.let { vm.identity(it) }
    val identityId = remember(editor.identityId) {
        initial?.id ?: UUID.randomUUID().toString()
    }
    val hasSecret = remember(editor.identityId) { vm.hasStoredSecret(identityId) }
    var email by remember(editor.identityId) { mutableStateOf(initial?.email.orEmpty()) }
    var displayName by remember(editor.identityId) {
        mutableStateOf(initial?.displayName.orEmpty())
    }
    var signature by remember(editor.identityId) {
        mutableStateOf(initial?.signature.orEmpty())
    }
    var authMode by remember(editor.identityId) {
        mutableStateOf(initial?.authMode ?: AuthMode.STANDARD)
    }
    var host by remember(editor.identityId) {
        mutableStateOf(initial?.smtpHost.orEmpty())
    }
    var port by remember(editor.identityId) {
        mutableStateOf(initial?.smtpPort?.toString() ?: "587")
    }
    var security by remember(editor.identityId) {
        mutableStateOf(initial?.security ?: SmtpSecurity.STARTTLS)
    }
    var username by remember(editor.identityId) {
        mutableStateOf(initial?.username.orEmpty())
    }
    var tokenPath by remember(editor.identityId) {
        mutableStateOf(initial?.tokenFilePath.orEmpty())
    }
    var secret by remember(editor.identityId) { mutableStateOf("") }
    var secretVisible by remember(editor.identityId) { mutableStateOf(false) }
    var error by remember(editor.identityId) { mutableStateOf<String?>(null) }
    val paged = vm.config.pagedScrolling
    val scrollState = rememberScrollState()

    LaunchedEffect(editor.identityId) { vm.clearIdentityTest() }

    fun buildIdentity(): SenderIdentity = SenderIdentity(
        id = identityId,
        email = email,
        displayName = displayName,
        signature = signature,
        authMode = authMode,
        smtpHost = host,
        smtpPort = port.trim().toIntOrNull() ?: 0,
        security = security,
        username = username,
        tokenFilePath = tokenPath,
    )

    fun save(): Boolean {
        if (editor.identityId == null && email.isBlank()) return true
        val result = vm.saveIdentity(buildIdentity(), secret)
        return if (result == null) {
            true
        } else {
            error = result
            false
        }
    }

    BackHandler { if (save()) vm.back() }

    Scaffold(
        topBar = {
            AppTopBar(
                title = if (editor.identityId == null) "Add identity" else "Edit identity",
                onBack = { if (save()) vm.back() },
            )
        },
    ) { padding ->
        val base = Modifier
            .fillMaxSize()
            .padding(padding)
            .verticalScroll(scrollState, enabled = !paged)
            .padding(12.dp)
        val modifier = if (paged) base.pagedScroll(scrollState) else base
        Column(modifier = modifier) {
            OutlinedTextField(
                value = email,
                onValueChange = { email = it },
                label = { Text("Email address") },
                singleLine = true,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Email),
                modifier = Modifier.fillMaxWidth(),
            )
            Spacer(Modifier.height(12.dp))
            OutlinedTextField(
                value = displayName,
                onValueChange = { displayName = it },
                label = { Text("Display name (optional)") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )

            Spacer(Modifier.height(20.dp))
            Text("Authentication", style = MaterialTheme.typography.titleMedium)
            Spacer(Modifier.height(4.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceEvenly,
            ) {
                for (option in listOf(AuthMode.STANDARD, AuthMode.XOAUTH2)) {
                    val selected = authMode == option
                    Column(
                        modifier = Modifier
                            .weight(1f)
                            .clickable {
                                authMode = option
                                if (option == AuthMode.XOAUTH2) {
                                    if (host.isBlank()) host = "smtp.gmail.com"
                                    if (port.isBlank() || port == "465") port = "587"
                                    security = SmtpSecurity.STARTTLS
                                    if (username.isBlank()) username = email
                                }
                            }
                            .padding(vertical = 8.dp),
                        horizontalAlignment = Alignment.CenterHorizontally,
                    ) {
                        if (selected) {
                            Icon(
                                painter = painterResource(R.drawable.ic_check),
                                contentDescription = "Selected",
                                modifier = Modifier.size(18.dp),
                            )
                        } else {
                            Spacer(Modifier.height(18.dp))
                        }
                        Text(
                            if (option == AuthMode.STANDARD) "Password" else "Gmail XOAUTH2",
                            style = MaterialTheme.typography.bodyLarge,
                            fontWeight = if (selected) FontWeight.Bold else FontWeight.Normal,
                        )
                    }
                }
            }

            Spacer(Modifier.height(12.dp))
            OutlinedTextField(
                value = host,
                onValueChange = { host = it },
                label = { Text("SMTP host") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )
            Spacer(Modifier.height(12.dp))
            OutlinedTextField(
                value = port,
                onValueChange = { port = it.filter { char -> char.isDigit() } },
                label = { Text("SMTP port") },
                singleLine = true,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                modifier = Modifier.fillMaxWidth(),
            )
            Spacer(Modifier.height(12.dp))
            Text("Security", style = MaterialTheme.typography.bodyLarge)
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceEvenly,
            ) {
                for (option in listOf(SmtpSecurity.STARTTLS, SmtpSecurity.SSL, SmtpSecurity.NONE)) {
                    val selected = security == option
                    Column(
                        modifier = Modifier
                            .weight(1f)
                            .clickable { security = option }
                            .padding(vertical = 8.dp),
                        horizontalAlignment = Alignment.CenterHorizontally,
                    ) {
                        if (selected) {
                            Icon(
                                painter = painterResource(R.drawable.ic_check),
                                contentDescription = "Selected",
                                modifier = Modifier.size(18.dp),
                            )
                        } else {
                            Spacer(Modifier.height(18.dp))
                        }
                        Text(
                            when (option) {
                                SmtpSecurity.STARTTLS -> "STARTTLS"
                                SmtpSecurity.SSL -> "SSL"
                                SmtpSecurity.NONE -> "None"
                            },
                            style = MaterialTheme.typography.bodyLarge,
                            fontWeight = if (selected) FontWeight.Bold else FontWeight.Normal,
                        )
                    }
                }
            }

            Spacer(Modifier.height(12.dp))
            if (authMode == AuthMode.STANDARD) {
                OutlinedTextField(
                    value = username,
                    onValueChange = { username = it },
                    label = { Text("Username") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                )
                Spacer(Modifier.height(12.dp))
                OutlinedTextField(
                    value = secret,
                    onValueChange = { secret = it },
                    label = { Text("Password") },
                    placeholder = if (hasSecret) {
                        { Text("Stored; leave blank to keep") }
                    } else {
                        null
                    },
                    singleLine = true,
                    visualTransformation = if (secretVisible) {
                        VisualTransformation.None
                    } else {
                        PasswordVisualTransformation()
                    },
                    trailingIcon = {
                        IconButton(onClick = { secretVisible = !secretVisible }) {
                            Icon(
                                painter = painterResource(
                                    if (secretVisible) R.drawable.ic_visibility_off
                                    else R.drawable.ic_visibility,
                                ),
                                contentDescription = if (secretVisible) "Hide password" else "Show password",
                            )
                        }
                    },
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password),
                    modifier = Modifier.fillMaxWidth(),
                )
            } else {
                OutlinedTextField(
                    value = tokenPath,
                    onValueChange = { tokenPath = it },
                    label = { Text("Token file path") },
                    placeholder = { Text("/sdcard/.../mail.example.token.gpg") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                )
                Spacer(Modifier.height(12.dp))
                OutlinedTextField(
                    value = secret,
                    onValueChange = { secret = it },
                    label = { Text("Token file passphrase") },
                    placeholder = if (hasSecret) {
                        { Text("Stored; leave blank to keep") }
                    } else {
                        null
                    },
                    singleLine = true,
                    visualTransformation = if (secretVisible) {
                        VisualTransformation.None
                    } else {
                        PasswordVisualTransformation()
                    },
                    trailingIcon = {
                        IconButton(onClick = { secretVisible = !secretVisible }) {
                            Icon(
                                painter = painterResource(
                                    if (secretVisible) R.drawable.ic_visibility_off
                                    else R.drawable.ic_visibility,
                                ),
                                contentDescription = if (secretVisible) "Hide passphrase" else "Show passphrase",
                            )
                        }
                    },
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password),
                    modifier = Modifier.fillMaxWidth(),
                )
            }

            Spacer(Modifier.height(20.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                OutlinedButton(
                    onClick = { vm.testIdentityForm(buildIdentity(), secret) },
                    enabled = vm.testingIdentity == null && !vm.sending,
                ) {
                    Text("Test")
                }
                if (vm.testingIdentity != null) {
                    Spacer(Modifier.width(12.dp))
                    Text("Testing...", style = MaterialTheme.typography.bodyMedium)
                }
            }
            vm.identityTestResult?.let { result ->
                Spacer(Modifier.height(8.dp))
                Text(result, style = MaterialTheme.typography.bodyMedium, fontWeight = FontWeight.Bold)
            }

            Spacer(Modifier.height(20.dp))
            Text("Signature", style = MaterialTheme.typography.titleMedium)
            Spacer(Modifier.height(4.dp))
            OutlinedTextField(
                value = signature,
                onValueChange = { signature = it },
                placeholder = { Text("Appended to new messages, replies, and forwards") },
                minLines = 3,
                modifier = Modifier.fillMaxWidth(),
            )

            if (error != null) {
                Spacer(Modifier.height(8.dp))
                Text(error!!, fontWeight = FontWeight.Bold, fontSize = 14.sp)
            }
            Spacer(Modifier.height(20.dp))
        }
    }
}

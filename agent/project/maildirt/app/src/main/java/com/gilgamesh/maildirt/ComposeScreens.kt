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
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.text.TextRange
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

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun ComposeScreen(vm: MainViewModel) {
    val seed = vm.composeSeed ?: return
    val identities = vm.identities()
    val identity = identities.firstOrNull { it.id == seed.identityId } ?: identities.firstOrNull()
    val context = LocalContext.current
    var to by remember { mutableStateOf(TextFieldValue(seed.to)) }
    var cc by remember { mutableStateOf(TextFieldValue(seed.cc)) }
    var bcc by remember { mutableStateOf(TextFieldValue(seed.bcc)) }
    var subject by remember { mutableStateOf(seed.subject) }
    var bodyField by remember {
        mutableStateOf(TextFieldValue(seed.body, selection = TextRange(0)))
    }
    val body = bodyField.text
    var attachments by remember { mutableStateOf(seed.attachments) }
    var activeField by remember { mutableStateOf<String?>(null) }
    val sending = vm.sending
    val posting = vm.posting
    val paged = vm.config.pagedScrolling
    val scrollState = rememberScrollState()

    val picker = rememberLauncherForActivityResult(
        ActivityResultContracts.OpenMultipleDocuments(),
    ) { uris ->
        if (uris.isNotEmpty()) {
            attachments = attachments + uris.mapNotNull { attachmentFor(context, it) }
        }
    }

    val fields = ComposeFields(
        identityId = identity?.id.orEmpty(),
        to = to.text,
        cc = cc.text,
        bcc = bcc.text,
        subject = subject,
        body = body,
    )
    val activeValue = when (activeField) {
        "to" -> to.text
        "cc" -> cc.text
        "bcc" -> bcc.text
        else -> ""
    }
    val activeCaret = when (activeField) {
        "to" -> to.selection.end
        "cc" -> cc.selection.end
        "bcc" -> bcc.selection.end
        else -> 0
    }
    val activeToken = MailText.addressToken(activeValue, activeCaret).text
    val suggestions = if (activeToken.isBlank()) {
        emptyList()
    } else {
        vm.addressSuggestions(activeToken)
    }

    BackHandler(enabled = true) {
        vm.postponeCompose(fields, attachments)
    }

    LaunchedEffect(vm.composeError) {
        if (vm.composeError != null) scrollState.animateScrollTo(0)
    }

    Scaffold(
        topBar = {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(48.dp)
                    .background(Color.White)
                    .padding(horizontal = 4.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                IconButton(
                    onClick = { vm.postponeCompose(fields, attachments) },
                    enabled = !posting && !sending,
                ) {
                    Icon(
                        painter = painterResource(R.drawable.ic_draft),
                        contentDescription = "Save draft",
                    )
                }
                Text(
                    when {
                        seed.draftPath != null -> "Draft"
                        seed.subject.startsWith("Fwd:", true) -> "Forward"
                        seed.inReplyTo != null -> "Reply"
                        else -> "New message"
                    },
                    style = MaterialTheme.typography.titleMedium,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f).padding(horizontal = 4.dp),
                )
                IconButton(
                    onClick = { vm.discardCompose() },
                    enabled = !sending && !posting,
                ) {
                    Icon(
                        painter = painterResource(R.drawable.ic_delete),
                        contentDescription = "Discard",
                    )
                }
                IconButton(
                    onClick = { vm.sendCompose(fields, attachments) },
                    enabled = !sending && !posting,
                ) {
                    Icon(
                        painter = painterResource(R.drawable.ic_send),
                        contentDescription = "Send",
                    )
                }
            }
        },
    ) { padding ->
        val base = Modifier
            .fillMaxSize()
            .padding(padding)
            .verticalScroll(scrollState, enabled = !paged)
            .padding(horizontal = 12.dp, vertical = 6.dp)
        val modifier = if (paged) base.pagedScroll(scrollState) else base
        Column(modifier = modifier) {
            vm.composeError?.let { error ->
                Text(error, fontWeight = FontWeight.Bold, fontSize = 14.sp)
                Spacer(Modifier.height(8.dp))
            }
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .clickable { vm.openIdentityPicker(fields, attachments) }
                    .padding(vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Column(Modifier.weight(1f)) {
                    Text("From", fontSize = 13.sp)
                    Text(
                        identity?.displayLabel ?: "Choose identity",
                        style = MaterialTheme.typography.bodyLarge,
                    )
                }
                Text("\u203A", fontSize = 26.sp)
            }

            RecipientField(
                label = "To",
                value = to,
                onValueChange = { to = it },
                onFocus = { activeField = "to" },
                suggestions = if (activeField == "to") suggestions else emptyList(),
                onPick = { to = to.autofilled(it); activeField = "to" },
            )
            RecipientField(
                label = "Cc",
                value = cc,
                onValueChange = { cc = it },
                onFocus = { activeField = "cc" },
                suggestions = if (activeField == "cc") suggestions else emptyList(),
                onPick = { cc = cc.autofilled(it); activeField = "cc" },
            )
            RecipientField(
                label = "Bcc",
                value = bcc,
                onValueChange = { bcc = it },
                onFocus = { activeField = "bcc" },
                suggestions = if (activeField == "bcc") suggestions else emptyList(),
                onPick = { bcc = bcc.autofilled(it); activeField = "bcc" },
            )
            OutlinedTextField(
                value = subject,
                onValueChange = { subject = it.replace("\n", "") },
                label = { Text("Subject") },
                singleLine = false,
                minLines = 1,
                modifier = Modifier.fillMaxWidth(),
            )

            Spacer(Modifier.height(8.dp))
            Row(verticalAlignment = Alignment.CenterVertically) {
                IconButton(onClick = { picker.launch(arrayOf("*/*")) }) {
                    Icon(
                        painter = painterResource(R.drawable.ic_attach),
                        contentDescription = "Add attachment",
                    )
                }
                Text(
                    if (attachments.isEmpty()) {
                        "No attachments"
                    } else {
                        "${attachments.size} attachment" + if (attachments.size == 1) "" else "s"
                    },
                    style = MaterialTheme.typography.bodyMedium,
                )
            }
            for ((index, attachment) in attachments.withIndex()) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        attachment.name,
                        style = MaterialTheme.typography.bodyMedium,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.weight(1f),
                    )
                    TextButton(onClick = {
                        attachments = attachments.filterIndexed { i, _ -> i != index }
                    }) {
                        Text("Remove")
                    }
                }
            }

            Spacer(Modifier.height(8.dp))
            OutlinedTextField(
                value = bodyField,
                onValueChange = { bodyField = it },
                placeholder = { Text("Message") },
                minLines = 6,
                modifier = Modifier.fillMaxWidth(),
            )

            if (sending) {
                Spacer(Modifier.height(8.dp))
                Text("Sending...", fontWeight = FontWeight.Bold)
            }
            if (posting) {
                Spacer(Modifier.height(8.dp))
                Text("Saving draft...", fontWeight = FontWeight.Bold)
            }

            Spacer(Modifier.height(24.dp))
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun RecipientField(
    label: String,
    value: TextFieldValue,
    onValueChange: (TextFieldValue) -> Unit,
    onFocus: () -> Unit,
    suggestions: List<AddressEntry>,
    onPick: (AddressEntry) -> Unit,
) {
    OutlinedTextField(
        value = value,
        onValueChange = { next -> onValueChange(next.withoutNewlines()) },
        label = { Text(label) },
        singleLine = false,
        minLines = 1,
        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Email),
        modifier = Modifier
            .fillMaxWidth()
            .onFocusChanged { if (it.isFocused) onFocus() },
    )
    for (suggestion in suggestions) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .clickable { onPick(suggestion) }
                .padding(horizontal = 12.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column {
                if (suggestion.name.isNotBlank()) {
                    Text(
                        suggestion.name,
                        style = MaterialTheme.typography.bodyLarge,
                        fontWeight = FontWeight.Bold,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
                Text(
                    suggestion.address,
                    style = MaterialTheme.typography.bodyMedium,
                    fontSize = 16.sp,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun IdentityPickerScreen(vm: MainViewModel) {
    val identities = vm.identities()
    Scaffold(
        topBar = {
            AppTopBar(
                title = "From",
                onBack = { vm.back() },
            )
        },
    ) { padding ->
        if (identities.isEmpty()) {
            Text(
                "No sender identities configured.",
                modifier = Modifier.padding(padding).padding(20.dp),
            )
        } else {
            LazyColumn(modifier = Modifier.fillMaxSize().padding(padding)) {
                items(identities, key = { it.id }) { identity ->
                    ListRow(
                        title = identity.displayLabel,
                        subtitle = if (identity.authMode == AuthMode.XOAUTH2) {
                            "Gmail XOAUTH2 · ${identity.smtpHost}"
                        } else {
                            "SMTP · ${identity.smtpHost}"
                        },
                        onClick = { vm.chooseIdentity(identity.id) },
                    )
                }
            }
        }
    }
}

/** Recipient text never takes literal newlines; wrapping is display-only. */
internal fun TextFieldValue.withoutNewlines(): TextFieldValue {
    if (!text.contains('\n')) return this
    val next = text.replace("\n", "")
    return copy(text = next, selection = TextRange(selection.end.coerceAtMost(next.length)))
}

/** Autocomplete replacement of the token at the caret, leaving the caret after it. */
internal fun TextFieldValue.autofilled(entry: AddressEntry): TextFieldValue {
    val address = MailText.formatAddress(entry.name, entry.address)
    val token = MailText.addressToken(text, selection.end)
    val next = text.substring(0, token.start) + address + text.substring(token.end)
    return copy(text = next, selection = TextRange(token.start + address.length))
}

internal fun attachmentFor(context: Context, uri: Uri): OutgoingAttachment? = runCatching {
    runCatching {
        context.contentResolver.takePersistableUriPermission(
            uri,
            Intent.FLAG_GRANT_READ_URI_PERMISSION,
        )
    }
    val name = context.contentResolver.query(
        uri,
        arrayOf(OpenableColumns.DISPLAY_NAME),
        null,
        null,
        null,
    )?.use { cursor ->
        if (cursor.moveToFirst()) cursor.getString(0) else null
    } ?: uri.lastPathSegment ?: "attachment"
    val mimeType = context.contentResolver.getType(uri) ?: "application/octet-stream"
    OutgoingAttachment(uri, name, mimeType)
}.getOrNull()

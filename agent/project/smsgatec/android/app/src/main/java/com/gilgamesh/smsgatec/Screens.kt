package com.gilgamesh.smsgatec

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
fun SmsgatecApp(
    vm: AppViewModel,
    state: UiState,
    onPickAttachment: () -> Unit,
    onSaveAttachment: (String) -> Unit,
) {
    val palette = LocalEink.current
    Column(Modifier.fillMaxSize().background(palette.bg)) {
        when (val screen = state.screen) {
            Screen.Conversations -> ConversationsScreen(vm, state)
            is Screen.Thread -> ThreadScreen(vm, state, screen, onPickAttachment)
            is Screen.AttachmentView -> AttachmentScreen(vm, state, onSaveAttachment)
            Screen.Settings -> SettingsScreen(vm, state)
            is Screen.AccountEditor -> AccountEditorScreen(vm, state, screen.id)
        }
    }
}

@Composable
private fun ConversationsScreen(vm: AppViewModel, state: UiState) {
    val palette = LocalEink.current
    var accountMenu by remember { mutableStateOf(false) }
    var actionsMenu by remember { mutableStateOf(false) }
    Column(Modifier.fillMaxSize()) {
        if (state.searching) {
            SearchTopBar(state.query, vm::setQuery, vm::clearSearch)
        } else {
            EinkTopBar(
                title = state.accountName,
                onTitleClick = if (state.accounts.size > 1) {
                    { accountMenu = true }
                } else {
                    null
                },
                actions = listOf(
                    TopAction(R.drawable.ic_search, "Search") { vm.toggleSearch() },
                    TopAction(R.drawable.ic_add, "New message") { vm.openNewThread() },
                ),
                titleMenu = {
                    DropdownMenu(
                        expanded = accountMenu,
                        onDismissRequest = { accountMenu = false },
                    ) {
                        for (account in state.accounts) {
                            DropdownMenuItem(
                                text = {
                                    Text(
                                        account.name,
                                        color = palette.fg,
                                        fontSize = 16.sp,
                                        fontWeight = if (account.id == state.config.activeAccount) {
                                            FontWeight.SemiBold
                                        } else {
                                            FontWeight.Normal
                                        },
                                    )
                                },
                                trailingIcon = if (account.id == state.config.activeAccount) {
                                    {
                                        Icon(
                                            painter = painterResource(R.drawable.ic_check),
                                            contentDescription = "Current account",
                                            tint = palette.fg,
                                            modifier = Modifier.size(18.dp),
                                        )
                                    }
                                } else {
                                    null
                                },
                                onClick = {
                                    accountMenu = false
                                    vm.selectAccount(account.id)
                                },
                            )
                        }
                    }
                },
                trailing = {
                    Box {
                        IconAction(R.drawable.ic_more_vert, "More") { actionsMenu = true }
                        DropdownMenu(
                            expanded = actionsMenu,
                            onDismissRequest = { actionsMenu = false },
                        ) {
                            DropdownMenuItem(
                                text = { Text("Refresh", color = palette.fg, fontSize = 16.sp) },
                                onClick = {
                                    actionsMenu = false
                                    vm.syncNow(full = true)
                                },
                            )
                            DropdownMenuItem(
                                text = { Text("Settings", color = palette.fg, fontSize = 16.sp) },
                                onClick = {
                                    actionsMenu = false
                                    vm.openScreen(Screen.Settings)
                                },
                            )
                        }
                    }
                },
            )
        }
        Box(Modifier.weight(1f)) {
            if (!state.hasAccounts) {
                EmptyAction(
                    "No gateway account yet. Add the SMSGate server address and credentials.",
                    "Open Settings",
                ) { vm.openScreen(Screen.Settings) }
            } else {
                EinkList(
                    entries = state.threads,
                    paged = state.config.pagedScrolling,
                    empty = if (state.searching && state.query.isNotEmpty()) {
                        "No conversations match this search."
                    } else {
                        "No messages yet. Refresh to read the gateway history."
                    },
                ) { row ->
                    ConversationRow(row) { vm.openThread(row.number) }
                }
            }
        }
        EinkStatusBar(state)
    }
}

@Composable
private fun ConversationRow(row: ThreadRow, onClick: () -> Unit) {
    val palette = LocalEink.current
    Column(
        Modifier
            .fillMaxWidth()
            .clickable { onClick() }
            .padding(horizontal = 12.dp, vertical = 8.dp)
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(
                text = row.name,
                color = palette.fg,
                fontSize = 16.sp,
                fontWeight = FontWeight.SemiBold,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.weight(1f),
            )
            Spacer(Modifier.width(8.dp))
            Text(row.time, color = palette.dim, fontSize = 11.sp)
        }
        Row(verticalAlignment = Alignment.CenterVertically) {
            if (row.hasMedia) {
                Icon(
                    painter = painterResource(R.drawable.ic_attachment),
                    contentDescription = "Attachment",
                    tint = palette.dim,
                    modifier = Modifier.size(13.dp),
                )
                Spacer(Modifier.width(3.dp))
            }
            Text(
                text = row.snippet,
                color = palette.dim,
                fontSize = 13.sp,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.weight(1f),
            )
            if (row.state.isNotEmpty()) {
                Spacer(Modifier.width(6.dp))
                Text(row.state, color = palette.dim, fontSize = 10.sp, maxLines = 1)
            }
        }
    }
}

@Composable
private fun ThreadScreen(
    vm: AppViewModel,
    state: UiState,
    screen: Screen.Thread,
    onPickAttachment: () -> Unit,
) {
    val palette = LocalEink.current
    val nameMap = remember(state.contactRows) {
        state.contactRows.associate { it.number to it.name }
    }
    Column(Modifier.fillMaxSize()) {
        if (state.searching) {
            SearchTopBar(state.query, vm::setQuery, vm::clearSearch)
        } else {
            EinkTopBar(
                title = if (screen.newThread) "New message" else state.currentName.ifEmpty { screen.number },
                onBack = { vm.back() },
                actions = listOf(
                    TopAction(R.drawable.ic_search, "Search messages") { vm.toggleSearch() },
                ),
            )
        }
        Box(Modifier.weight(1f)) {
            EinkList(
                entries = state.threadMessages,
                paged = state.config.pagedScrolling,
                empty = if (state.searching && state.query.isNotEmpty()) {
                    "No messages match this search."
                } else if (screen.newThread) {
                    "Write the first message below."
                } else {
                    "No messages yet."
                },
                scrollToEnd = true,
            ) { message ->
                MessageBlock(message, state.currentName, nameMap) { index ->
                    vm.openAttachment(message.key, index)
                }
            }
        }
        if (!state.searching) {
            InlineComposer(vm, state, screen, onPickAttachment)
        }
    }
}

@Composable
private fun InlineComposer(
    vm: AppViewModel,
    state: UiState,
    screen: Screen.Thread,
    onPickAttachment: () -> Unit,
) {
    val palette = LocalEink.current
    val compose = state.compose
    val nameMap = remember(state.contactRows) {
        state.contactRows.associate { it.number to it.name }
    }
    val suggestionQuery = compose.number.lowercase()
    val suggestions = if (screen.newThread && compose.number.isNotBlank() && recipient(compose.number) == null) {
        val qDigits = digits(suggestionQuery)
        state.contactRows.filter {
            it.name.lowercase().contains(suggestionQuery) ||
                (qDigits.isNotEmpty() && digits(it.number).contains(qDigits))
        }.take(5)
    } else {
        emptyList()
    }
    Column(
        Modifier
            .fillMaxWidth()
            .background(palette.bg)
            .drawBehind {
                if (size.height > 0f) {
                    drawLine(palette.dim, Offset(0f, 0f), Offset(size.width, 0f), strokeWidth = 1f)
                }
            }
    ) {
        FormError(compose.error)
        if (compose.sending) {
            Text(
                "Sending…",
                color = palette.dim,
                fontSize = 12.sp,
                modifier = Modifier.padding(horizontal = 12.dp, vertical = 2.dp),
            )
        }
        if (screen.newThread) {
            for ((index, number) in compose.recipients.withIndex()) {
                Row(
                    Modifier.fillMaxWidth().padding(start = 12.dp, end = 4.dp, top = 2.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        nameMap[number] ?: number,
                        color = palette.fg,
                        fontSize = 14.sp,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.weight(1f),
                    )
                    IconAction(R.drawable.ic_close, "Remove recipient") {
                        vm.removeComposeRecipient(index)
                    }
                }
            }
            FormField(
                label = if (compose.recipients.isEmpty()) "To" else "Add recipient",
                value = compose.number,
                onValueChange = vm::setComposeNumber,
                placeholder = "+12025550123",
            )
            for (match in suggestions) {
                SectionRow(match.name, value = match.number) {
                    vm.addComposeRecipient(match.number)
                }
            }
            val typed = recipient(compose.number)
            if (typed != null && compose.recipients.none { phoneKey(it) == phoneKey(typed) }) {
                SectionRow("Add recipient", value = typed) {
                    vm.addComposeRecipient(typed)
                }
            }
        }
        if (compose.mms) {
            FormField(
                label = "Subject",
                value = compose.subject,
                onValueChange = vm::setComposeSubject,
                placeholder = "Optional",
            )
            for ((index, file) in compose.attachments.withIndex()) {
                Row(
                    Modifier.fillMaxWidth().padding(start = 12.dp, end = 4.dp, top = 2.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(
                        painter = painterResource(R.drawable.ic_attachment),
                        contentDescription = null,
                        tint = palette.fg,
                        modifier = Modifier.size(14.dp),
                    )
                    Spacer(Modifier.width(6.dp))
                    Text(
                        file.name,
                        color = palette.fg,
                        fontSize = 13.sp,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.weight(1f),
                    )
                    Text(
                        "${(file.size + 1023) / 1024} KB",
                        color = palette.dim,
                        fontSize = 10.sp,
                        modifier = Modifier.padding(horizontal = 6.dp),
                    )
                    IconAction(R.drawable.ic_close, "Remove attachment") {
                        vm.removeAttachment(index)
                    }
                }
            }
        }
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 4.dp, vertical = 4.dp),
            verticalAlignment = Alignment.Bottom,
        ) {
            Box(Modifier.padding(start = 4.dp, bottom = 8.dp)) {
                ModeChip(
                    label = if (compose.mms) "MMS" else "SMS",
                    active = compose.mms,
                    onClick = { vm.toggleMms() },
                )
            }
            Box(
                Modifier
                    .weight(1f)
                    .padding(horizontal = 6.dp, vertical = 8.dp)
            ) {
                if (compose.text.isEmpty()) {
                    Text("Message", color = palette.dim, fontSize = 16.sp)
                }
                BasicTextField(
                    value = compose.text,
                    onValueChange = vm::setComposeText,
                    minLines = 1,
                    maxLines = 5,
                    textStyle = TextStyle(color = palette.fg, fontSize = 16.sp, lineHeight = 21.sp),
                    cursorBrush = SolidColor(palette.fg),
                    modifier = Modifier.fillMaxWidth(),
                )
            }
            if (compose.mms) {
                IconAction(R.drawable.ic_attachment, "Add attachment", onClick = onPickAttachment)
            }
            IconAction(
                iconRes = R.drawable.ic_send,
                description = "Send",
                enabled = !compose.sending &&
                    (compose.text.isNotBlank() || compose.attachments.isNotEmpty()) &&
                    (compose.recipients.isNotEmpty() || recipient(compose.number) != null),
                onClick = { vm.send() },
            )
        }
    }
}


@Composable
private fun ModeChip(label: String, active: Boolean, onClick: () -> Unit) {
    val palette = LocalEink.current
    Box(
        Modifier
            .height(30.dp)
            .background(if (active) palette.fg else palette.bg)
            .border(1.dp, palette.fg)
            .clickable { onClick() }
            .padding(horizontal = 8.dp),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            text = label,
            color = if (active) palette.bg else palette.fg,
            fontSize = 11.sp,
            fontWeight = FontWeight.SemiBold,
            maxLines = 1,
        )
    }
}

@Composable
private fun MessageBlock(
    message: Message,
    contactName: String,
    nameMap: Map<String, String>,
    onOpenMedia: (Int) -> Unit,
) {
    val palette = LocalEink.current
    val outgoing = message.direction == "out"
    val textAlign = if (outgoing) TextAlign.End else TextAlign.Start
    val headerName = when {
        outgoing -> "You"
        message.isGroup && message.sender.isNotEmpty() ->
            nameMap[message.sender] ?: message.sender
        else -> contactName.ifEmpty { message.number }
    }
    Column(
        Modifier
            .fillMaxWidth()
            .padding(
                start = if (outgoing) 48.dp else 12.dp,
                end = if (outgoing) 12.dp else 48.dp,
                top = 8.dp,
                bottom = 8.dp,
            ),
        horizontalAlignment = if (outgoing) Alignment.End else Alignment.Start,
    ) {
        Row(
            Modifier.fillMaxWidth(),
            horizontalArrangement = if (outgoing) Arrangement.End else Arrangement.Start,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                text = headerName,
                color = palette.dim,
                fontSize = 11.sp,
                fontWeight = FontWeight.SemiBold,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                modifier = if (outgoing) Modifier else Modifier.weight(1f, fill = false),
            )
            Text(
                " · ${message.dateLabel()}",
                color = palette.dim,
                fontSize = 11.sp,
                maxLines = 1,
            )
            if (message.type.startsWith("MMS")) {
                Text(" · MMS", color = palette.dim, fontSize = 11.sp)
            }
            if (outgoing && message.state.isNotEmpty()) {
                Text(" · ${message.state}", color = palette.dim, fontSize = 11.sp)
            }
        }
        if (message.subject.isNotEmpty()) {
            Text(
                text = message.subject,
                color = palette.fg,
                fontSize = 15.sp,
                fontWeight = FontWeight.SemiBold,
                textAlign = textAlign,
                modifier = Modifier.fillMaxWidth().padding(top = 2.dp),
            )
        }
        if (message.text.isNotEmpty() && message.text != message.subject) {
            SelectionContainer {
                Text(
                    text = message.text,
                    color = palette.fg,
                    fontSize = 16.sp,
                    lineHeight = 22.sp,
                    textAlign = textAlign,
                    modifier = Modifier.fillMaxWidth().padding(top = 3.dp),
                )
            }
        }
        for ((index, part) in message.media.withIndex()) {
            if (outgoing) {
                Row(
                    Modifier
                        .clickable { onOpenMedia(index) }
                        .padding(top = 4.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(
                        painter = painterResource(R.drawable.ic_attachment),
                        contentDescription = "Open attachment",
                        tint = palette.fg,
                        modifier = Modifier.size(15.dp),
                    )
                    Spacer(Modifier.width(5.dp))
                    Text(
                        text = safeName(part.name, part.contentType),
                        color = palette.fg,
                        fontSize = 13.sp,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.widthIn(max = 200.dp),
                    )
                    if (part.size > 0) {
                        Text(
                            "${(part.size + 1023) / 1024} KB",
                            color = palette.dim,
                            fontSize = 11.sp,
                            modifier = Modifier.padding(start = 6.dp),
                        )
                    }
                }
            } else {
                Row(
                    Modifier
                        .fillMaxWidth()
                        .clickable { onOpenMedia(index) }
                        .padding(top = 4.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(
                        painter = painterResource(R.drawable.ic_attachment),
                        contentDescription = "Open attachment",
                        tint = palette.fg,
                        modifier = Modifier.size(15.dp),
                    )
                    Spacer(Modifier.width(5.dp))
                    Text(
                        text = safeName(part.name, part.contentType),
                        color = palette.fg,
                        fontSize = 13.sp,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.weight(1f),
                    )
                    if (part.size > 0) {
                        Text(
                            "${(part.size + 1023) / 1024} KB",
                            color = palette.dim,
                            fontSize = 11.sp,
                            modifier = Modifier.padding(start = 6.dp),
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun AttachmentScreen(vm: AppViewModel, state: UiState, onSave: (String) -> Unit) {
    val palette = LocalEink.current
    val attachment = state.attachment ?: return
    Column(Modifier.fillMaxSize()) {
        EinkTopBar(title = "Attachment", onBack = { vm.back() })
        EinkScrollColumn(paged = state.config.pagedScrolling, modifier = Modifier.weight(1f)) {
            Text(
                safeName(attachment.part.name, attachment.part.contentType),
                color = palette.fg,
                fontSize = 18.sp,
                fontWeight = FontWeight.SemiBold,
                modifier = Modifier.padding(horizontal = 12.dp, vertical = 10.dp),
            )
            Text(
                "${attachment.part.contentType.ifEmpty { "unknown type" }} · " +
                    if (attachment.part.size > 0) "${(attachment.part.size + 1023) / 1024} KB" else "size unknown",
                color = palette.dim,
                fontSize = 12.sp,
                modifier = Modifier.padding(horizontal = 12.dp),
            )
            Text(
                "Message: ${attachment.message.dateLabel()} · " +
                    if (attachment.message.direction == "out") "You" else state.currentName,
                color = palette.dim,
                fontSize = 12.sp,
                modifier = Modifier.padding(horizontal = 12.dp, vertical = 4.dp),
            )
            if (attachment.busy) {
                Text(
                    "Downloading…",
                    color = palette.dim,
                    fontSize = 14.sp,
                    modifier = Modifier.padding(12.dp),
                )
            }
            if (attachment.error.isNotEmpty()) {
                FormError(attachment.error)
            }
            Spacer(Modifier.height(8.dp))
            SectionRow("Open", icon = R.drawable.ic_open, onClick = {
                val path = attachment.path
                if (path != null) vm.markOpen(path) else vm.notice("Attachment is not available yet")
            })
            SectionRow("Save", icon = R.drawable.ic_download, onClick = {
                val path = attachment.path
                if (path != null) onSave(attachment.part.name) else vm.notice("Attachment is not available yet")
            })
        }
        EinkStatusBar(state)
    }
}

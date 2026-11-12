package com.gilgamesh.smsgatec

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
fun SettingsScreen(vm: AppViewModel, state: UiState) {
    val palette = LocalEink.current
    val config = state.config
    var poll by remember(config.pollSeconds) { mutableStateOf(config.pollSeconds.toString()) }
    var reconcile by remember(config.reconcileSeconds) { mutableStateOf(config.reconcileSeconds.toString()) }
    var timeout by remember(config.timeoutSeconds) { mutableStateOf(config.timeoutSeconds.toString()) }
    var error by remember { mutableStateOf("") }
    var accountMenu by remember { mutableStateOf<String?>(null) }

    fun commit() {
        val pollValue = poll.toIntOrNull()
        val reconcileValue = reconcile.toIntOrNull()
        val timeoutValue = timeout.toIntOrNull()
        if (pollValue == null || reconcileValue == null || timeoutValue == null) {
            error = "Intervals and timeout must be numbers"
            return
        }
        val next = config.copy(
            pollSeconds = pollValue,
            reconcileSeconds = reconcileValue,
            timeoutSeconds = timeoutValue,
        )
        error = vm.saveSettings(next) ?: ""
    }

    BackHandler(enabled = true) { commit() }

    Column(Modifier.fillMaxSize()) {
        EinkTopBar(
            title = "Settings",
            onBack = { commit() },
        )
        FormError(error)
        EinkScrollColumn(paged = state.config.pagedScrolling, modifier = Modifier.weight(1f)) {
            Row(
                Modifier
                    .fillMaxWidth()
                    .padding(start = 12.dp, top = 4.dp, bottom = 0.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    "ACCOUNTS",
                    color = palette.dim,
                    fontSize = 10.sp,
                    modifier = Modifier.weight(1f),
                )
                IconAction(R.drawable.ic_add, "Add account") {
                    vm.openScreen(Screen.AccountEditor(null))
                }
            }
            for ((index, account) in state.accounts.withIndex()) {
                Row(
                    Modifier
                        .fillMaxWidth()
                        .clickable { vm.openScreen(Screen.AccountEditor(account.id)) }
                        .padding(start = 12.dp, top = 10.dp, bottom = 10.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Column(Modifier.weight(1f)) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(
                                account.name,
                                color = palette.fg,
                                fontSize = 16.sp,
                                fontWeight = FontWeight.SemiBold,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                            )
                            if (account.id == config.activeAccount) {
                                Text("  current", color = palette.dim, fontSize = 11.sp)
                            }
                        }
                        Text(
                            account.server,
                            color = palette.dim,
                            fontSize = 12.sp,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                    Box {
                        IconAction(R.drawable.ic_more_vert, "Account actions") {
                            accountMenu = account.id
                        }
                        DropdownMenu(
                            expanded = accountMenu == account.id,
                            onDismissRequest = { accountMenu = null },
                        ) {
                            DropdownMenuItem(
                                text = { Text("Edit", color = palette.fg, fontSize = 16.sp) },
                                onClick = {
                                    accountMenu = null
                                    vm.openScreen(Screen.AccountEditor(account.id))
                                },
                            )
                            if (index > 0) {
                                DropdownMenuItem(
                                    text = { Text("Move up", color = palette.fg, fontSize = 16.sp) },
                                    onClick = {
                                        accountMenu = null
                                        vm.moveAccount(account.id, -1)
                                    },
                                )
                            }
                            if (index < state.accounts.lastIndex) {
                                DropdownMenuItem(
                                    text = { Text("Move down", color = palette.fg, fontSize = 16.sp) },
                                    onClick = {
                                        accountMenu = null
                                        vm.moveAccount(account.id, 1)
                                    },
                                )
                            }
                            if (state.accounts.size > 1) {
                                DropdownMenuItem(
                                    text = {
                                        Text(
                                            "Remove",
                                            color = palette.fg,
                                            fontSize = 16.sp,
                                            fontWeight = FontWeight.SemiBold,
                                        )
                                    },
                                    onClick = {
                                        accountMenu = null
                                        error = vm.deleteAccount(account.id) ?: ""
                                    },
                                )
                            }
                        }
                    }
                }
            }
            Text(
                "UPDATES",
                color = palette.dim,
                fontSize = 10.sp,
                modifier = Modifier.padding(start = 12.dp, top = 10.dp, bottom = 2.dp),
            )
            SectionRow(
                "Update mode",
                value = if (config.updateMode == "push") "Push" else "Poll",
                onClick = {
                    vm.setUpdateMode(if (config.updateMode == "push") "poll" else "push")
                },
            )
            FormField("Poll seconds (15–3600)", poll, { poll = it }, numeric = true)
            FormField("Reconcile seconds (60–86400)", reconcile, { reconcile = it }, numeric = true)
            FormField("Timeout seconds (3–120)", timeout, { timeout = it }, numeric = true)
            Text(
                "APPEARANCE",
                color = palette.dim,
                fontSize = 10.sp,
                modifier = Modifier.padding(start = 12.dp, top = 10.dp, bottom = 2.dp),
            )
            SectionRow(
                "Theme",
                value = when (config.theme) {
                    "dark" -> "Dark"
                    "light" -> "Light"
                    else -> "Auto"
                },
                onClick = {
                    val next = when (config.theme) {
                        "auto" -> "light"
                        "light" -> "dark"
                        else -> "auto"
                    }
                    vm.setTheme(next)
                },
            )
            SectionRow(
                "Scrolling",
                value = if (config.pagedScrolling) "Paged" else "Smooth",
                onClick = { vm.togglePagedScrolling() },
            )
            Text(
                "MAINTENANCE",
                color = palette.dim,
                fontSize = 10.sp,
                modifier = Modifier.padding(start = 12.dp, top = 10.dp, bottom = 2.dp),
            )
            SectionRow("Refresh now", icon = R.drawable.ic_refresh) { vm.syncNow(full = true) }
            Spacer(Modifier.height(10.dp))
            Text(
                "${state.messageCount} messages · ${state.contactCount} contacts · v${APP_VERSION}",
                color = palette.dim,
                fontSize = 11.sp,
                modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp),
            )
            Text(
                "Passwords are encrypted with an Android Keystore key and never stored in config.json.",
                color = palette.dim,
                fontSize = 11.sp,
                modifier = Modifier.padding(horizontal = 12.dp, vertical = 4.dp),
            )
            Spacer(Modifier.height(16.dp))
        }
    }
}

@Composable
fun AccountEditorScreen(vm: AppViewModel, state: UiState, id: String?) {
    val palette = LocalEink.current
    val existing = state.accounts.firstOrNull { it.id == id }
    var name by remember(id) { mutableStateOf(existing?.name ?: "") }
    var server by remember(id) { mutableStateOf(existing?.server ?: "http://") }
    var username by remember(id) { mutableStateOf(existing?.username ?: "") }
    var password by remember(id) { mutableStateOf("") }
    var error by remember(id) { mutableStateOf("") }
    var testResult by remember(id) { mutableStateOf("") }

    fun commit() {
        val result = vm.saveAccount(id, name, server, username, password)
        if (result != null) {
            error = result
        } else {
            vm.openScreen(Screen.Settings)
        }
    }

    fun commitOrDiscard() {
        if (existing == null && name.isBlank() && username.isBlank() && password.isBlank() &&
            (server.isBlank() || server == "http://")
        ) {
            vm.back()
        } else {
            commit()
        }
    }

    BackHandler(enabled = true) { commitOrDiscard() }

    Column(Modifier.fillMaxSize()) {
        EinkTopBar(
            title = if (existing == null) "New account" else "Edit account",
            onBack = { commitOrDiscard() },
        )
        FormError(error)
        EinkScrollColumn(paged = state.config.pagedScrolling, modifier = Modifier.weight(1f)) {
            FormField("Name", name, { name = it }, placeholder = "Android phone")
            FormField("Server", server, { server = it }, placeholder = "http://127.0.0.1:8080")
            FormField("Username", username, { username = it })
            FormField(
                "Password", password, { password = it }, secret = true,
                placeholder = if (existing != null) "Leave blank to keep the current password" else "",
            )
            if (existing != null) {
                Text(
                    "Leave the password blank to keep the one stored on this device.",
                    color = palette.dim,
                    fontSize = 11.sp,
                    modifier = Modifier.padding(horizontal = 12.dp, vertical = 6.dp),
                )
            }
            SectionRow("Test connection", icon = R.drawable.ic_check) {
                val candidate = Account(
                    id = id ?: "test",
                    name = name, server = server.trim().trimEnd('/'), username = username.trim(),
                )
                testResult = "Testing…"
                vm.testAccount(candidate, password) { result -> testResult = result }
            }
            if (testResult.isNotEmpty()) {
                Text(
                    testResult,
                    color = palette.fg,
                    fontSize = 13.sp,
                    modifier = Modifier.padding(horizontal = 12.dp, vertical = 6.dp),
                )
            }
            Spacer(Modifier.height(16.dp))
        }
    }
}

package com.gilgamesh.smsgatec

import android.content.Intent
import android.net.Uri
import android.os.Bundle
import androidx.core.content.ContextCompat
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.platform.LocalContext
import androidx.core.content.FileProvider
import androidx.lifecycle.viewmodel.compose.viewModel
import java.io.File

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            val vm: AppViewModel = viewModel()
            val state = vm.state
            val context = LocalContext.current
            val attachLauncher = rememberLauncherForActivityResult(
                ActivityResultContracts.OpenDocument()
            ) { uri -> uri?.let { vm.addAttachment(it) } }
            val saveLauncher = rememberLauncherForActivityResult(
                ActivityResultContracts.CreateDocument("*/*")
            ) { uri -> uri?.let { vm.saveAttachment(it) } }
            val contactsPermission = rememberLauncherForActivityResult(
                ActivityResultContracts.RequestPermission()
            ) { vm.onContactsPermissionChanged() }
            LaunchedEffect(Unit) {
                if (ContextCompat.checkSelfPermission(
                        context, android.Manifest.permission.READ_CONTACTS,
                    ) != android.content.pm.PackageManager.PERMISSION_GRANTED
                ) {
                    contactsPermission.launch(android.Manifest.permission.READ_CONTACTS)
                }
            }
            LaunchedEffect(state.openPath) {
                val path = state.openPath ?: return@LaunchedEffect
                try {
                    val file = File(path)
                    val uri = FileProvider.getUriForFile(
                        context, "${context.packageName}.fileprovider", file,
                    )
                    val intent = Intent(Intent.ACTION_VIEW).apply {
                        setDataAndType(uri, context.contentResolver.getType(uri) ?: "*/*")
                        addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
                    }
                    context.startActivity(intent)
                } catch (e: Exception) {
                    vm.notice("No app can open this attachment; save it instead")
                }
                vm.openedFile()
            }

            val dark = when (state.config.theme) {
                "dark" -> true
                "light" -> false
                else -> resources.configuration.uiMode and
                    android.content.res.Configuration.UI_MODE_NIGHT_MASK ==
                    android.content.res.Configuration.UI_MODE_NIGHT_YES
            }
            SmsgatecTheme(dark) {
                BackHandler(enabled = state.screen != Screen.Conversations) { vm.back() }
                SmsgatecApp(
                    vm = vm,
                    state = state,
                    onPickAttachment = { attachLauncher.launch(arrayOf("*/*")) },
                    onSaveAttachment = { name -> saveLauncher.launch(safeName(name, "")) },
                )
            }
        }
    }
}

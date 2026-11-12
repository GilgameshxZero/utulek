package com.gilgamesh.maildirt

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.LocalRippleConfiguration
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        handleNotificationIntent(intent)
        MailNotificationService.start(this)
        setContent {
            MaildirtTheme {
                MaildirtApp()
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        handleNotificationIntent(intent)
    }

    override fun onResume() {
        super.onResume()
        AppVisibility.foreground = true
    }

    override fun onPause() {
        AppVisibility.foreground = false
        super.onPause()
    }

    private fun handleNotificationIntent(intent: Intent?) {
        val path = intent?.getStringExtra("path") ?: return
        val folderId = intent.getStringExtra("folderId")
        val folderRole = intent.getStringExtra("folderRole")
        OpenRequests.pending = OpenRequest(folderId, folderRole, path)
        intent.removeExtra("folderId")
        intent.removeExtra("folderRole")
        intent.removeExtra("path")
    }
}

private val Black = Color(0xFF000000)
private val White = Color(0xFFFFFFFF)
val InkDim = Color(0x9E000000)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MaildirtTheme(content: @Composable () -> Unit) {
    CompositionLocalProvider(LocalRippleConfiguration provides null) {
        MaterialTheme(
            colorScheme = lightColorScheme(
                primary = Black,
                onPrimary = White,
                primaryContainer = Black,
                onPrimaryContainer = White,
                inversePrimary = Black,
                secondary = Black,
                onSecondary = White,
                secondaryContainer = White,
                onSecondaryContainer = Black,
                tertiary = Black,
                onTertiary = White,
                tertiaryContainer = White,
                onTertiaryContainer = Black,
                background = White,
                onBackground = Black,
                surface = White,
                onSurface = Black,
                surfaceVariant = White,
                onSurfaceVariant = Black,
                surfaceTint = White,
                inverseSurface = Black,
                inverseOnSurface = White,
                error = Black,
                onError = White,
                errorContainer = White,
                onErrorContainer = Black,
                outline = Black,
                outlineVariant = Black,
                scrim = Black,
                surfaceBright = White,
                surfaceDim = White,
                surfaceContainerLowest = White,
                surfaceContainerLow = White,
                surfaceContainer = White,
                surfaceContainerHigh = White,
                surfaceContainerHighest = White,
            ),
            typography = Typography(
                bodyLarge = TextStyle(fontSize = 16.sp, lineHeight = 22.sp),
                bodyMedium = TextStyle(fontSize = 14.sp, lineHeight = 20.sp),
                titleLarge = TextStyle(fontSize = 20.sp, fontWeight = FontWeight.SemiBold),
                titleMedium = TextStyle(fontSize = 17.sp, fontWeight = FontWeight.SemiBold),
            ),
            content = content,
        )
    }
}

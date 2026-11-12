package com.gilgamesh.maildirt

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContent {
            MaildirtTheme {
                MaildirtApp()
            }
        }
    }
}

@Composable
fun MaildirtTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = lightColorScheme(
            primary = Color.Black,
            onPrimary = Color.White,
            primaryContainer = Color.Black,
            onPrimaryContainer = Color.White,
            secondary = Color.Black,
            onSecondary = Color.White,
            background = Color.White,
            onBackground = Color.Black,
            surface = Color.White,
            onSurface = Color.Black,
            surfaceVariant = Color(0xFFEEEEEE),
            onSurfaceVariant = Color.Black,
            outline = Color.Black,
        ),
        typography = Typography(
            bodyLarge = TextStyle(fontSize = 18.sp, lineHeight = 26.sp),
            bodyMedium = TextStyle(fontSize = 16.sp, lineHeight = 23.sp),
            titleLarge = TextStyle(fontSize = 24.sp, fontWeight = FontWeight.Bold),
            titleMedium = TextStyle(fontSize = 20.sp, fontWeight = FontWeight.SemiBold),
        ),
        content = content,
    )
}

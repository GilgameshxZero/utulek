package com.gilgamesh.smsgatec

import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.LocalRippleConfiguration
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp

val InkBlack = Color(0xFF000000)
val InkWhite = Color(0xFFFFFFFF)

data class EinkColors(
    val bg: Color,
    val fg: Color,
    val dim: Color,
    val inverseBg: Color,
    val inverseFg: Color,
)

val LocalEink = staticCompositionLocalOf {
    EinkColors(InkWhite, InkBlack, InkBlack.copy(alpha = 0.62f), InkBlack, InkWhite)
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SmsgatecTheme(dark: Boolean, content: @Composable () -> Unit) {
    val colors = if (dark) {
        EinkColors(InkBlack, InkWhite, InkWhite.copy(alpha = 0.62f), InkWhite, InkBlack)
    } else {
        EinkColors(InkWhite, InkBlack, InkBlack.copy(alpha = 0.62f), InkBlack, InkWhite)
    }
    val scheme = if (dark) {
        darkColorScheme(
            primary = InkWhite, onPrimary = InkBlack,
            secondary = InkWhite, onSecondary = InkBlack,
            background = InkBlack, onBackground = InkWhite,
            surface = InkBlack, onSurface = InkWhite,
            surfaceVariant = InkBlack, onSurfaceVariant = InkWhite,
            outline = InkWhite, outlineVariant = InkWhite,
            error = InkWhite, onError = InkBlack,
            surfaceTint = InkBlack,
        )
    } else {
        lightColorScheme(
            primary = InkBlack, onPrimary = InkWhite,
            secondary = InkBlack, onSecondary = InkWhite,
            background = InkWhite, onBackground = InkBlack,
            surface = InkWhite, onSurface = InkBlack,
            surfaceVariant = InkWhite, onSurfaceVariant = InkBlack,
            outline = InkBlack, outlineVariant = InkBlack,
            error = InkBlack, onError = InkWhite,
            surfaceTint = InkWhite,
        )
    }
    CompositionLocalProvider(LocalEink provides colors, LocalRippleConfiguration provides null) {
        MaterialTheme(
            colorScheme = scheme,
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

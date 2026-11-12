package com.gilgamesh.smsgatec

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectVerticalDragGestures
import androidx.compose.foundation.gestures.scrollBy
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch
import kotlin.math.abs

data class TopAction(
    val icon: Int,
    val description: String,
    val enabled: Boolean = true,
    val onClick: () -> Unit,
)

@Composable
fun EinkTopBar(
    title: String,
    onBack: (() -> Unit)? = null,
    actions: List<TopAction> = emptyList(),
    onTitleClick: (() -> Unit)? = null,
    titleMenu: (@Composable () -> Unit)? = null,
    trailing: (@Composable () -> Unit)? = null,
) {
    val palette = LocalEink.current
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .height(46.dp)
            .background(palette.bg)
            .padding(horizontal = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (onBack != null) {
            IconAction(iconRes = R.drawable.ic_back, description = "Back", onClick = onBack)
        } else {
            Spacer(Modifier.width(8.dp))
        }
        Box(Modifier.weight(1f)) {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .clickable(enabled = onTitleClick != null) { onTitleClick?.invoke() },
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    text = title,
                    color = palette.fg,
                    fontSize = 17.sp,
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f, fill = false),
                )
                if (onTitleClick != null) {
                    Icon(
                        painter = painterResource(R.drawable.ic_arrow_down),
                        contentDescription = "Switch account",
                        tint = palette.fg,
                        modifier = Modifier.size(18.dp),
                    )
                }
            }
            titleMenu?.invoke()
        }
        for (action in actions) {
            IconAction(
                iconRes = action.icon,
                description = action.description,
                onClick = action.onClick,
                enabled = action.enabled,
            )
        }
        trailing?.invoke()
    }
}

@Composable
fun IconAction(
    iconRes: Int,
    description: String,
    enabled: Boolean = true,
    onClick: () -> Unit,
) {
    val palette = LocalEink.current
    Box(
        modifier = Modifier
            .size(44.dp)
            .clickable(enabled = enabled) { onClick() },
        contentAlignment = Alignment.Center,
    ) {
        Icon(
            painter = painterResource(iconRes),
            contentDescription = description,
            tint = if (enabled) palette.fg else palette.dim,
            modifier = Modifier.size(22.dp),
        )
    }
}

@Composable
fun SearchTopBar(
    query: String,
    onQueryChange: (String) -> Unit,
    onClose: () -> Unit,
) {
    val palette = LocalEink.current
    val focusRequester = remember { FocusRequester() }
    val keyboard = LocalSoftwareKeyboardController.current
    LaunchedEffect(Unit) {
        focusRequester.requestFocus()
        keyboard?.show()
    }
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .height(46.dp)
            .background(palette.bg)
            .padding(horizontal = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        IconAction(iconRes = R.drawable.ic_close, description = "Close search", onClick = onClose)
        Box(Modifier.weight(1f)) {
            if (query.isEmpty()) {
                Text("Search", color = palette.dim, fontSize = 16.sp)
            }
            BasicTextField(
                value = query,
                onValueChange = onQueryChange,
                singleLine = true,
                textStyle = TextStyle(color = palette.fg, fontSize = 16.sp),
                cursorBrush = SolidColor(palette.fg),
                modifier = Modifier
                    .fillMaxWidth()
                    .focusRequester(focusRequester),
            )
        }
        Spacer(Modifier.width(8.dp))
    }
}

@Composable
fun EinkStatusBar(state: UiState) {
    val palette = LocalEink.current
    val text = when {
        state.status.isNotEmpty() -> state.status
        !state.hasAccounts -> "No account · open Settings"
        else -> {
            val connection = if (state.connected) "Connected" else "Disconnected"
            val synced = state.lastSync?.let { ago(it) } ?: "not synced"
            "$connection · $synced · ${state.messageCount} messages · ${state.contactCount} contacts"
        }
    }
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .height(20.dp)
            .background(palette.bg)
            .padding(horizontal = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            text = text,
            color = palette.dim,
            fontSize = 10.sp,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

fun ago(stamp: java.time.Instant): String {
    val seconds = java.time.Duration.between(stamp, java.time.Instant.now()).seconds
    return when {
        seconds < 60 -> "just now"
        seconds < 3600 -> "${seconds / 60}m ago"
        seconds < 86400 -> "${seconds / 3600}h ago"
        else -> "${seconds / 86400}d ago"
    }
}

/**
 * List surface. In paged mode a vertical swipe turns one page and the list
 * never flings; smooth mode scrolls normally. No page buttons are shown.
 */
@Composable
fun <T> EinkList(
    entries: List<T>,
    empty: String,
    paged: Boolean = true,
    scrollToEnd: Boolean = false,
    row: @Composable (T) -> Unit,
) {
    val palette = LocalEink.current
    val listState = rememberLazyListState()
    val scope = rememberCoroutineScope()
    val density = LocalDensity.current
    LaunchedEffect(entries.size) {
        if (scrollToEnd && entries.isNotEmpty()) {
            listState.scrollToItem(entries.lastIndex)
        }
    }
    if (entries.isEmpty()) {
        Box(Modifier.fillMaxSize().background(palette.bg).padding(16.dp)) {
            Text(empty, color = palette.dim, fontSize = 14.sp)
        }
    } else {
        BoxWithConstraints(Modifier.fillMaxSize().background(palette.bg)) {
            val pagePx = with(density) { maxHeight.toPx() }
            var drag by remember { mutableFloatStateOf(0f) }
            val swipe = if (paged) {
                Modifier.pointerInput(entries.size, pagePx) {
                    detectVerticalDragGestures(
                        onDragStart = { drag = 0f },
                        onDragCancel = { drag = 0f },
                        onDragEnd = {
                            val threshold = 36.dp.toPx()
                            if (abs(drag) > threshold) {
                                val delta = if (drag < 0) pagePx * 0.9f else -pagePx * 0.9f
                                scope.launch { listState.scrollBy(delta) }
                            }
                            drag = 0f
                        },
                    ) { change, amount ->
                        change.consume()
                        drag += amount
                    }
                }
            } else {
                Modifier
            }
            LazyColumn(
                state = listState,
                userScrollEnabled = !paged,
                modifier = Modifier.fillMaxSize().then(swipe),
            ) {
                items(entries) { item -> row(item) }
            }
        }
    }
}


/**
 * Form surface with the same scrolling behavior as lists: paged swipes when
 * enabled, otherwise ordinary smooth scrolling.
 */
@Composable
fun EinkScrollColumn(
    paged: Boolean,
    modifier: Modifier = Modifier,
    content: @Composable ColumnScope.() -> Unit,
) {
    val palette = LocalEink.current
    val scroll = rememberScrollState()
    val scope = rememberCoroutineScope()
    val density = LocalDensity.current
    BoxWithConstraints(modifier.fillMaxSize()) {
        val pagePx = with(density) { maxHeight.toPx() }
        var drag by remember { mutableFloatStateOf(0f) }
        val swipe = if (paged) {
            Modifier.pointerInput(pagePx, scroll.canScrollForward, scroll.canScrollBackward) {
                detectVerticalDragGestures(
                    onDragStart = { drag = 0f },
                    onDragCancel = { drag = 0f },
                    onDragEnd = {
                        val threshold = 36.dp.toPx()
                        if (abs(drag) > threshold) {
                            val delta = if (drag < 0) pagePx * 0.9f else -pagePx * 0.9f
                            scope.launch { scroll.scrollBy(delta) }
                        }
                        drag = 0f
                    },
                ) { change, amount ->
                    change.consume()
                    drag += amount
                }
            }
        } else {
            Modifier
        }
        Column(
            Modifier
                .fillMaxSize()
                .background(palette.bg)
                .then(swipe)
                .verticalScroll(scroll, enabled = !paged),
            content = content,
        )
    }
}

@Composable
fun SectionRow(
    title: String,
    value: String? = null,
    icon: Int? = null,
    danger: Boolean = false,
    onClick: (() -> Unit)? = null,
) {
    val palette = LocalEink.current
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(enabled = onClick != null) { onClick?.invoke() }
            .padding(horizontal = 12.dp, vertical = 11.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (icon != null) {
            Icon(
                painter = painterResource(icon),
                contentDescription = null,
                tint = palette.fg,
                modifier = Modifier.size(20.dp),
            )
            Spacer(Modifier.width(12.dp))
        }
        Text(
            text = title,
            color = palette.fg,
            fontSize = 16.sp,
            fontWeight = if (danger) FontWeight.SemiBold else FontWeight.Normal,
            modifier = Modifier.weight(1f),
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
        if (value != null) {
            Text(
                text = value,
                color = palette.dim,
                fontSize = 13.sp,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.padding(start = 12.dp),
            )
        }
    }
}

@Composable
fun FormField(
    label: String,
    value: String,
    onValueChange: (String) -> Unit,
    secret: Boolean = false,
    numeric: Boolean = false,
    singleLine: Boolean = true,
    minLines: Int = 1,
    placeholder: String = "",
    modifier: Modifier = Modifier,
) {
    val palette = LocalEink.current
    Column(
        modifier
            .fillMaxWidth()
            .padding(horizontal = 12.dp, vertical = 4.dp)
    ) {
        Text(label, color = palette.dim, fontSize = 11.sp)
        Box(
            Modifier
                .fillMaxWidth()
                .padding(vertical = 6.dp)
                .drawBehind {
                    drawLine(
                        palette.dim, Offset(0f, size.height), Offset(size.width, size.height),
                        strokeWidth = 1f,
                    )
                }
        ) {
            if (value.isEmpty() && placeholder.isNotEmpty()) {
                Text(placeholder, color = palette.dim, fontSize = 16.sp)
            }
            BasicTextField(
                value = value,
                onValueChange = onValueChange,
                singleLine = singleLine,
                minLines = minLines,
                textStyle = TextStyle(color = palette.fg, fontSize = 16.sp, lineHeight = 22.sp),
                cursorBrush = SolidColor(palette.fg),
                visualTransformation = if (secret) PasswordVisualTransformation() else VisualTransformation.None,
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

@Composable
fun FormError(message: String) {
    if (message.isEmpty()) return
    val palette = LocalEink.current
    Text(
        text = message,
        color = palette.fg,
        fontSize = 13.sp,
        fontWeight = FontWeight.SemiBold,
        modifier = Modifier.fillMaxWidth().padding(horizontal = 12.dp, vertical = 6.dp),
    )
}

@Composable
fun EmptyAction(text: String, action: String, onClick: () -> Unit) {
    val palette = LocalEink.current
    Column(Modifier.fillMaxSize().padding(24.dp)) {
        Text(text, color = palette.dim, fontSize = 14.sp)
        Spacer(Modifier.height(12.dp))
        Text(
            text = action,
            color = palette.fg,
            fontSize = 15.sp,
            fontWeight = FontWeight.SemiBold,
            modifier = Modifier
                .clickable { onClick() }
                .padding(vertical = 8.dp),
        )
    }
}

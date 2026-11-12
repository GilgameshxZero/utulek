package com.gilgamesh.maildirt

import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.TimeZone
import javax.mail.internet.InternetAddress

/** Pure text/address helpers shared by compose, drafts and replies. */
/** The comma-delimited recipient token around a caret position. */
data class AddressToken(val start: Int, val end: Int, val text: String)

object MailText {
    fun signatureBlock(signature: String): String =
        if (signature.isBlank()) "" else "\n\n--\n" + signature.trimEnd()

    /** Replace a trailing signature block, preserving the rest of the body. */
    fun replaceSignature(body: String, signature: String): String {
        val marker = "\n\n--\n"
        val index = body.lastIndexOf(marker)
        val base = if (index >= 0) body.substring(0, index) else body
        return base + signatureBlock(signature)
    }

    /** `Name <address>` for autocomplete, quoting the name when needed. */
    fun formatAddress(name: String, address: String): String {
        val clean = name.trim()
        if (clean.isEmpty()) return address
        val needsQuote = clean.any { it.code > 127 } || clean.any { it in ",<>\"@;:" }
        val personal = if (needsQuote) {
            "\"" + clean.replace("\\", "\\\\").replace("\"", "\\\"") + "\""
        } else {
            clean
        }
        return "$personal <$address>"
    }

    fun prefixedSubject(subject: String, prefix: String): String {
        val clean = subject.ifBlank { "(no subject)" }
        val lower = clean.lowercase()
        return if (lower.startsWith("re:") || lower.startsWith("fwd:") || lower.startsWith("fw:")) {
            clean
        } else {
            "$prefix $clean"
        }
    }

    fun quotedBody(message: ParsedMessage): String = "\n\n\n" + quoteFor(message)

    fun quoteFor(message: ParsedMessage): String {
        val text = message.text?.takeIf { it.isNotBlank() }
            ?: message.html?.let { stripHtml(it) }.orEmpty()
        val date = SimpleDateFormat("yyyy/MM/dd HH:mm", Locale.getDefault())
            .apply { timeZone = TimeZone.getDefault() }
            .format(Date(message.date))
        return "On $date, ${message.from} wrote:\n" +
            text.lines().joinToString("\n") { "> $it" }
    }

    fun referencesFor(message: ParsedMessage): String? {
        val id = message.messageId
        val existing = message.references
        return when {
            existing.isNullOrBlank() -> id
            id.isNullOrBlank() -> existing
            existing.contains(id) -> existing
            else -> "$existing $id"
        }
    }

    fun addressesOf(value: String): List<String> =
        runCatching {
            InternetAddress.parse(value.replace('\n', ','), false)
                .mapNotNull { it.address?.lowercase() }
        }.getOrDefault(emptyList())

    /** Comma-delimited token around [cursor], trimmed, with its replace range. */
    fun addressToken(text: String, cursor: Int): AddressToken {
        val caret = cursor.coerceIn(0, text.length)
        val leftComma = if (caret == 0) -1 else text.lastIndexOf(',', caret - 1)
        val left = leftComma + 1
        val rightComma = text.indexOf(',', caret)
        val right = if (rightComma < 0) text.length else rightComma
        val raw = text.substring(left, right)
        val first = raw.indexOfFirst { !it.isWhitespace() }
        if (first < 0) return AddressToken(right, right, "")
        val last = raw.indexOfLast { !it.isWhitespace() } + 1
        return AddressToken(left + first, left + last, raw.substring(first, last))
    }

    fun recipientLines(value: String): List<String> =
        value.split('\n').map { it.trim() }.filter { it.isNotEmpty() }

    fun stripHtml(html: String): String =
        html.replace(Regex("(?i)<br\\s*/?>"), "\n")
            .replace(Regex("(?i)</p>"), "\n")
            .replace(Regex("(?s)<[^>]+>"), "")
            .replace("&nbsp;", " ")
            .replace("&amp;", "&")
            .replace("&lt;", "<")
            .replace("&gt;", ">")
            .replace("&quot;", "\"")
            .trim()
}

package com.gilgamesh.maildirt

import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.TimeZone
import javax.mail.internet.InternetAddress

/** Pure text/address helpers shared by compose, drafts and replies. */
object MailText {
    fun buildOutgoingBody(body: String, signature: String): String {
        val builder = StringBuilder(body.trimEnd())
        if (signature.isNotBlank()) {
            if (builder.isNotEmpty()) builder.append('\n')
            builder.append('\n').append(signature.trimEnd())
        }
        return builder.toString()
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

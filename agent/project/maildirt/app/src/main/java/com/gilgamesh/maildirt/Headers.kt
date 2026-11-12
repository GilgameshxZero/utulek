package com.gilgamesh.maildirt

import java.io.File
import javax.mail.internet.InternetAddress
import javax.mail.internet.MailDateFormat
import javax.mail.internet.MimeUtility

data class RawHeaderInfo(
    val from: String,
    val fromAddress: String,
    val to: String,
    val cc: String,
    val subject: String,
    val date: Long,
    val hasAttachments: Boolean,
    val messageId: String?,
)

/** One shared header reader for maildir listing, address book, search and notifications. */
object HeaderParser {
    const val MAX_HEADER_BYTES = 128 * 1024

    fun read(file: File, maxBytes: Int = MAX_HEADER_BYTES): Map<String, String> =
        parseHeaders(readHeaderText(file, maxBytes))

    fun info(file: File): RawHeaderInfo {
        val headers = read(file)
        val fallback = file.lastModified()
        val contentType = headers["content-type"].orEmpty()
        return RawHeaderInfo(
            from = displayAddress(headers["from"].orEmpty()),
            fromAddress = decode(headers["from"].orEmpty()),
            to = decode(headers["to"].orEmpty()),
            cc = decode(headers["cc"].orEmpty()),
            subject = decode(headers["subject"].orEmpty()).ifBlank { "(no subject)" },
            date = parseDate(headers["date"].orEmpty(), fallback),
            hasAttachments = contentType.contains("multipart/mixed", ignoreCase = true),
            messageId = headers["message-id"]?.trim()?.takeIf { it.isNotBlank() },
        )
    }

    fun messageId(file: File): String? =
        read(file, 32 * 1024)["message-id"]?.trim()?.takeIf { it.isNotBlank() }

    /** Plain body text for indexing/notifications, with attachment encodings removed. */
    fun bodyText(file: File, maxBytes: Int): String {
        val text = readText(file, maxBytes)
        val crlf = text.indexOf("\r\n\r\n")
        val lf = text.indexOf("\n\n")
        val start = when {
            crlf >= 0 && lf >= 0 -> minOf(crlf, lf) + if (crlf < lf) 4 else 2
            crlf >= 0 -> crlf + 4
            lf >= 0 -> lf + 2
            else -> 0
        }
        return sanitizeBody(text.substring(start))
    }

    fun readText(file: File, maxBytes: Int): String {
        val length = minOf(file.length(), maxBytes.toLong()).toInt()
        if (length <= 0) return ""
        val bytes = ByteArray(length)
        file.inputStream().use { input ->
            var offset = 0
            while (offset < length) {
                val read = input.read(bytes, offset, length - offset)
                if (read <= 0) break
                offset += read
            }
        }
        return String(bytes, Charsets.ISO_8859_1)
    }

    fun displayAddress(value: String): String {
        if (value.isBlank()) return "(unknown sender)"
        return runCatching {
            val address = InternetAddress(value)
            address.personal?.takeIf { it.isNotBlank() } ?: address.address.orEmpty()
        }.getOrDefault(decode(value)).ifBlank { "(unknown sender)" }
    }

    fun decode(value: String): String =
        runCatching { MimeUtility.decodeText(value) }.getOrDefault(value)

    fun parseDate(value: String, fallback: Long): Long {
        if (value.isBlank()) return fallback
        return runCatching { MailDateFormat().parse(value)?.time ?: fallback }.getOrDefault(fallback)
    }

    /** Format an InternetAddress with a decoded display name. */
    fun format(address: InternetAddress): String {
        val personal = decode(address.personal.orEmpty())
        return if (personal.isNotBlank()) "$personal <${address.address}>" else address.address.orEmpty()
    }

    fun parseAddress(value: String): Pair<String, String>? = runCatching {
        InternetAddress.parseHeader(value, false)
            .asSequence()
            .filter { it.address?.contains('@') == true }
            .firstOrNull()
            ?.let { decode(it.personal.orEmpty().trim()) to it.address }
    }.getOrNull()

    private fun readHeaderText(file: File, maxBytes: Int): String = runCatching {
        val builder = StringBuilder()
        file.inputStream().bufferedReader(Charsets.ISO_8859_1).use { reader ->
            var total = 0
            while (total < maxBytes) {
                val line = reader.readLine() ?: break
                if (line.isEmpty()) break
                total += line.length + 1
                builder.append(line).append('\n')
            }
        }
        builder.toString()
    }.getOrDefault("")

    private fun parseHeaders(text: String): Map<String, String> {
        val headers = HashMap<String, String>()
        var lastKey: String? = null
        for (rawLine in text.split('\n')) {
            val line = rawLine.trimEnd('\r')
            if (line.isEmpty()) continue
            if ((line.startsWith(' ') || line.startsWith('\t')) && lastKey != null) {
                val key = lastKey ?: continue
                headers[key] = headers[key].orEmpty() + " " + line.trim()
                continue
            }
            val separator = line.indexOf(':')
            if (separator <= 0) continue
            val key = line.substring(0, separator).trim().lowercase()
            headers[key] = line.substring(separator + 1).trim()
            lastKey = key
        }
        return headers
    }

    private fun sanitizeBody(text: String): String {
        val builder = StringBuilder(text.length)
        for (line in text.split('\n')) {
            val trimmed = line.trim()
            if (trimmed.startsWith("--") && trimmed.length < 120) continue
            if (isBase64Line(trimmed)) continue
            if (trimmed.startsWith("Content-") || trimmed.startsWith("MIME-Version:")) continue
            builder.append(line).append(' ')
        }
        return builder.toString()
    }

    private fun isBase64Line(line: String): Boolean {
        if (line.length < 60) return false
        var allowed = 0
        for (char in line) {
            when {
                char.isLetterOrDigit() -> allowed++
                char == '+' || char == '/' || char == '=' -> allowed++
                char == ' ' || char == '\t' -> allowed++
                else -> return false
            }
        }
        return allowed.toDouble() / line.length > 0.95
    }
}

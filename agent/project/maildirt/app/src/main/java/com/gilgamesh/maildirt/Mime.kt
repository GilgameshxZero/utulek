package com.gilgamesh.maildirt

import android.content.Context
import android.net.Uri
import android.util.Base64
import java.io.ByteArrayInputStream
import java.io.File
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.Properties
import javax.mail.Address
import javax.mail.Message
import javax.mail.Multipart
import javax.mail.Part
import javax.mail.Session
import javax.mail.internet.InternetAddress
import javax.mail.internet.MimeMessage
import javax.mail.internet.MimeUtility

data class Attachment(
    val name: String,
    val mimeType: String,
    val size: Long,
    val file: File,
)

data class InlineImage(
    val mimeType: String,
    val bytes: ByteArray,
)

data class MimeAlternative(
    val mimeType: String,
    val content: String,
)

data class ParsedMessage(
    val subject: String,
    val from: String,
    val to: String,
    val cc: String,
    val date: Long,
    val html: String?,
    val text: String?,
    val alternatives: List<MimeAlternative>,
    val attachments: List<Attachment>,
    val inlineImages: Map<String, InlineImage>,
)

class MimeReader(private val context: Context) {

    fun parse(file: File, cacheKey: String): ParsedMessage {
        if (file.length() > MAX_MESSAGE_BYTES) {
            throw IllegalStateException("Message is too large to open (${file.length() / 1024 / 1024} MB)")
        }

        val cacheDir = File(context.cacheDir, "msg_$cacheKey")
        cacheDir.deleteRecursively()
        cacheDir.mkdirs()

        val bytes = file.inputStream().use { it.readBytes() }
        val session = Session.getInstance(Properties())
        val message = MimeMessage(session, ByteArrayInputStream(bytes))

        val state = ParseState()
        walk(message, state, cacheDir)

        return ParsedMessage(
            subject = decodeHeader(message.subject).ifBlank { "(no subject)" },
            from = formatAddresses(message.from),
            to = formatAddresses(message.getRecipients(Message.RecipientType.TO)),
            cc = formatAddresses(message.getRecipients(Message.RecipientType.CC)),
            date = runCatching { message.sentDate?.time }.getOrNull() ?: file.lastModified(),
            html = state.html,
            text = state.text,
            alternatives = state.alternatives.map { MimeAlternative(it.key, it.value) },
            attachments = state.attachments,
            inlineImages = state.inline,
        )
    }

    private class ParseState {
        var text: String? = null
        var html: String? = null
        val alternatives = LinkedHashMap<String, String>()
        val attachments = ArrayList<Attachment>()
        val inline = HashMap<String, InlineImage>()
        var inlineBytes = 0
    }

    private fun walk(part: Part, state: ParseState, cacheDir: File) {
        val fileName = runCatching { part.fileName }.getOrNull()
        val disposition = runCatching { part.disposition }.getOrNull()
        val contentId = runCatching {
            part.getHeader("Content-ID")?.firstOrNull()
        }.getOrNull()?.trim()?.trim('<', '>')

        if (part.isMimeType("multipart/*")) {
            val multipart = runCatching { part.content as? Multipart }.getOrNull()
            if (multipart != null) {
                for (index in 0 until multipart.count) {
                    runCatching { multipart.getBodyPart(index) }.getOrNull()?.let {
                        walk(it, state, cacheDir)
                    }
                }
            }
            return
        }

        if (part.isMimeType("message/rfc822")) {
            val inner = runCatching { part.content as? Part }.getOrNull()
            if (inner != null && fileName.isNullOrBlank()) {
                walk(inner, state, cacheDir)
                return
            }
        }

        val isAttachment = !fileName.isNullOrBlank() || Part.ATTACHMENT.equals(disposition, true)
        if (isAttachment) {
            saveAttachment(part, fileName, cacheDir, state)
            return
        }

        if (part.isMimeType("text/*")) {
            val mimeType = runCatching {
                part.contentType?.substringBefore(';')?.trim()
            }.getOrNull()?.takeIf { it.isNotBlank() }?.lowercase() ?: "text/plain"
            val content = runCatching { part.content as? String }.getOrNull()
            if (content != null) {
                state.alternatives.putIfAbsent(mimeType, content)
                when (mimeType) {
                    "text/html" -> if (state.html == null) state.html = content
                    "text/plain" -> if (state.text == null) state.text = content
                }
            }
            return
        }

        if (contentId != null) {
            addInline(part, contentId, state)
        }
    }

    private fun saveAttachment(part: Part, rawName: String?, cacheDir: File, state: ParseState) {
        val name = safeFileName(rawName, state.attachments.size)
        val target = File(cacheDir, name)
        val size = runCatching {
            part.inputStream.use { input ->
                target.outputStream().use { output -> input.copyTo(output) }
            }
        }.getOrElse {
            target.delete()
            return
        }
        val mimeType = runCatching { part.contentType?.substringBefore(';')?.trim() }
            .getOrNull()
            ?.takeIf { it.isNotBlank() }
            ?: "application/octet-stream"
        state.attachments.add(Attachment(name, mimeType, size, target))
    }

    private fun addInline(part: Part, contentId: String, state: ParseState) {
        if (state.inlineBytes > MAX_INLINE_TOTAL) return
        val data = runCatching { part.inputStream.use { it.readBytes() } }.getOrNull() ?: return
        if (data.size > MAX_INLINE_ONE) return
        val mimeType = runCatching { part.contentType?.substringBefore(';')?.trim() }
            .getOrNull()
            ?.takeIf { it.isNotBlank() }
            ?: "application/octet-stream"
        state.inline[contentId] = InlineImage(mimeType, data)
        state.inlineBytes += data.size
    }

    private fun safeFileName(rawName: String?, index: Int): String {
        val base = rawName?.trim()?.takeIf { it.isNotEmpty() } ?: "attachment-$index"
        val cleaned = base
            .replace(Regex("""[\\/:*?"<>|\u0000-\u001f]"""), "_")
            .trim()
            .take(120)
            .ifBlank { "attachment-$index" }
        return "${index}_$cleaned"
    }

    private fun formatAddresses(addresses: Array<Address>?): String =
        addresses?.joinToString(", ") { address ->
            if (address is InternetAddress) {
                val personal = address.personal
                if (!personal.isNullOrBlank()) "$personal <${address.address}>" else address.address.orEmpty()
            } else {
                address.toString()
            }
        }.orEmpty()

    private fun decodeHeader(value: String?): String {
        if (value == null) return ""
        return runCatching { MimeUtility.decodeText(value) }.getOrDefault(value)
    }

    companion object {
        private const val MAX_MESSAGE_BYTES = 50L * 1024 * 1024
        private const val MAX_INLINE_ONE = 8 * 1024 * 1024
        private const val MAX_INLINE_TOTAL = 24 * 1024 * 1024
    }
}

private val CID_PATTERN = Regex("""(?i)cid:([^"'\s)>]+)""")

fun renderMessageHtml(message: ParsedMessage, alternativeMime: String? = null): String {
    val selected = alternativeMime?.let { mime ->
        message.alternatives.firstOrNull { it.mimeType.equals(mime, ignoreCase = true) }
    }

    val body = when {
        selected != null && selected.mimeType.equals("text/html", ignoreCase = true) ->
            rewriteCids(sanitizeHtml(selected.content), message.inlineImages)
        selected != null -> "<pre>${escapeHtml(selected.content)}</pre>"
        !message.html.isNullOrBlank() ->
            rewriteCids(sanitizeHtml(message.html), message.inlineImages)
        !message.text.isNullOrBlank() -> "<pre>${escapeHtml(message.text)}</pre>"
        else -> "<p><i>(This message has no displayable body.)</i></p>"
    }

    val builder = StringBuilder()
    builder.append("<!doctype html><html><head><meta charset=\"utf-8\">")
    builder.append("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">")
    builder.append(
        """
        <style>
        html,body{margin:0;padding:12px;background:#fff;color:#000;width:100%;max-width:100%;overflow-x:hidden;}
        body{font-family:sans-serif;font-size:17px;line-height:1.45;word-wrap:break-word;overflow-wrap:anywhere;}
        *{box-sizing:border-box;max-width:100%!important;}
        img,iframe,video{max-width:100%!important;height:auto!important;}
        table{width:100%!important;table-layout:fixed;}
        td,th{word-break:break-word;}
        pre{white-space:pre-wrap;word-wrap:break-word;font-family:sans-serif;font-size:17px;}
        blockquote{margin:8px 0 8px 12px;padding-left:10px;border-left:2px solid #000;}
        a{color:#000;text-decoration:underline;}
        .header{border-bottom:1px solid #000;margin-bottom:12px;padding-bottom:10px;}
        .subject{font-size:21px;font-weight:bold;line-height:1.3;margin-bottom:8px;}
        .label{font-size:13px;font-weight:bold;text-transform:uppercase;margin-top:6px;}
        .value{font-size:16px;}
        </style>
        """.trimIndent()
    )
    builder.append("</head><body>")

    builder.append("<div class=\"header\">")
    builder.append("<div class=\"subject\">").append(escapeHtml(message.subject)).append("</div>")
    appendField(builder, "From", message.from)
    if (message.to.isNotBlank()) appendField(builder, "To", message.to)
    if (message.cc.isNotBlank()) appendField(builder, "Cc", message.cc)
    appendField(builder, "Date", formatDate(message.date))
    builder.append("</div>")

    builder.append(body)
    builder.append("</body></html>")
    return builder.toString()
}

private fun appendField(builder: StringBuilder, label: String, value: String) {
    builder.append("<div class=\"label\">").append(label).append("</div>")
    builder.append("<div class=\"value\">").append(escapeHtml(value)).append("</div>")
}

private fun rewriteCids(html: String, inline: Map<String, InlineImage>): String {
    if (inline.isEmpty() || !html.contains("cid:", ignoreCase = true)) return html
    return CID_PATTERN.replace(html) { match ->
        val raw = match.groupValues[1]
        val image = findInline(raw, inline)
        if (image == null) {
            match.value
        } else {
            val encoded = Base64.encodeToString(image.bytes, Base64.NO_WRAP)
            "data:${image.mimeType};base64,$encoded"
        }
    }
}

private fun findInline(key: String, inline: Map<String, InlineImage>): InlineImage? {
    val cleaned = key.trim().trim('<', '>')
    inline[cleaned]?.let { return it }
    val decoded = runCatching { Uri.decode(cleaned) }.getOrDefault(cleaned)
    inline[decoded]?.let { return it }
    return inline.entries.firstOrNull { (cid, _) ->
        cid.equals(cleaned, ignoreCase = true) ||
            cid.equals(decoded, ignoreCase = true) ||
            cid.substringBefore('@').equals(cleaned.substringBefore('@'), ignoreCase = true)
    }?.value
}

private fun sanitizeHtml(html: String): String {
    var result = html
    result = result.replace(Regex("""(?is)<script.*?</script>"""), "")
    result = result.replace(Regex("""(?is)<iframe.*?</iframe>"""), "")
    result = result.replace(Regex("""(?is)<object.*?</object>"""), "")
    result = result.replace(Regex("""(?is)<embed[^>]*>"""), "")
    result = result.replace(Regex("""(?i)\son[a-z]+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)"""), "")
    result = result.replace(Regex("""(?i)<meta[^>]+http-equiv\s*=\s*"?refresh[^>]*>"""), "")
    result = result.replace(Regex("(?i)href\\s*=\\s*\"javascript:[^\"]*\""), "href=\"#\"")
    result = result.replace(Regex("(?i)href\\s*=\\s*'javascript:[^']*'"), "href=\"#\"")
    return result
}

private fun escapeHtml(value: String): String =
    value.replace("&", "&amp;")
        .replace("<", "&lt;")
        .replace(">", "&gt;")
        .replace("\"", "&quot;")

private fun formatDate(timestamp: Long): String =
    SimpleDateFormat("EEE, d MMM yyyy HH:mm", Locale.getDefault()).format(Date(timestamp))

internal fun formatSize(bytes: Long): String = when {
    bytes >= 1024 * 1024 -> String.format(Locale.getDefault(), "%.1f MB", bytes / 1024.0 / 1024.0)
    bytes >= 1024 -> String.format(Locale.getDefault(), "%.1f KB", bytes / 1024.0)
    else -> "$bytes B"
}

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
import java.util.TimeZone
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
    val messageId: String? = null,
    val references: String? = null,
    val calendar: CalendarInvite? = null,
)

class MimeReader(private val context: Context) {

    fun parse(file: File, cacheKey: String): ParsedMessage {
        if (file.length() > MAX_MESSAGE_BYTES) {
            throw IllegalStateException("Message is too large to open (${file.length() / 1024 / 1024} MB)")
        }

        val cacheDir = File(context.cacheDir, "msg_$cacheKey")
        val stamp = "${file.lastModified()}_${file.length()}"
        val stampFile = File(cacheDir, ".stamp")
        val reuse = cacheDir.isDirectory &&
            runCatching { stampFile.readText() }.getOrNull() == stamp
        if (!reuse) {
            cacheDir.deleteRecursively()
            cacheDir.mkdirs()
            runCatching { stampFile.writeText(stamp) }
        }

        val bytes = file.inputStream().use { it.readBytes() }
        val session = Session.getInstance(Properties())
        val message = MimeMessage(session, ByteArrayInputStream(bytes))

        val state = ParseState()
        walk(message, state, cacheDir, reuse)

        val calendar = state.calendarTexts
            .asSequence()
            .mapNotNull { CalendarMessaging.parseInvite(it) }
            .firstOrNull()
            ?: state.attachments
                .filter { it.isCalendarFile() }
                .asSequence()
                .mapNotNull { attachment ->
                    runCatching { attachment.file.readText() }
                        .getOrNull()
                        ?.let { CalendarMessaging.parseInvite(it) }
                }
                .firstOrNull()

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
            messageId = message.getHeader("Message-ID")?.firstOrNull()?.trim(),
            references = message.getHeader("References")?.firstOrNull()?.trim(),
            calendar = calendar,
        )
    }

    private class ParseState {
        var text: String? = null
        var html: String? = null
        val alternatives = LinkedHashMap<String, String>()
        val attachments = ArrayList<Attachment>()
        val inline = HashMap<String, InlineImage>()
        val calendarTexts = ArrayList<String>()
        var inlineBytes = 0
    }

    private fun Attachment.isCalendarFile(): Boolean =
        mimeType.equals("text/calendar", ignoreCase = true) ||
            mimeType.equals("application/ics", ignoreCase = true) ||
            name.endsWith(".ics", ignoreCase = true)

    private fun walk(part: Part, state: ParseState, cacheDir: File, reuse: Boolean) {
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
                        walk(it, state, cacheDir, reuse)
                    }
                }
            }
            return
        }

        if (part.isMimeType("message/rfc822")) {
            val inner = runCatching { part.content as? Part }.getOrNull()
            if (inner != null && fileName.isNullOrBlank()) {
                walk(inner, state, cacheDir, reuse)
                return
            }
        }

        val isAttachment = !fileName.isNullOrBlank() || Part.ATTACHMENT.equals(disposition, true)
        if (isAttachment) {
            saveAttachment(part, fileName, cacheDir, state, reuse)
            return
        }

        if (part.isMimeType("text/*")) {
            val mimeType = runCatching {
                part.contentType?.substringBefore(';')?.trim()
            }.getOrNull()?.takeIf { it.isNotBlank() }?.lowercase() ?: "text/plain"
            val content = readTextPart(part)
            if (content != null) {
                if (mimeType == "text/calendar") state.calendarTexts.add(content)
                state.alternatives.putIfAbsent(mimeType, content)
                when (mimeType) {
                    "text/html" -> if (state.html == null) state.html = content
                    "text/plain" -> if (state.text == null) state.text = content
                }
            }
            return
        }

        if (part.isMimeType("application/ics")) {
            readTextPart(part)?.let { state.calendarTexts.add(it) }
            return
        }

        if (contentId != null) {
            addInline(part, contentId, state)
        }
    }

    private fun saveAttachment(
        part: Part,
        rawName: String?,
        cacheDir: File,
        state: ParseState,
        reuse: Boolean,
    ) {
        val displayName = safeName(rawName, state.attachments.size)
        val target = File(cacheDir, "${state.attachments.size}_$displayName")
        val mimeType = runCatching { part.contentType?.substringBefore(';')?.trim() }
            .getOrNull()
            ?.takeIf { it.isNotBlank() }
            ?: "application/octet-stream"
        if (reuse && target.isFile) {
            state.attachments.add(Attachment(displayName, mimeType, target.length(), target))
            return
        }
        val size = runCatching {
            part.inputStream.use { input ->
                target.outputStream().use { output -> input.copyTo(output) }
            }
        }.getOrElse {
            target.delete()
            return
        }
        state.attachments.add(Attachment(displayName, mimeType, size, target))
    }

    /** Unknown text subtypes come back as a stream, not a String. */
    private fun readTextPart(part: Part): String? {
        (runCatching { part.content as? String }.getOrNull())?.let { return it }
        return runCatching {
            val charset = Regex("(?i)charset=\"?([^;\"]+)")
                .find(part.contentType.orEmpty())?.groupValues?.get(1)?.trim()
                ?: "UTF-8"
            val bytes = part.inputStream.use { it.readBytes() }
            bytes.toString(java.nio.charset.Charset.forName(charset))
        }.onFailure { android.util.Log.w("MaildirtCal", "could not read text part", it) }.getOrNull()
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

    private fun safeName(rawName: String?, index: Int): String {
        val base = rawName?.trim()?.takeIf { it.isNotEmpty() } ?: "attachment-$index"
        return base
            .replace(Regex("""[\\/:*?"<>|\u0000-\u001f]"""), "_")
            .trim()
            .take(120)
            .ifBlank { "attachment-$index" }
    }

    private fun formatAddresses(addresses: Array<Address>?): String =
        addresses?.joinToString("\n") { address ->
            if (address is InternetAddress) HeaderParser.format(address) else address.toString()
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

private const val PAPERCLIP_SVG =
    "<svg class=\"summaryicon\" viewBox=\"0 0 24 24\" xmlns=\"http://www.w3.org/2000/svg\">" +
        "<path fill=\"#000000\" d=\"M16.5,6v11.5c0,2.21 -1.79,4 -4,4s-4,-1.79 -4,-4V5" +
        "c0,-1.38 1.12,-2.5 2.5,-2.5s2.5,1.12 2.5,2.5v10.5c0,0.55 -0.45,1 -1,1s-1,-0.45 " +
        "-1,-1V6H10v9.5c0,1.38 1.12,2.5 2.5,2.5s2.5,-1.12 2.5,-2.5V5c0,-2.21 -1.79,-4 " +
        "-4,-4S7,2.79 7,5v12.5c0,3.04 2.46,5.5 5.5,5.5s5.5,-2.46 5.5,-5.5V6h-1.5z\"/></svg>"

private const val SHARE_ICON_SVG =
    "<svg class=\"shareicon\" viewBox=\"0 0 24 24\" xmlns=\"http://www.w3.org/2000/svg\">" +
        "<path fill=\"#000000\" d=\"M18,16.08c-0.76,0 -1.44,0.3 -1.96,0.77L8.91,12.7" +
        "c0.05,-0.23 0.09,-0.46 0.09,-0.7s-0.04,-0.47 -0.09,-0.7l7.05,-4.11" +
        "c0.54,0.5 1.25,0.81 2.04,0.81 1.66,0 3,-1.34 3,-3s-1.34,-3 -3,-3" +
        "-3,1.34 -3,3c0,0.24 0.04,0.47 0.09,0.7L8.04,9.81C7.5,9.31 6.79,9 6,9" +
        "c-1.66,0 -3,1.34 -3,3s1.34,3 3,3c0.79,0 1.5,-0.31 2.04,-0.81l7.12,4.16" +
        "c-0.05,0.21 -0.08,0.43 -0.08,0.65 0,1.61 1.31,2.92 2.92,2.92s2.92,-1.31 " +
        "2.92,-2.92 -1.31,-2.92 -2.92,-2.92z\"/></svg>"

fun renderSourceHtml(source: String): String =
    "<!doctype html><html><head><meta charset=\"utf-8\">" +
        "<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">" +
        "<style>html,body{margin:0;padding:6px;background:#fff;color:#000;}" +
        "body{font-family:monospace;font-size:12px;line-height:1.3;}" +
        "pre{white-space:pre-wrap;word-wrap:break-word;}</style></head><body><pre>" +
        escapeHtml(source) + "</pre></body></html>"

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
        html,body{margin:0;padding:4px;background:#fff;color:#000;width:100%;max-width:100%;overflow-x:hidden;}
        body{font-family:sans-serif;font-size:18px;line-height:1.45;word-wrap:break-word;overflow-wrap:anywhere;filter:grayscale(1);}
        body *{margin-left:0!important;margin-right:0!important;padding-left:0!important;padding-right:0!important;}
        html,body{background:#fff!important;}
        body,div,p,span,li,td,th,tr,table,h1,h2,h3,h4,h5,h6,blockquote,pre,code{color:#000!important;background-color:transparent!important;}
        *{box-sizing:border-box;min-width:0!important;max-width:100%!important;}
        div,p,span,li,td,th{min-width:0!important;max-width:100%!important;white-space:normal!important;overflow-wrap:anywhere!important;word-break:break-word!important;}
        table{width:100%!important;max-width:100%!important;table-layout:fixed!important;border-collapse:collapse!important;border-spacing:0!important;}
        td,th{width:auto!important;}
        img,svg,video,iframe,canvas{width:auto!important;max-width:100%!important;height:auto!important;}
        pre,code{white-space:pre-wrap!important;word-break:break-word!important;max-width:100%!important;font-family:sans-serif;font-size:18px;}
        blockquote{margin:6px 0;padding-left:8px;font-style:italic;}
        a{color:#000;text-decoration:underline;}
        .header{margin:0 0 10px 0;padding:0;background:#fff!important;}
        .field{margin:6px 0;}
        .field .label{display:block;font-size:16px;font-weight:bold;text-transform:uppercase;letter-spacing:0.4px;line-height:1.2;}
        .value,.value-list,.subject-value{font-size:16px;line-height:1.35;}
        .attachments{margin:6px 0 10px 0;background:#fff!important;}
        .attach-summary{display:flex;align-items:center;gap:7px;font-size:16px;font-weight:bold;padding:2px 0 4px 0;}
        .summaryicon{width:18px!important;height:18px!important;max-width:18px!important;max-height:18px!important;display:block;}
        .attach{display:flex;align-items:center;gap:8px;font-size:16px;line-height:1.3;margin:3px 0;}
        .attach .name{flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap!important;}
        .attach .name a{color:#000;text-decoration:underline;}
        .attach .size{font-size:13px;white-space:nowrap!important;}
        .attach .share{flex:none;display:inline-flex;width:26px;height:26px;align-items:center;justify-content:center;}
        .shareicon{width:20px!important;height:20px!important;max-width:20px!important;max-height:20px!important;display:block;}
        </style>
        """.trimIndent()
    )
    builder.append("</head><body>")

    builder.append("<div class=\"header\">")
    appendField(builder, "Subject", message.subject)
    appendField(builder, "From", message.from)
    if (message.to.isNotBlank()) appendAddressField(builder, "To", message.to)
    if (message.cc.isNotBlank()) appendAddressField(builder, "Cc", message.cc)
    appendField(builder, "Date", formatDate(message.date))
    builder.append("</div>")

    if (message.attachments.isNotEmpty()) {
        val attachmentCount = message.attachments.size
        builder.append("<div class=\"attachments\">")
        builder.append("<div class=\"attach-summary\">")
            .append(PAPERCLIP_SVG)
            .append("<span>")
            .append(attachmentCount)
            .append(if (attachmentCount == 1) " attachment" else " attachments")
            .append("</span></div>")
        for ((index, attachment) in message.attachments.withIndex()) {
            builder.append("<div class=\"attach\">")
            builder.append("<span class=\"name\"><a href=\"maildirt://attachment/")
                .append(index).append("\">")
                .append(escapeHtml(attachment.name)).append("</a></span>")
            builder.append("<span class=\"size\">").append(formatSize(attachment.size)).append("</span>")
            builder.append("<a class=\"share\" href=\"maildirt://share/")
                .append(index).append("\" title=\"Share\">")
                .append(SHARE_ICON_SVG).append("</a>")
            builder.append("</div>")
        }
        builder.append("</div>")
    }

    builder.append(stripDocumentWrapper(body))
    builder.append("</body></html>")
    return builder.toString()
}

private fun appendField(builder: StringBuilder, label: String, value: String) {
    builder.append("<div class=\"field\"><span class=\"label\">").append(label).append("</span>")
    builder.append("<div class=\"value\">")
        .append(escapeHtml(value).replace("\n", "<br>"))
        .append("</div></div>")
}

private fun appendAddressField(builder: StringBuilder, label: String, value: String) {
    builder.append("<div class=\"field\"><span class=\"label\">").append(label).append("</span>")
    builder.append("<div class=\"value-list\">")
        .append(escapeHtml(value).replace("\n", "<br>"))
        .append("</div></div>")
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

private fun stripDocumentWrapper(html: String): String =
    html
        .replace(Regex("""(?is)<!doctype[^>]*>"""), "")
        .replace(Regex("""(?is)</?html[^>]*>"""), "")
        .replace(Regex("""(?is)</?head[^>]*>"""), "")
        .replace(Regex("""(?is)<body[^>]*>"""), "")
        .replace(Regex("""(?is)</body>"""), "")

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
    SimpleDateFormat("yyyy/MM/dd HH:mm", Locale.getDefault())
        .apply { timeZone = TimeZone.getDefault() }
        .format(Date(timestamp))

internal fun formatSize(bytes: Long): String = when {
    bytes >= 1024 * 1024 -> String.format(Locale.getDefault(), "%.1f MB", bytes / 1024.0 / 1024.0)
    bytes >= 1024 -> String.format(Locale.getDefault(), "%.1f KB", bytes / 1024.0)
    else -> "$bytes B"
}

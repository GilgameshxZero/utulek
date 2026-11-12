package com.gilgamesh.smsgatec

import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import okhttp3.Credentials
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response
import java.io.IOException
import java.time.Instant
import java.time.format.DateTimeFormatter
import java.util.Base64
import java.util.concurrent.TimeUnit

class ApiError(
    message: String,
    val status: Int = 0,
    val uncertain: Boolean = false,
) : Exception(message)

@Serializable
private data class SendTextRequest(
    val id: String,
    val textMessage: ApiText,
    val phoneNumbers: List<String>,
    val withDeliveryReport: Boolean = true,
)

@Serializable
private data class SendMmsRequest(
    val id: String,
    val phoneNumbers: List<String>,
    val mmsMessage: ApiMms,
)

private val JSON_MEDIA = "application/json; charset=utf-8".toMediaType()
private val WHOLE_SECONDS = DateTimeFormatter.ofPattern("yyyy-MM-dd'T'HH:mm:ss'Z'").withZone(java.time.ZoneOffset.UTC)

class Gateway(server: String, private val username: String, private val password: String, timeoutSeconds: Int) {
    val base: String = server.trimEnd('/')
    private val client: OkHttpClient = OkHttpClient.Builder()
        .connectTimeout(timeoutSeconds.toLong(), TimeUnit.SECONDS)
        .readTimeout(timeoutSeconds.toLong(), TimeUnit.SECONDS)
        .writeTimeout(timeoutSeconds.toLong(), TimeUnit.SECONDS)
        .followRedirects(false)
        .followSslRedirects(false)
        .build()

    private fun request(method: String, path: String, body: String? = null): Response {
        val url = (base + path).toHttpUrlOrNull()
            ?: throw ApiError("Invalid gateway address")
        val builder = Request.Builder()
            .url(url)
            .header("Authorization", Credentials.basic(username, password))
        if (body != null) {
            builder.method(method, body.toRequestBody(JSON_MEDIA))
        } else {
            builder.method(method, null)
        }
        val response = try {
            client.newCall(builder.build()).execute()
        } catch (e: IOException) {
            throw ApiError("Gateway unreachable; check the phone, network, and address", uncertain = method == "POST")
        }
        if (!response.isSuccessful) {
            response.close()
            throw ApiError(
                "Gateway HTTP ${response.code}",
                response.code,
                method == "POST" && (response.code >= 500 || response.code == 409),
            )
        }
        return response
    }

    private fun <T> requestJson(method: String, path: String, body: String?, decode: (String) -> T): Pair<Response, T> {
        request(method, path, body).use { response ->
            val text = response.body?.string().orEmpty()
            if (text.length > 32 * 1024 * 1024) throw ApiError("Gateway response is too large")
            val value = try {
                decode(text)
            } catch (e: Exception) {
                throw ApiError("Invalid gateway response", uncertain = method == "POST")
            }
            return response to value
        }
    }

    fun health(): Health = requestJson("GET", "/health", null) {
        json.decodeFromString<Health>(it)
    }.second

    private fun listing(path: String): List<ApiMessage> {
        val result = mutableListOf<ApiMessage>()
        var offset = 0
        while (offset < 100000) {
            val separator = if (path.contains('?')) "&" else "?"
            val pagePath = "$path${separator}limit=500&offset=$offset"
            var total = -1
            val rows = requestJson("GET", pagePath, null) { text ->
                json.decodeFromString<List<ApiMessage>>(text)
            }.let { (response, value) ->
                total = response.header("X-Total-Count")?.toIntOrNull() ?: -1
                value
            }
            result.addAll(rows)
            offset += rows.size
            if (rows.size < 500 || (total >= 0 && offset >= total)) break
        }
        if (offset >= 100000) throw ApiError("History exceeds 100,000 records")
        return result
    }

    fun message(id: String): ApiMessage = requestJson("GET", "/message/" + urlEncode(id), null) {
        json.decodeFromString<ApiMessage>(it)
    }.second.also {
        if (it.id.isEmpty()) throw ApiError("Missing message ID")
    }

    fun syncSince(known: Map<String, Message>, since: Instant?): List<Message> {
        var inboxPath = "/inbox?includeAttachments=true"
        var outboxPath = "/messages"
        if (since != null) {
            val filter = "from=" + urlEncode(
                WHOLE_SECONDS.format(since.minusSeconds(300))
            )
            inboxPath += "&$filter"
            outboxPath += "?$filter"
        }
        if (known.isEmpty()) {
            outboxPath += if (outboxPath.contains('?')) "&includeContent=true" else "?includeContent=true"
        }
        val inbox = listing(inboxPath)
        val outbox = listing(outboxPath)
        val result = mutableListOf<Message>()
        for (row in inbox) result.addAll(row.messages())
        val seen = mutableSetOf<String>()
        for (row in outbox) {
            if (row.id.isEmpty()) continue
            seen.add(row.id)
            var current = row
            if (row.textMessage == null && row.mmsMessage == null) {
                val cached = known[row.id]
                current = if (cached != null && cached.text != "[Text unavailable]") {
                    row.copy(textMessage = ApiText(cached.text))
                } else {
                    message(row.id)
                }
            }
            for (m in current.messages()) {
                val old = known[row.id]
                result.add(if (old != null) mergeMessage(old, m) else m)
            }
        }
        val now = Instant.now()
        for ((id, old) in known) {
            if (id in seen) continue
            val recentNonMms = old.state == "Sent" && !old.type.startsWith("MMS") &&
                old.stamp()?.let { it.isAfter(now.minusSeconds(48 * 3600)) } == true
            val check = old.state == "Uncertain" || (since != null &&
                (old.state == "Pending" || old.state == "Processed" || recentNonMms))
            if (!check) continue
            val row = try {
                message(id)
            } catch (e: ApiError) {
                if (e.status == 404) continue
                throw e
            }
            val current = if (row.textMessage == null && row.mmsMessage == null) {
                row.copy(textMessage = ApiText(old.text))
            } else {
                row
            }
            for (m in current.messages()) result.add(mergeMessage(old, m))
        }
        return result
    }

    data class SendOutcome(val state: String, val groupId: String = "")

    /** Sends one SMS, MMS, or a multi-recipient group. */
    fun send(numbers: List<String>, id: String, text: String, subject: String, parts: List<WireAttachment>): SendOutcome {
        val groupOrMms = numbers.size > 1 || parts.isNotEmpty() || subject.isNotBlank()
        val body = if (groupOrMms) {
            json.encodeToString(
                SendMmsRequest(id, numbers, ApiMms(subject = subject, text = text, attachments = parts))
            )
        } else {
            json.encodeToString(SendTextRequest(id, ApiText(text), numbers))
        }
        val row = requestJson("POST", "/message", body) { json.decodeFromString<ApiMessage>(it) }.second
        if (row.id.isEmpty()) throw ApiError("Send outcome unknown", uncertain = true)
        val state = row.state.ifEmpty {
            row.recipients.firstOrNull { it.state.isNotEmpty() }?.state ?: "Pending"
        }
        return SendOutcome(state, row.groupId)
    }

    fun download(messageId: String, part: Attachment): ByteArray {
        request("GET", "/inbox/${urlEncode(messageId)}/attachments/${part.partId}").use { response ->
            val data = response.body?.bytes() ?: throw ApiError("Attachment download failed")
            if (data.size > 64 * 1024 * 1024) throw ApiError("Attachment exceeds 64 MiB")
            return data
        }
    }

    /**
     * SSE stream used by authenticated bridges (Google Voice). Reconnects with
     * backoff and never falls back to polling while the app is in front.
     */
    suspend fun streamEvents(retrySeconds: Int, onEvent: (PushEvent) -> Unit) {
        val streamClient = client.newBuilder()
            .readTimeout(0, TimeUnit.SECONDS)
            .build()
        var delay = retrySeconds.coerceAtLeast(5).toLong()
        var lastError: String? = null
        while (true) {
            try {
                val url = (base + "/events").toHttpUrlOrNull() ?: throw ApiError("Invalid gateway address")
                val request = Request.Builder()
                    .url(url)
                    .header("Authorization", Credentials.basic(username, password))
                    .header("Accept", "text/event-stream")
                    .build()
                streamClient.newCall(request).execute().use { response ->
                    if (!response.isSuccessful ||
                        !response.header("Content-Type").orEmpty().startsWith("text/event-stream")
                    ) {
                        throw ApiError(lastError ?: "Event stream unavailable")
                    }
                    delay = retrySeconds.coerceAtLeast(5).toLong()
                    lastError = null
                    val source = response.body?.source() ?: throw ApiError("Event stream closed")
                    while (true) {
                        val line = source.readUtf8Line() ?: break
                        if (!line.startsWith("data: ")) continue
                        val event = try {
                            json.decodeFromString<PushEvent>(line.removePrefix("data: "))
                        } catch (e: Exception) {
                            continue
                        }
                        onEvent(event)
                    }
                }
            } catch (e: kotlinx.coroutines.CancellationException) {
                throw e
            } catch (e: Exception) {
                lastError = e.message
            }
            kotlinx.coroutines.delay(delay * 1000)
            delay = (delay * 2).coerceAtMost(300)
        }
    }

    fun close() {
        client.dispatcher.executorService.shutdown()
        client.connectionPool.evictAll()
    }
}

private fun urlEncode(value: String): String =
    java.net.URLEncoder.encode(value, "UTF-8").replace("%7E", "~")

fun PushEvent.incoming(): Message? {
    val p = payload
    if (p.sender.isEmpty() || p.messageId.isEmpty()) return null
    try {
        Instant.parse(p.receivedAt)
    } catch (e: Exception) {
        return null
    }
    val base = Message(
        number = phoneKey(p.sender), direction = "in",
        timestamp = p.receivedAt, state = "Received",
    )
    return when (event) {
        "sms:received" -> {
            val hash = p.messageId.toLongOrNull(16) ?: return null
            base.copy(
                id = "text:" + hash.toInt().toString(),
                type = "SMS", text = p.message,
            )
        }
        "mms:received" -> {
            if (p.transactionId.isEmpty()) return null
            base.copy(
                id = "mms-header:" + javaHash(p.transactionId),
                type = "MMS", text = p.subject.ifEmpty { "MMS notification" },
                subject = p.subject,
            )
        }
        "mms:downloaded" -> base.copy(
            id = "mms:" + javaHash(p.messageId),
            type = "MMS_DOWNLOADED",
            text = p.body,
            subject = p.subject,
            media = p.attachments.map {
                Attachment(partId = it.partId, name = it.name, contentType = it.contentType, size = it.size)
            },
        )
        else -> null
    }
}

fun javaHash(value: String): Int {
    var h = 0
    for (c in value) h = 31 * h + c.code
    return h
}

fun decodeWireAttachment(part: PushPart): ByteArray? = try {
    Base64.getDecoder().decode(part.data)
} catch (e: Exception) {
    null
}

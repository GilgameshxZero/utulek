package com.gilgamesh.maildirt

import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.TimeZone

enum class CalendarResponseState(
    val partStat: String,
    val label: String,
    val subjectPrefix: String,
) {
    ACCEPTED("ACCEPTED", "Accept", "Accepted"),
    TENTATIVE("TENTATIVE", "Tentative", "Tentative"),
    DECLINED("DECLINED", "Decline", "Declined"),
}

data class CalendarAttendee(
    val email: String,
    val name: String,
    val partStat: String,
)

data class CalendarInvite(
    val uid: String,
    val summary: String,
    val description: String,
    val location: String,
    val organizerEmail: String,
    val organizerName: String,
    val attendees: List<CalendarAttendee>,
    val start: String,
    val end: String?,
    val duration: String?,
    val sequence: Int,
    val rawProperties: Map<String, String>,
    val timezoneBlocks: List<List<String>>,
) {
    val attendeeEmails: List<String>
        get() = attendees.map { it.email.lowercase() }

    val canRespond: Boolean
        get() = uid.isNotBlank() &&
            organizerEmail.isNotBlank() &&
            rawProperties.containsKey("DTSTART")
}

/**
 * Reader/writer for the calendar-message conventions used by neomutt_xp's
 * mutt-ical helper: RFC 5545 reply with a single ATTENDEE and the invitation's
 * UID, ORGANIZER, SEQUENCE, RECURRENCE-ID and time properties preserved.
 */
object CalendarMessaging {
    private const val PRODID = "-//PYVOBJECT//NONSGML Version 0.9.9//EN"
    private const val MAX_BYTES = 512 * 1024

    fun parseInvite(text: String): CalendarInvite? {
        if (text.isBlank() || text.length > MAX_BYTES) return null
        val root = runCatching { IcalReader.parse(text) }.onFailure {
            android.util.Log.w("MaildirtCal", "calendar parse failed", it)
        }.getOrNull() ?: return null
        if (!root.name.equals("VCALENDAR", ignoreCase = true)) return null
        val method = root.value("METHOD").orEmpty().uppercase()
        if (method.isNotBlank() && method != "REQUEST") return null
        val event = root.children.firstOrNull { it.name.equals("VEVENT", ignoreCase = true) }
            ?: return null
        if (event.value("UID").isNullOrBlank()) return null
        val organizer = event.property("ORGANIZER") ?: return null
        val organizerEmail = mailtoAddress(organizer)
        if (organizerEmail.isBlank()) return null

        val startProperty = event.property("DTSTART") ?: return null
        val endProperty = event.property("DTEND")
        val durationProperty = event.property("DURATION")
        val attendees = event.properties("ATTENDEE").mapNotNull { property ->
            val email = mailtoAddress(property)
            if (email.isBlank()) null else CalendarAttendee(
                email = email,
                name = property.params["CN"].orEmpty(),
                partStat = property.params["PARTSTAT"].orEmpty(),
            )
        }

        val raw = LinkedHashMap<String, String>()
        for (name in listOf("UID", "SUMMARY", "DTSTART", "DTEND", "DURATION", "ORGANIZER", "SEQUENCE", "RECURRENCE-ID")) {
            event.property(name)?.let { raw[name] = it.raw }
        }

        val timezoneBlocks = root.children
            .filter { it.name.equals("VTIMEZONE", ignoreCase = true) }
            .map { it.rawLines }

        return CalendarInvite(
            uid = event.text("UID"),
            summary = event.text("SUMMARY").ifBlank { "(no subject)" },
            description = event.text("DESCRIPTION"),
            location = event.text("LOCATION"),
            organizerEmail = organizerEmail,
            organizerName = organizer.params["CN"].orEmpty(),
            attendees = attendees,
            start = describeDate(startProperty),
            end = endProperty?.let { describeDate(it) },
            duration = durationProperty?.value?.let { describeDuration(it) },
            sequence = event.value("SEQUENCE")?.trim()?.toIntOrNull() ?: 0,
            rawProperties = raw,
            timezoneBlocks = timezoneBlocks,
        )
    }

    fun buildReply(
        invite: CalendarInvite,
        identityEmail: String,
        state: CalendarResponseState,
    ): String {
        val lines = ArrayList<String>()
        lines += "BEGIN:VCALENDAR"
        lines += "VERSION:2.0"
        lines += "PRODID:$PRODID"
        lines += "METHOD:REPLY"
        lines += "BEGIN:VEVENT"
        for (name in listOf("UID", "SUMMARY", "DTSTART", "DTEND", "DURATION", "ORGANIZER", "SEQUENCE", "RECURRENCE-ID")) {
            invite.rawProperties[name]?.let { lines += it }
        }
        lines += "DTSTAMP:${utcStamp()}"
        lines += "ATTENDEE;CN=$identityEmail;PARTSTAT=${state.partStat}:mailto:$identityEmail"
        lines += "PRIORITY:5"
        when (state) {
            CalendarResponseState.ACCEPTED -> {
                lines += "STATUS:CONFIRMED"
                lines += "X-MICROSOFT-CDO-BUSYSTATUS:BUSY"
            }
            CalendarResponseState.TENTATIVE -> {
                lines += "STATUS:TENTATIVE"
                lines += "X-MICROSOFT-CDO-BUSYSTATUS:TENTATIVE"
            }
            CalendarResponseState.DECLINED -> Unit
        }
        lines += "END:VEVENT"
        invite.timezoneBlocks.forEach { block -> lines += block }
        lines += "END:VCALENDAR"

        val out = StringBuilder()
        for (line in lines) {
            if (out.isNotEmpty()) out.append("\r\n")
            out.append(fold(line))
        }
        out.append("\r\n")
        return out.toString()
    }

    fun subjectFor(invite: CalendarInvite, state: CalendarResponseState): String =
        "${state.subjectPrefix}: ${invite.summary}"

    private fun utcStamp(): String =
        SimpleDateFormat("yyyyMMdd'T'HHmmss'Z'", Locale.US)
            .apply { timeZone = TimeZone.getTimeZone("UTC") }
            .format(Date())

    private fun mailtoAddress(property: IcalProperty): String {
        val raw = property.value.trim()
        val value = if (raw.contains(':')) raw.substringAfter(':') else raw
        return value.trim()
    }

    private fun describeDate(property: IcalProperty): String {
        val value = property.value.trim()
        val isDate = property.params["VALUE"].equals("DATE", ignoreCase = true) || value.length == 8
        if (isDate) {
            val parsed = runCatching { dateFormat("yyyyMMdd").parse(value) }.getOrNull()
            return parsed?.let { dateFormat("yyyy-MM-dd").format(it) + " (all day)" } ?: value
        }
        val utc = value.endsWith("Z")
        val parsed = runCatching {
            dateFormat("yyyyMMdd'T'HHmmss").parse(value.removeSuffix("Z"))
        }.getOrNull() ?: return value
        val text = dateFormat("yyyy-MM-dd HH:mm").format(parsed)
        return when {
            utc -> "$text UTC"
            property.params["TZID"] != null -> "$text (${property.params["TZID"]})"
            else -> text
        }
    }

    private fun describeDuration(value: String): String? {
        val match = Regex("""^([+-])?P(?:(\d+)W)?(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$""")
            .find(value.trim().uppercase()) ?: return null
        val sign = match.groupValues[1]
        val weeks = match.groupValues[2]
        val days = match.groupValues[3]
        val hours = match.groupValues[4]
        val minutes = match.groupValues[5]
        val parts = ArrayList<String>()
        weeks.toIntOrNull()?.let { parts += "${it * 7} d" }
        days.toIntOrNull()?.let { parts += "$it d" }
        hours.toIntOrNull()?.let { parts += "$it h" }
        minutes.toIntOrNull()?.let { parts += "$it min" }
        if (parts.isEmpty()) return null
        return (if (sign == "-") "-" else "") + parts.joinToString(" ")
    }

    private fun dateFormat(pattern: String): SimpleDateFormat =
        SimpleDateFormat(pattern, Locale.US).apply { timeZone = TimeZone.getTimeZone("UTC") }

    private fun fold(line: String): String {
        val out = StringBuilder()
        var bytes = 0
        for (char in line) {
            val size = char.toString().toByteArray(Charsets.UTF_8).size
            if (bytes + size > 75) {
                out.append("\r\n ")
                bytes = 1
            }
            out.append(char)
            bytes += size
        }
        return out.toString()
    }
}

private class IcalProperty(
    val name: String,
    val params: Map<String, String>,
    val value: String,
    val raw: String,
)

private class IcalComponent(val name: String) {
    val properties = ArrayList<IcalProperty>()
    val children = ArrayList<IcalComponent>()
    val rawLines = ArrayList<String>()

    fun property(name: String): IcalProperty? =
        properties.firstOrNull { it.name.equals(name, ignoreCase = true) }

    fun properties(name: String): List<IcalProperty> =
        properties.filter { it.name.equals(name, ignoreCase = true) }

    fun value(name: String): String? = property(name)?.value

    fun text(name: String): String = unescape(value(name).orEmpty())
}

private object IcalReader {
    fun parse(text: String): IcalComponent {
        val lines = unfold(text)
        val stack = ArrayList<IcalComponent>()
        var root: IcalComponent? = null
        for (line in lines) {
            if (line.isBlank()) continue
            val colon = firstUnquoted(line, ':')
            if (colon < 0) continue
            val header = line.substring(0, colon)
            val value = line.substring(colon + 1)
            val fields = splitUnquoted(header, ';')
            val name = fields.first().trim().uppercase()
            if (name == "BEGIN") {
                val component = IcalComponent(value.trim())
                if (stack.isNotEmpty()) {
                    stack.last().children += component
                } else if (root == null) {
                    root = component
                }
                stack += component
                stack.forEach { it.rawLines += line }
            } else if (name == "END") {
                if (stack.isNotEmpty()) {
                    stack.forEach { it.rawLines += line }
                    stack.removeAt(stack.lastIndex)
                }
            } else if (stack.isNotEmpty()) {
                val params = HashMap<String, String>()
                for (field in fields.drop(1)) {
                    val eq = firstUnquoted(field, '=')
                    if (eq < 0) continue
                    val key = field.substring(0, eq).trim().uppercase()
                    val paramValue = field.substring(eq + 1).trim().removeSurrounding("\"")
                    params[key] = paramValue
                }
                stack.last().properties += IcalProperty(name, params, value, line)
                stack.forEach { it.rawLines += line }
            }
        }
        return root ?: throw IllegalArgumentException("Incomplete iCalendar")
    }

    private fun unfold(text: String): List<String> {
        val result = ArrayList<String>()
        for (line in text.replace("\r\n", "\n").replace('\r', '\n').split('\n')) {
            if (line.startsWith(" ") || line.startsWith("\t")) {
                if (result.isNotEmpty()) result[result.lastIndex] += line.substring(1)
            } else {
                result += line
            }
        }
        return result
    }

    private fun splitUnquoted(value: String, separator: Char): List<String> {
        val result = ArrayList<String>()
        var start = 0
        var quoted = false
        for (index in value.indices) {
            val char = value[index]
            if (char == '"') quoted = !quoted
            if (char == separator && !quoted) {
                result += value.substring(start, index)
                start = index + 1
            }
        }
        result += value.substring(start)
        return result
    }

    private fun firstUnquoted(value: String, target: Char): Int {
        var quoted = false
        for (index in value.indices) {
            val char = value[index]
            if (char == '"') quoted = !quoted
            if (char == target && !quoted) return index
        }
        return -1
    }
}

private fun unescape(value: String): String {
    val out = StringBuilder()
    var index = 0
    while (index < value.length) {
        val char = value[index]
        if (char == '\\' && index + 1 < value.length) {
            when (val next = value[index + 1]) {
                'n', 'N' -> out.append('\n')
                ',', ';', '\\' -> out.append(next)
                else -> {
                    out.append('\\')
                    out.append(next)
                }
            }
            index += 2
        } else {
            out.append(char)
            index++
        }
    }
    return out.toString()
}

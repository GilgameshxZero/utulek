package com.gilgamesh.maildirt

import android.content.Context
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.io.File
import java.io.FileOutputStream
import java.nio.file.Files
import java.nio.file.StandardCopyOption
import java.util.UUID
import javax.mail.internet.InternetAddress
import javax.mail.internet.MailDateFormat
import javax.mail.internet.MimeUtility

data class MaildirEntry(
    val relativePath: String,
    val flags: String,
    val separator: Char?,
    val unread: Boolean,
    val replied: Boolean,
    val flagged: Boolean,
    val hasAttachments: Boolean,
    val from: String,
    val subject: String,
    val date: Long,
    val size: Long,
)

@Serializable
data class CachedHeader(
    val mtime: Long,
    val size: Long,
    val from: String,
    val subject: String,
    val date: Long,
    val hasAttachments: Boolean,
)

@Serializable
data class FolderIndex(
    val files: Map<String, CachedHeader> = emptyMap(),
)

class FolderIndexStore(context: Context) {
    private val dir = File(context.filesDir, "index").apply { mkdirs() }
    private val json = Json { ignoreUnknownKeys = true }

    fun load(folderPath: String): FolderIndex = runCatching {
        val file = fileFor(folderPath)
        if (file.exists()) json.decodeFromString(FolderIndex.serializer(), file.readText())
        else FolderIndex()
    }.getOrDefault(FolderIndex())

    fun save(folderPath: String, index: FolderIndex) {
        runCatching {
            val file = fileFor(folderPath)
            val tmp = File(file.parentFile, "${file.name}.tmp")
            tmp.writeText(json.encodeToString(FolderIndex.serializer(), index))
            if (!tmp.renameTo(file)) {
                file.delete()
                tmp.renameTo(file)
            }
        }
    }

    private fun fileFor(path: String): File {
        val hash = Integer.toHexString(path.hashCode())
        return File(dir, "idx_${hash}_${path.length}.json")
    }
}

class MaildirFolderStore(
    folder: MaildirFolder,
    private val indexStore: FolderIndexStore? = null,
) {
    val root: File = runCatching { File(folder.path).canonicalFile }
        .getOrDefault(File(folder.path))
    val role: FolderRole = folder.role
    private var preferredSeparatorCache: Char? = null

    fun isValid(): Boolean =
        File(root, "cur").isDirectory &&
            File(root, "new").isDirectory &&
            File(root, "tmp").isDirectory

    fun fileFor(relativePath: String): File = File(root, relativePath)

    fun list(): List<MaildirEntry> {
        val index = indexStore?.load(root.path) ?: FolderIndex()
        val updated = HashMap<String, CachedHeader>()
        val entries = ArrayList<MaildirEntry>()

        for (sub in listOf("cur", "new")) {
            val dir = File(root, sub)
            if (!dir.isDirectory) continue
            val files = dir.listFiles()
                ?: throw IllegalStateException("Cannot list ${dir.absolutePath}")
            for (file in files) {
                if (!file.isFile || file.name.startsWith(".")) continue
                val relativePath = "$sub/${file.name}"
                val mtime = file.lastModified()
                val size = file.length()
                val cached = index.files[relativePath]
                val header = if (cached != null && cached.mtime == mtime && cached.size == size) {
                    RawHeader(cached.from, cached.subject, cached.date, cached.hasAttachments)
                } else {
                    HeaderReader.read(file)
                }
                updated[relativePath] = CachedHeader(
                    mtime = mtime,
                    size = size,
                    from = header.from,
                    subject = header.subject,
                    date = header.date,
                    hasAttachments = header.hasAttachments,
                )
                val info = parseInfo(file.name)
                entries.add(
                    MaildirEntry(
                        relativePath = relativePath,
                        flags = info.flags,
                        separator = info.separator,
                        unread = !info.flags.contains('S'),
                        replied = info.flags.contains('R'),
                        flagged = info.flags.contains('F'),
                        hasAttachments = header.hasAttachments,
                        from = header.from,
                        subject = header.subject,
                        date = header.date,
                        size = size,
                    ),
                )
            }
        }

        if (indexStore != null && updated != index.files) {
            indexStore.save(root.path, FolderIndex(updated))
        }
        entries.sortByDescending { it.date }
        return entries
    }

    fun setSeen(entry: MaildirEntry, seen: Boolean): MaildirEntry? {
        if (seen && !entry.unread) return entry
        if (!seen && entry.unread) return entry

        val source = fileFor(entry.relativePath)
        if (!source.exists()) return null
        val info = parseInfo(source.name)
        val separator = info.separator ?: preferredSeparator()
        val flags = if (seen) addFlag(info.flags, 'S') else removeFlag(info.flags, 'S')

        if (!seen && source.parentFile?.name == "new") {
            return entry
        }

        val targetDirName = if (seen) "cur" else (source.parentFile?.name ?: "cur")
        val name = if (info.separator == null && !seen) {
            source.name
        } else {
            "${info.base}$separator" + "2," + flags
        }
        val unique = uniqueName(targetDirName, name)
        val target = File(File(root, targetDirName), unique)

        if (target.absolutePath != source.absolutePath && !moveFile(source, target)) {
            return null
        }
        return entry.copy(
            relativePath = "$targetDirName/${target.name}",
            flags = flags,
            separator = separator,
            unread = !flags.contains('S'),
            replied = flags.contains('R'),
            flagged = flags.contains('F'),
        )
    }

    fun moveTo(
        targetStore: MaildirFolderStore,
        entry: MaildirEntry,
        addSeen: Boolean,
    ): Boolean {
        val source = fileFor(entry.relativePath)
        if (!source.exists()) return false
        val info = parseInfo(source.name)
        val separator = info.separator ?: targetStore.preferredSeparator()
        val flags = if (addSeen) addFlag(info.flags, 'S') else info.flags

        val keepUnreadInNew = !flags.contains('S') && source.parentFile?.name == "new"
        val targetDirName = if (keepUnreadInNew) "new" else "cur"
        val name = if (targetDirName == "new" && flags.isEmpty()) {
            info.base
        } else {
            "${info.base}$separator" + "2," + flags
        }
        val unique = targetStore.uniqueName(targetDirName, name)
        val target = File(File(targetStore.root, targetDirName), unique)
        return targetStore.moveFile(source, target)
    }

    fun delete(entry: MaildirEntry): Boolean = fileFor(entry.relativePath).delete()

    fun preferredSeparator(): Char {
        preferredSeparatorCache?.let { return it }
        var colon = 0
        var semicolon = 0
        for (sub in listOf("cur", "new")) {
            File(root, sub).listFiles()?.forEach { file ->
                if (file.name.contains(":2,")) colon++
                if (file.name.contains(";2,")) semicolon++
            }
        }
        val separator = if (semicolon > colon) ';' else ':'
        preferredSeparatorCache = separator
        return separator
    }

    private fun uniqueName(dirName: String, name: String): String {
        if (!File(File(root, dirName), name).exists()) return name
        val info = parseInfo(name)
        val suffix = ".${UUID.randomUUID().toString().take(8)}"
        return if (info.separator == null) {
            name + suffix
        } else {
            "${info.base}$suffix${info.separator}" + "2," + info.flags
        }
    }

    private fun moveFile(source: File, target: File): Boolean {
        target.parentFile?.mkdirs()
        val atomic = runCatching {
            Files.move(
                source.toPath(),
                target.toPath(),
                StandardCopyOption.ATOMIC_MOVE,
            )
            true
        }.getOrDefault(false)
        if (atomic) return true

        return runCatching {
            FileOutputStream(target).use { output ->
                source.inputStream().use { input -> input.copyTo(output) }
                output.fd.sync()
            }
            source.delete()
        }.getOrElse {
            target.delete()
            false
        }
    }

    private fun addFlag(flags: String, flag: Char): String =
        (flags + flag).toCharArray().distinct().sorted().joinToString("")

    private fun removeFlag(flags: String, flag: Char): String =
        flags.filter { it != flag }.toCharArray().distinct().sorted().joinToString("")

    private data class Info(
        val base: String,
        val separator: Char?,
        val flags: String,
    )

    private fun parseInfo(name: String): Info {
        val colon = name.lastIndexOf(":2,")
        val semicolon = name.lastIndexOf(";2,")
        val index = maxOf(colon, semicolon)
        if (index < 0) return Info(name, null, "")
        val separator = name[index]
        return Info(
            base = name.substring(0, index),
            separator = separator,
            flags = name.substring(index + 3),
        )
    }
}

private data class RawHeader(
    val from: String,
    val subject: String,
    val date: Long,
    val hasAttachments: Boolean,
)

private object HeaderReader {
    private const val MAX_HEADER_BYTES = 128 * 1024

    fun read(file: File): RawHeader {
        val headerText = runCatching { readHeaderText(file) }.getOrDefault("")
        val headers = parseHeaders(headerText)
        val fallback = file.lastModified()

        val from = displayAddress(headers["from"].orEmpty())
        val subject = decode(headers["subject"].orEmpty()).ifBlank { "(no subject)" }
        val date = parseDate(headers["date"].orEmpty(), fallback)
        val contentType = headers["content-type"].orEmpty()
        val hasAttachments = contentType.contains("multipart/mixed", ignoreCase = true)

        return RawHeader(from, subject, date, hasAttachments)
    }

    private fun readHeaderText(file: File): String {
        val length = minOf(file.length(), MAX_HEADER_BYTES.toLong()).toInt()
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
        val text = String(bytes, Charsets.ISO_8859_1)
        val crlf = text.indexOf("\r\n\r\n")
        val lf = text.indexOf("\n\n")
        val end = when {
            crlf >= 0 && lf >= 0 -> minOf(crlf, lf)
            crlf >= 0 -> crlf
            lf >= 0 -> lf
            else -> text.length
        }
        return text.substring(0, end)
    }

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
            val value = line.substring(separator + 1).trim()
            headers[key] = value
            lastKey = key
        }
        return headers
    }

    private fun decode(value: String): String =
        runCatching { MimeUtility.decodeText(value) }.getOrDefault(value)

    private fun displayAddress(value: String): String {
        if (value.isBlank()) return "(unknown sender)"
        return runCatching {
            val address = InternetAddress(value)
            address.personal?.takeIf { it.isNotBlank() } ?: address.address.orEmpty()
        }.getOrDefault(decode(value)).ifBlank { "(unknown sender)" }
    }

    private fun parseDate(value: String, fallback: Long): Long {
        if (value.isBlank()) return fallback
        return runCatching { MailDateFormat().parse(value)?.time ?: fallback }.getOrDefault(fallback)
    }
}

package com.gilgamesh.maildirt

import android.content.Context
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.io.File
import java.io.FileOutputStream
import java.nio.file.Files
import java.nio.file.StandardCopyOption
import java.util.UUID

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
    val to: String = "",
    val cc: String = "",
    val fromAddress: String = "",
)

@Serializable
data class FolderIndex(
    val files: Map<String, CachedHeader> = emptyMap(),
)

class FolderIndexStore(context: Context) {
    private val dir = File(context.filesDir, "index").apply { mkdirs() }
    private val json = Json { ignoreUnknownKeys = true }

    fun load(folderPath: String): FolderIndex = synchronized(LOCK) {
        runCatching {
            val file = fileFor(folderPath)
            if (file.exists()) json.decodeFromString(FolderIndex.serializer(), file.readText())
            else FolderIndex()
        }.getOrDefault(FolderIndex())
    }

    fun save(folderPath: String, index: FolderIndex) {
        synchronized(LOCK) {
            runCatching {
                val file = fileFor(folderPath)
                val tmp = File(file.parentFile, "${file.name}.${UUID.randomUUID().toString().take(8)}.tmp")
                tmp.writeText(json.encodeToString(FolderIndex.serializer(), index))
                runCatching {
                    Files.move(
                        tmp.toPath(),
                        file.toPath(),
                        StandardCopyOption.REPLACE_EXISTING,
                        StandardCopyOption.ATOMIC_MOVE,
                    )
                }.onFailure {
                    runCatching {
                        Files.move(
                            tmp.toPath(),
                            file.toPath(),
                            StandardCopyOption.REPLACE_EXISTING,
                        )
                    }
                }
                if (tmp.exists()) tmp.delete()
            }
        }
    }

    private fun fileFor(path: String): File {
        val hash = Integer.toHexString(path.hashCode())
        return File(dir, "idx_${hash}_${path.length}.json")
    }

    private companion object {
        val LOCK = Any()
    }
}

class MaildirFolderStore(
    folder: MaildirFolder,
    private val indexStore: FolderIndexStore? = null,
) {
    val root: File = runCatching { File(folder.path).canonicalFile }
        .getOrDefault(File(folder.path))
    val role: FolderRole = folder.role

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
                val info = if (cached != null && cached.mtime == mtime && cached.size == size) {
                    RawHeaderInfo(
                        from = cached.from,
                        fromAddress = cached.fromAddress,
                        to = cached.to,
                        cc = cached.cc,
                        subject = cached.subject,
                        date = cached.date,
                        hasAttachments = cached.hasAttachments,
                        messageId = null,
                    )
                } else {
                    HeaderParser.info(file)
                }
                updated[relativePath] = CachedHeader(
                    mtime = mtime,
                    size = size,
                    from = info.from,
                    subject = info.subject,
                    date = info.date,
                    hasAttachments = info.hasAttachments,
                    to = info.to,
                    cc = info.cc,
                    fromAddress = info.fromAddress,
                )
                val fileInfo = parseInfo(file.name)
                entries.add(
                    MaildirEntry(
                        relativePath = relativePath,
                        flags = fileInfo.flags,
                        separator = fileInfo.separator,
                        unread = !fileInfo.flags.contains('S'),
                        replied = fileInfo.flags.contains('R'),
                        flagged = fileInfo.flags.contains('F'),
                        hasAttachments = info.hasAttachments,
                        from = info.from,
                        subject = info.subject,
                        date = info.date,
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
        val separator = preferredSeparator()
        val flags = if (seen) withFlag(info.flags, 'S') else removeFlag(info.flags, 'S')

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
        val separator = targetStore.preferredSeparator()
        val flags = if (addSeen) withFlag(info.flags, 'S') else info.flags

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

    fun deletePath(relativePath: String): Boolean = fileFor(relativePath).delete()

    fun entry(relativePath: String): MaildirEntry? = findEntry(relativePath)

    fun addFlag(relativePath: String, flag: Char): Boolean {
        val entry = findEntry(relativePath) ?: return false
        return addFlag(entry, flag) != null
    }

    private fun findEntry(relativePath: String): MaildirEntry? {
        val file = fileFor(relativePath)
        if (!file.exists()) return null
        val info = parseInfo(file.name)
        return MaildirEntry(
            relativePath = relativePath,
            flags = info.flags,
            separator = info.separator,
            unread = !info.flags.contains('S'),
            replied = info.flags.contains('R'),
            flagged = info.flags.contains('F'),
            hasAttachments = false,
            from = "",
            subject = "",
            date = file.lastModified(),
            size = file.length(),
        )
    }

    fun append(bytes: ByteArray, flags: String = "S"): String? {
        val tmpDir = File(root, "tmp").apply { mkdirs() }
        val name = "${System.currentTimeMillis()}.${UUID.randomUUID().toString().take(8)}"
        val tmp = File(tmpDir, name)
        runCatching {
            FileOutputStream(tmp).use { output ->
                output.write(bytes)
                output.fd.sync()
            }
        }.onFailure { tmp.delete(); return null }

        val target = File(File(root, "cur"), "$name${preferredSeparator()}2,$flags")
        if (!moveFile(tmp, target)) {
            tmp.delete()
            return null
        }
        return "cur/${target.name}"
    }

    fun addFlag(entry: MaildirEntry, flag: Char): MaildirEntry? {
        val source = fileFor(entry.relativePath)
        if (!source.exists()) return null
        val info = parseInfo(source.name)
        if (info.flags.contains(flag)) return entry
        val separator = preferredSeparator()
        val flags = withFlag(info.flags, flag)
        val targetDirName = source.parentFile?.name ?: "cur"
        val target = File(File(root, targetDirName), "${info.base}$separator" + "2," + flags)
        if (target.absolutePath != source.absolutePath && !moveFile(source, target)) return null
        return entry.copy(
            relativePath = "$targetDirName/${target.name}",
            flags = flags,
            separator = separator,
            replied = flags.contains('R'),
        )
    }

    fun preferredSeparator(): Char = ';'

    private fun uniqueName(dirName: String, name: String): String {
        if (!File(File(root, dirName), name).exists()) return name
        val info = parseInfo(name)
        val suffix = ".${UUID.randomUUID().toString().take(8)}"
        return if (info.separator == null) {
            name + suffix
        } else {
            "${info.base}$suffix${preferredSeparator()}2,${info.flags}"
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

    private fun withFlag(flags: String, flag: Char): String =
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


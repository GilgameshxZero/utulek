package com.gilgamesh.maildirt

import android.content.Context
import android.content.pm.PackageManager
import android.provider.ContactsContract
import java.io.File
import javax.mail.internet.InternetAddress
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json

@Serializable
data class AddressEntry(
    val name: String = "",
    val address: String,
    val count: Int = 1,
)

@Serializable
private data class AddressCache(
    val signature: String = "",
    val entries: List<AddressEntry> = emptyList(),
)

class AddressBook(context: Context) {
    private val appContext = context.applicationContext
    private val file = File(context.filesDir, "addresses.json")
    private val indexStore = FolderIndexStore(context)
    private val json = Json { ignoreUnknownKeys = true }

    @Volatile
    private var entries: List<AddressEntry> = emptyList()

    fun suggestions(query: String, limit: Int = 8): List<AddressEntry> {
        val needle = query.substringAfterLast(',').trim().lowercase()
        if (needle.isEmpty()) return emptyList()
        val matches = entries.asSequence()
            .filter { entry ->
                entry.address.lowercase().contains(needle) ||
                    entry.name.lowercase().contains(needle)
            }
            .take(limit)
            .toList()
        val only = matches.singleOrNull()
        if (only != null && only.address.equals(needle, ignoreCase = true)) return emptyList()
        return matches
    }

    /** Rebuilds the cache when maildirs or system contacts changed; IO thread. */
    fun refresh(folders: List<MaildirFolder>) {
        val contacts = systemContacts()
        val cache = loadCache()
        val signature = signature(folders, contacts)
        if (cache != null && signature == cache.signature) {
            entries = cache.entries
            return
        }

        val counts = HashMap<String, AddressEntry>()
        for (contact in contacts) {
            counts[contact.address.lowercase()] = contact
        }
        for (folder in folders) {
            val index = indexStore.load(folder.path)
            val root = File(folder.path)
            for (sub in listOf("cur", "new")) {
                val dir = File(root, sub)
                if (!dir.isDirectory) continue
                dir.listFiles()?.forEach { message ->
                    if (!message.isFile || message.name.startsWith(".")) return@forEach
                    val cached = index.files["$sub/${message.name}"]
                    if (cached != null &&
                        (cached.to.isNotBlank() || cached.cc.isNotBlank() || cached.fromAddress.isNotBlank())
                    ) {
                        addAll(counts, cached.fromAddress)
                        addAll(counts, cached.to)
                        addAll(counts, cached.cc)
                    } else {
                        val headers = HeaderParser.read(message)
                        addAll(counts, headers["from"].orEmpty())
                        addAll(counts, headers["to"].orEmpty())
                        addAll(counts, headers["cc"].orEmpty())
                    }
                }
            }
        }

        val result = counts.values.sortedWith(
            compareByDescending<AddressEntry> { it.count }.thenBy { it.address },
        )
        entries = result
        runCatching {
            val tmp = File(file.parentFile, "${file.name}.tmp")
            tmp.writeText(json.encodeToString(AddressCache.serializer(), AddressCache(signature, result)))
            if (!tmp.renameTo(file)) {
                file.delete()
                tmp.renameTo(file)
            }
        }
    }

    private fun addAll(counts: HashMap<String, AddressEntry>, value: String) {
        if (value.isBlank()) return
        val parsed = runCatching { InternetAddress.parseHeader(value, false) }.getOrNull()
            ?: return
        for (address in parsed) {
            val mailbox = address.address ?: continue
            if (!mailbox.contains('@')) continue
            val key = mailbox.lowercase()
            val name = HeaderParser.decode(address.personal.orEmpty().trim())
            val existing = counts[key]
            counts[key] = when {
                existing == null -> AddressEntry(name, mailbox, 1)
                else -> existing.copy(
                    name = existing.name.ifBlank { name },
                    count = existing.count + 1,
                )
            }
        }
    }

    private fun systemContacts(): List<AddressEntry> {
        if (appContext.checkSelfPermission(android.Manifest.permission.READ_CONTACTS) !=
            PackageManager.PERMISSION_GRANTED
        ) {
            return emptyList()
        }
        val result = LinkedHashMap<String, AddressEntry>()
        runCatching {
            appContext.contentResolver.query(
                ContactsContract.CommonDataKinds.Email.CONTENT_URI,
                arrayOf(
                    ContactsContract.CommonDataKinds.Email.ADDRESS,
                    ContactsContract.CommonDataKinds.Email.DISPLAY_NAME,
                ),
                null,
                null,
                null,
            )?.use { cursor ->
                val addressIndex = cursor.getColumnIndexOrThrow(
                    ContactsContract.CommonDataKinds.Email.ADDRESS,
                )
                val nameIndex = cursor.getColumnIndexOrThrow(
                    ContactsContract.CommonDataKinds.Email.DISPLAY_NAME,
                )
                while (cursor.moveToNext()) {
                    val address = cursor.getString(addressIndex)?.trim().orEmpty()
                    if (!address.contains('@')) continue
                    val name = cursor.getString(nameIndex)?.trim().orEmpty()
                    val key = address.lowercase()
                    val existing = result[key]
                    if (existing == null) {
                        result[key] = AddressEntry(name, address, CONTACT_WEIGHT)
                    } else if (existing.name.isBlank()) {
                        result[key] = existing.copy(name = name)
                    }
                }
            }
        }
        return result.values.toList()
    }

    private fun loadCache(): AddressCache? = runCatching {
        if (file.exists()) json.decodeFromString(AddressCache.serializer(), file.readText())
        else null
    }.getOrNull()

    private fun signature(folders: List<MaildirFolder>, contacts: List<AddressEntry>): String {
        val mail = folders.sortedBy { it.path }.joinToString("|") { folder ->
            val root = File(folder.path)
            val cur = File(root, "cur")
            val new = File(root, "new")
            "${folder.path}:${cur.lastModified()}:${new.lastModified()}:${cur.list()?.size ?: 0}:${new.list()?.size ?: 0}"
        }
        val contactHash = contacts.joinToString(";") { "${it.address.lowercase()}=${it.name}" }.hashCode()
        return "$mail|contacts:$contactHash"
    }

    private companion object {
        const val CONTACT_WEIGHT = 1_000_000
    }
}

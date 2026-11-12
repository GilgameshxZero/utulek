package com.gilgamesh.smsgatec

import android.content.ContentValues
import android.content.Context
import android.database.sqlite.SQLiteDatabase
import android.database.sqlite.SQLiteOpenHelper
import java.io.File
import java.security.MessageDigest

private class StoreDb(context: Context, name: String) : SQLiteOpenHelper(context, name, null, 1) {
    override fun onCreate(db: SQLiteDatabase) {
        db.execSQL("CREATE TABLE IF NOT EXISTS messages (key TEXT PRIMARY KEY, payload TEXT NOT NULL)")
    }

    override fun onUpgrade(db: SQLiteDatabase, oldVersion: Int, newVersion: Int) {}
}

/**
 * Message history cache. Same record shape and merge rules as the terminal
 * client; unchanged records skip the database write.
 */
class MessageStore(context: Context, private val cacheEnabled: Boolean, name: String = "messages.db") {
    private val records = LinkedHashMap<String, Message>()
    private var db: StoreDb? = null
    val mediaDir: File = File(context.filesDir, "media").apply { mkdirs() }
    var revision: Long = 0
        private set

    init {
        if (cacheEnabled) {
            val helper = StoreDb(context, name)
            db = helper
            try {
                helper.readableDatabase.query("messages", arrayOf("payload"), null, null, null, null, null).use { cursor ->
                    while (cursor.moveToNext()) {
                        try {
                            val message = json.decodeFromString<Message>(cursor.getString(0))
                            records[message.key] = message
                        } catch (e: Exception) {
                            // Ignore a damaged row; a refresh will restore it.
                        }
                    }
                }
            } catch (e: Exception) {
                // A broken cache must not prevent the app from starting.
            }
        }
    }

    @Synchronized
    fun all(): List<Message> = records.values.toList()

    @Synchronized
    fun get(key: String): Message? = records[key]

    @Synchronized
    fun size(): Int = records.size

    @Synchronized
    fun put(messages: List<Message>): Int {
        val changed = mutableListOf<Message>()
        for (raw in messages) {
            val old = records[raw.key]
            val merged = if (old != null) mergeMessage(old, raw) else raw
            if (old != merged) changed.add(merged)
        }
        if (changed.isEmpty()) return 0
        val database = db?.writableDatabase
        if (database != null) {
            database.beginTransaction()
            try {
                for (m in changed) {
                    val values = ContentValues().apply {
                        put("key", m.key)
                        put("payload", json.encodeToString(Message.serializer(), m))
                    }
                    database.insertWithOnConflict(
                        "messages", null, values, SQLiteDatabase.CONFLICT_REPLACE,
                    )
                }
                database.setTransactionSuccessful()
            } finally {
                database.endTransaction()
            }
        }
        for (m in changed) records[m.key] = m
        revision++
        return changed.size
    }

    @Synchronized
    fun delete(key: String) {
        if (records.remove(key) == null) return
        db?.writableDatabase?.delete("messages", "key = ?", arrayOf(key))
        revision++
    }

    /** Keep only the newest N messages when a gateway history is very large. */
    @Synchronized
    fun trim(max: Int = 20000) {
        if (records.size <= max) return
        val sorted = records.values.sortedByDescending { it.stamp() }
        val keep = sorted.take(max).map { it.key }.toHashSet()
        val removed = records.keys.filter { it !in keep }
        if (removed.isEmpty()) return
        db?.writableDatabase?.let { database ->
            database.beginTransaction()
            try {
                for (key in removed) database.delete("messages", "key = ?", arrayOf(key))
                database.setTransactionSuccessful()
            } finally {
                database.endTransaction()
            }
        }
        for (key in removed) records.remove(key)
        revision++
    }

    @Synchronized
    fun close() {
        db?.close()
        db = null
    }

    fun storeMedia(part: Attachment, data: ByteArray): String {
        val digest = MessageDigest.getInstance("SHA-256").digest(data)
        val hash = digest.joinToString("") { "%02x".format(it) }
        val name = hash + "-" + safeName(part.name, part.contentType)
        val file = File(mediaDir, name)
        if (!file.exists()) {
            val temp = File(mediaDir, name + ".tmp")
            temp.writeBytes(data)
            if (!temp.renameTo(file)) {
                file.writeBytes(data)
                temp.delete()
            }
        }
        return file.absolutePath
    }
}

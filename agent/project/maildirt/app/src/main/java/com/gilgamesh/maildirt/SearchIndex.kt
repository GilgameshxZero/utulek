package com.gilgamesh.maildirt

import android.content.Context
import android.database.sqlite.SQLiteDatabase
import android.database.sqlite.SQLiteOpenHelper
import java.io.File

/**
 * Persistent full-text index over maildir headers and bodies, backed by SQLite
 * FTS4. One row per message; synced incrementally against file size.
 */
class SearchIndex(context: Context) {
    private val lastHashes = HashMap<String, Int>()
    private val helper = object : SQLiteOpenHelper(context, "search.db", null, 1) {
        override fun onCreate(db: SQLiteDatabase) {
            db.execSQL(
                "CREATE TABLE meta(" +
                    "folder TEXT NOT NULL, path TEXT NOT NULL, size INTEGER NOT NULL, " +
                    "PRIMARY KEY(folder, path))",
            )
            db.execSQL(
                "CREATE VIRTUAL TABLE fts USING fts4(" +
                    "folder, path, sender, subject, body, tokenize=unicode61)",
            )
        }

        override fun onUpgrade(db: SQLiteDatabase, oldVersion: Int, newVersion: Int) {
            db.execSQL("DROP TABLE IF EXISTS fts")
            db.execSQL("DROP TABLE IF EXISTS meta")
            onCreate(db)
        }
    }

    fun sync(folder: MaildirFolder, entries: List<MaildirEntry>) {
        var signature = 1
        for (entry in entries) {
            signature = 31 * signature + entry.relativePath.hashCode()
            signature = 31 * signature + entry.size.hashCode()
        }
        synchronized(LOCK) {
            if (lastHashes[folder.path] == signature) return
            lastHashes[folder.path] = signature
        }
        synchronized(LOCK) {
            val db = helper.writableDatabase
            db.beginTransaction()
            try {
                val known = HashMap<String, Long>()
                db.rawQuery(
                    "SELECT path, size FROM meta WHERE folder = ?",
                    arrayOf(folder.path),
                ).use { cursor ->
                    while (cursor.moveToNext()) {
                        known[cursor.getString(0)] = cursor.getLong(1)
                    }
                }

                for (entry in entries) {
                    val previousSize = known.remove(entry.relativePath)
                    if (previousSize != null && previousSize == entry.size) continue
                    val body = HeaderParser.bodyText(
                        File(folder.path, entry.relativePath),
                        MAX_BODY_BYTES,
                    )
                    db.execSQL(
                        "DELETE FROM fts WHERE folder = ? AND path = ?",
                        arrayOf(folder.path, entry.relativePath),
                    )
                    db.execSQL(
                        "INSERT INTO fts(folder, path, sender, subject, body) VALUES(?,?,?,?,?)",
                        arrayOf(folder.path, entry.relativePath, entry.from, entry.subject, body),
                    )
                    db.execSQL(
                        "INSERT OR REPLACE INTO meta(folder, path, size) VALUES(?,?,?)",
                        arrayOf(folder.path, entry.relativePath, entry.size),
                    )
                }

                for (stale in known.keys) {
                    db.execSQL(
                        "DELETE FROM fts WHERE folder = ? AND path = ?",
                        arrayOf(folder.path, stale),
                    )
                    db.execSQL(
                        "DELETE FROM meta WHERE folder = ? AND path = ?",
                        arrayOf(folder.path, stale),
                    )
                }
                db.setTransactionSuccessful()
            } finally {
                db.endTransaction()
            }
        }
    }

    fun search(folder: MaildirFolder, query: String, includeBody: Boolean): Set<String> {
        val terms = query.trim()
            .split(Regex("\\s+"))
            .map { term -> term.replace(Regex("[^\\p{L}\\p{N}@._-]"), "") }
            .filter { it.isNotEmpty() }
        if (terms.isEmpty()) return emptySet()

        synchronized(LOCK) {
            val db = helper.readableDatabase
            var result: MutableSet<String>? = null
            for (term in terms) {
                val matches = HashSet<String>()
                if (includeBody) {
                    queryPaths(db, folder.path, "$term*", matches)
                } else {
                    queryPaths(db, folder.path, "sender:$term*", matches)
                    queryPaths(db, folder.path, "subject:$term*", matches)
                }
                result = if (result == null) matches else (result ?: matches).also { it.retainAll(matches) }
                if (result.orEmpty().isEmpty()) break
            }
            return result.orEmpty()
        }
    }

    fun pruneFolders(configured: Set<String>) {
        synchronized(LOCK) {
            val db = helper.writableDatabase
            if (configured.isEmpty()) {
                db.execSQL("DELETE FROM fts")
                db.execSQL("DELETE FROM meta")
                return
            }
            val placeholders = configured.joinToString(",") { "?" }
            val args = configured.toTypedArray()
            db.execSQL("DELETE FROM fts WHERE folder NOT IN ($placeholders)", args)
            db.execSQL("DELETE FROM meta WHERE folder NOT IN ($placeholders)", args)
        }
    }

    private fun queryPaths(
        db: SQLiteDatabase,
        folder: String,
        match: String,
        into: MutableSet<String>,
    ) {
        runCatching {
            db.rawQuery(
                "SELECT path FROM fts WHERE folder = ? AND fts MATCH ?",
                arrayOf(folder, match),
            ).use { cursor ->
                while (cursor.moveToNext()) into.add(cursor.getString(0))
            }
        }
    }

    private companion object {
        const val MAX_BODY_BYTES = 64 * 1024
        val LOCK = Any()
    }
}

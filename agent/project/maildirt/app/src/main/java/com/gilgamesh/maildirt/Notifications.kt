package com.gilgamesh.maildirt

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.os.FileObserver
import android.os.IBinder
import android.util.Log
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import java.io.File
import kotlinx.coroutines.CoroutineExceptionHandler
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

object AppVisibility {
    @Volatile
    var foreground: Boolean = false
}

data class OpenRequest(
    val folderId: String?,
    val folderRole: String?,
    val path: String,
)

object OpenRequests {
    var pending: OpenRequest? by mutableStateOf(null)
}

class MailNotificationService : Service() {
    private val failureHandler = CoroutineExceptionHandler { _, throwable ->
        Log.e(TAG, "notification watcher task failed", throwable)
    }
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO + failureHandler)
    private val observers = mutableListOf<FileObserver>()
    private var scanJob: Job? = null
    private var pollJob: Job? = null
    private var watchedPath: String? = null
    private lateinit var prefs: android.content.SharedPreferences

    override fun onCreate() {
        super.onCreate()
        prefs = getSharedPreferences("notify", 0)
        createChannels()
        runCatching { startForeground(SERVICE_NOTIFICATION_ID, foregroundNotification()) }
            .onFailure {
                Log.e(TAG, "could not start foreground watcher", it)
                stopSelf()
            }
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        scope.launch {
            runCatching {
                stopWatching()
                restartWatching()
            }.onFailure { Log.e(TAG, "could not start watching inbox", it) }
        }
        return START_STICKY
    }

    override fun onDestroy() {
        stopWatching()
        scope.cancel()
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    private suspend fun restartWatching() {
        val config = ConfigStore(this).load()
        val folder = config.folders.firstOrNull { it.role == FolderRole.INBOX }
        if (folder == null) {
            stopWatching()
            return
        }
        stopWatching()
        watchedPath = folder.path
        val pollSeconds = config.notifyPollSeconds.coerceIn(30, 86_400)

        val baselineKey = "baseline:${folder.path}"
        if (!prefs.getBoolean(baselineKey, false)) {
            val seen = loadSeen(folder.path)
            for (relativePath in messageFiles(folder)) {
                val key = notificationKey(File(folder.path, relativePath))
                if (key != null) seen.add(key)
            }
            saveSeen(folder.path, seen)
            prefs.edit().putBoolean(baselineKey, true).apply()
        }

        val mask = FileObserver.CREATE or FileObserver.MOVED_TO or FileObserver.CLOSE_WRITE or
            FileObserver.DELETE or FileObserver.MOVED_FROM
        for (sub in listOf("cur", "new")) {
            val dir = File(folder.path, sub)
            if (!dir.isDirectory) continue
            val observer = object : FileObserver(dir.absolutePath, mask) {
                override fun onEvent(event: Int, path: String?) {
                    scanJob?.cancel()
                    scanJob = scope.launch {
                        delay(800)
                        scan(folder)
                    }
                }
            }
            observer.startWatching()
            observers.add(observer)
        }

        Log.d(TAG, "watching ${folder.path} (poll ${pollSeconds}s)")
        scan(folder)

        pollJob = scope.launch {
            var lastCur = File(folder.path, "cur").lastModified()
            var lastNew = File(folder.path, "new").lastModified()
            while (isActive) {
                delay(pollSeconds * 1000L)
                val cur = File(folder.path, "cur").lastModified()
                val new = File(folder.path, "new").lastModified()
                if (cur != lastCur || new != lastNew) {
                    lastCur = cur
                    lastNew = new
                    scan(folder)
                }
            }
        }
    }

    private fun stopWatching() {
        scanJob?.cancel()
        pollJob?.cancel()
        observers.forEach { runCatching { it.stopWatching() } }
        observers.clear()
        watchedPath = null
    }

    private fun messageFiles(folder: MaildirFolder): List<String> {
        val result = ArrayList<String>()
        for (sub in listOf("cur", "new")) {
            val dir = File(folder.path, sub)
            dir.listFiles()?.forEach { file ->
                if (file.isFile && !file.name.startsWith(".")) result.add("$sub/${file.name}")
            }
        }
        return result
    }

    private fun scan(folder: MaildirFolder) {
        val config = ConfigStore(this).load()
        val seen = loadSeen(folder.path).toHashSet()
        val newKeys = ArrayList<String>()

        for (relativePath in messageFiles(folder)) {
            runCatching {
                val file = File(folder.path, relativePath)
                val key = notificationKey(file) ?: return@runCatching
                if (key in seen) return@runCatching
                seen.add(key)
                newKeys.add(key)
                if (AppVisibility.foreground) return@runCatching
                Log.d(TAG, "new message $relativePath")
                postMessageNotification(folder, relativePath, key, config)
            }.onFailure { Log.w(TAG, "could not inspect $relativePath", it) }
        }

        if (newKeys.isNotEmpty()) {
            val ordered = loadSeen(folder.path).apply { addAll(newKeys) }
            saveSeen(folder.path, ordered)
        }
    }

    private fun postMessageNotification(
        folder: MaildirFolder,
        relativePath: String,
        key: String,
        config: AppConfig,
    ) {
        if (!NotificationManagerCompat.from(this).areNotificationsEnabled()) return
        val file = File(folder.path, relativePath)
        val info = HeaderParser.info(file)
        val channel = if (config.notifySound) CHANNEL_SOUND else CHANNEL_SILENT

        val title: String
        val text: String
        if (config.notifyShowContent) {
            title = info.from
            text = info.subject
        } else {
            title = "New mail"
            text = "Open maildirt to read it"
        }
        val snippet = if (config.notifyShowContent) {
            HeaderParser.bodyText(file, 4096).trim().take(200).ifBlank { null }
        } else {
            null
        }
        val style = NotificationCompat.BigTextStyle()
            .bigText(listOfNotNull(text, snippet).joinToString("\n"))

        val openIntent = Intent(this, MainActivity::class.java).apply {
            putExtra("folderId", folder.id)
            putExtra("folderRole", folder.role.name)
            putExtra("path", relativePath)
            flags = Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_CLEAR_TOP
        }
        val openPending = PendingIntent.getActivity(
            this,
            key.hashCode(),
            openIntent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )

        val builder = NotificationCompat.Builder(this, channel)
            .setSmallIcon(R.drawable.ic_launcher_foreground)
            .setContentTitle(title)
            .setContentText(text)
            .setStyle(style)
            .setWhen(info.date)
            .setAutoCancel(true)
            .setContentIntent(openPending)
            .setCategory(NotificationCompat.CATEGORY_EMAIL)

        builder.addAction(
            R.drawable.ic_reply,
            "Mark read",
            actionIntent(folder, relativePath, "read", key.hashCode()),
        )
        if (config.folders.any { it.role == FolderRole.ARCHIVE }) {
            builder.addAction(
                R.drawable.ic_archive,
                "Archive",
                actionIntent(folder, relativePath, "archive", key.hashCode()),
            )
        }

        runCatching {
            NotificationManagerCompat.from(this).notify(key.hashCode(), builder.build())
        }.onSuccess {
            Log.d(TAG, "posted notification for $relativePath")
        }.onFailure {
            Log.w(TAG, "could not post notification for $relativePath", it)
        }
    }

    private fun actionIntent(
        folder: MaildirFolder,
        relativePath: String,
        operation: String,
        id: Int,
    ): PendingIntent {
        val intent = Intent(this, NotificationActionReceiver::class.java).apply {
            putExtra("folderId", folder.id)
            putExtra("path", relativePath)
            putExtra("op", operation)
            putExtra("notificationId", id)
        }
        return PendingIntent.getBroadcast(
            this,
            id + operation.hashCode(),
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
    }

    private fun notificationKey(file: File): String? {
        if (!file.isFile) return null
        val id = HeaderParser.messageId(file)
        if (!id.isNullOrBlank()) return id
        val base = file.name
        val cut = base.indexOfLast { it == ':' || it == ';' }
        return if (cut > 0) base.substring(0, cut) else base
    }

    /**
     * Seen keys were once stored as a StringSet; current builds store a newline-separated
     * String. Read either form and rewrite the legacy type so it cannot crash the process.
     */
    private fun loadSeen(folderPath: String): MutableList<String> {
        val key = "seen:$folderPath"
        val stored = runCatching { prefs.all[key] }.getOrNull() ?: return mutableListOf()
        val keys = LinkedHashSet<String>()
        when (stored) {
            is String -> keys.addAll(stored.split('\n').filter { it.isNotBlank() })
            is Set<*> -> {
                keys.addAll(stored.filterIsInstance<String>())
                runCatching { prefs.edit().putString(key, keys.joinToString("\n")).apply() }
                    .onFailure { Log.w(TAG, "could not migrate seen keys", it) }
            }
            else -> Log.w(TAG, "unexpected seen-key type ${stored.javaClass.name}")
        }
        return keys.toMutableList()
    }

    private fun saveSeen(folderPath: String, keys: List<String>) {
        val trimmed = if (keys.size > 5000) keys.takeLast(5000) else keys
        runCatching { prefs.edit().putString("seen:$folderPath", trimmed.joinToString("\n")).apply() }
            .onFailure { Log.w(TAG, "could not save seen keys", it) }
    }

    private fun createChannels() {
        val manager = getSystemService(NotificationManager::class.java)
        manager.createNotificationChannel(
            NotificationChannel(CHANNEL_SERVICE, "Mail watcher", NotificationManager.IMPORTANCE_MIN),
        )
        manager.createNotificationChannel(
            NotificationChannel(CHANNEL_SOUND, "New mail", NotificationManager.IMPORTANCE_HIGH),
        )
        manager.createNotificationChannel(
            NotificationChannel(CHANNEL_SILENT, "New mail (silent)", NotificationManager.IMPORTANCE_LOW),
        )
    }

    private fun foregroundNotification(): Notification {
        val intent = Intent(this, MainActivity::class.java)
        val pending = PendingIntent.getActivity(
            this,
            0,
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
        return NotificationCompat.Builder(this, CHANNEL_SERVICE)
            .setSmallIcon(R.drawable.ic_launcher_foreground)
            .setContentTitle("maildirt")
            .setContentText("Watching inbox for new mail")
            .setOngoing(true)
            .setContentIntent(pending)
            .setPriority(NotificationCompat.PRIORITY_MIN)
            .build()
    }

    companion object {
        private const val TAG = "MailNotify"
        const val CHANNEL_SERVICE = "service"
        const val CHANNEL_SOUND = "mail_sound"
        const val CHANNEL_SILENT = "mail_silent"
        const val SERVICE_NOTIFICATION_ID = 1

        fun start(context: Context) {
            runCatching {
                ContextCompat.startForegroundService(
                    context,
                    Intent(context, MailNotificationService::class.java),
                )
            }
        }
    }
}

class NotificationActionReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val folderId = intent.getStringExtra("folderId") ?: return
        val path = intent.getStringExtra("path") ?: return
        val operation = intent.getStringExtra("op") ?: return
        val notificationId = intent.getIntExtra("notificationId", 0)

        runCatching {
            val config = ConfigStore(context).load()
            val folder = config.folders.firstOrNull { it.id == folderId } ?: return
            val source = MaildirFolderStore(folder)
            val entry = source.entry(path) ?: return

            when (operation) {
                "read" -> source.setSeen(entry, true)
                "archive" -> {
                    val archive = config.folders.firstOrNull { it.role == FolderRole.ARCHIVE }
                    if (archive != null) {
                        source.moveTo(MaildirFolderStore(archive), entry, addSeen = true)
                    }
                }
            }
            NotificationManagerCompat.from(context).cancel(notificationId)
        }.onFailure {
            Log.w("MailNotify", "notification action $operation failed", it)
        }
    }
}

/** Restart the inbox watcher after a reboot or an app update. */
class MailNotificationBootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        when (intent.action) {
            Intent.ACTION_BOOT_COMPLETED,
            Intent.ACTION_MY_PACKAGE_REPLACED,
            -> MailNotificationService.start(context)
        }
    }
}

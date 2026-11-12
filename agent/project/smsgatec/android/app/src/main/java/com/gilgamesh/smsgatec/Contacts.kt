package com.gilgamesh.smsgatec

import android.content.Context
import android.content.pm.PackageManager
import android.provider.ContactsContract
import androidx.core.content.ContextCompat
import java.util.Locale

/** Names come only from the Android system contacts provider. */
class ContactBook(private val context: Context) {
    private var system: Map<String, Contact> = emptyMap()
    private var systemLoaded = false

    fun hasContactsPermission(): Boolean =
        ContextCompat.checkSelfPermission(context, android.Manifest.permission.READ_CONTACTS) ==
            PackageManager.PERMISSION_GRANTED

    private fun loadSystem() {
        if (systemLoaded) return
        systemLoaded = true
        if (!hasContactsPermission()) return
        val result = mutableMapOf<String, Contact>()
        try {
            context.contentResolver.query(
                ContactsContract.CommonDataKinds.Phone.CONTENT_URI,
                arrayOf(
                    ContactsContract.CommonDataKinds.Phone.DISPLAY_NAME,
                    ContactsContract.CommonDataKinds.Phone.NUMBER,
                ),
                null, null, null,
            )?.use { cursor ->
                val nameIndex = cursor.getColumnIndex(ContactsContract.CommonDataKinds.Phone.DISPLAY_NAME)
                val numberIndex = cursor.getColumnIndex(ContactsContract.CommonDataKinds.Phone.NUMBER)
                while (cursor.moveToNext()) {
                    val name = cursor.getString(nameIndex)?.trim().orEmpty()
                    val number = cursor.getString(numberIndex) ?: continue
                    val key = phoneKey(number)
                    if (key.isEmpty()) continue
                    result[key] = Contact(name.ifEmpty { "Unnamed contact" })
                }
            }
        } catch (e: Exception) {
            // The provider can be unavailable; numbers remain usable.
        }
        system = result
    }

    fun invalidateSystem() {
        systemLoaded = false
        system = emptyMap()
    }

    @Synchronized
    fun read(query: String = ""): Map<String, Contact> {
        loadSystem()
        val terms = query.lowercase(Locale.ROOT).split(Regex("\\s+")).filter { it.isNotEmpty() }
        if (terms.isEmpty()) return system
        return system.filter { (number, contact) ->
            val name = contact.name.lowercase(Locale.ROOT)
            terms.all { name.contains(it) || number.contains(it) }
        }
    }
}

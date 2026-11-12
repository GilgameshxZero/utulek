package com.gilgamesh.smsgatec

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class ModelTest {
    @Test
    fun recipientNormalizesAndValidates() {
        assertEquals("+12025550123", recipient("00 1 (202) 555-0123"))
        assertEquals("+12025550123", recipient("+1 202-555-0123"))
        assertNull(recipient("555-0123"))
        assertNull(recipient("+0123456789"))
    }

    @Test
    fun phoneKeyUsesLastDigits() {
        assertEquals("+2025550123", phoneKey("(202) 555-0123"))
        assertEquals("+12025550123", phoneKey("0012025550123"))
    }

    @Test
    fun mergeKeepsContentAndStrongerDeliveryState() {
        val old = Message(
            id = "1", number = "+12025550123", direction = "out",
            text = "hello", timestamp = "2026-01-01T00:00:00Z", state = "Delivered",
            type = "MMS", media = listOf(
                Attachment(partId = 1, name = "a.jpg", contentType = "image/jpeg", localPath = "/tmp/a"),
            ),
        )
        val refreshed = Message(
            id = "1", number = "+12025550123", direction = "out",
            text = "[Text unavailable]", timestamp = "2026-01-01T00:00:00Z", state = "Sent",
            type = "SMS",
        )
        val merged = mergeMessage(old, refreshed)
        assertEquals("hello", merged.text)
        assertEquals("Delivered", merged.state)
        assertEquals("MMS", merged.type)
        assertEquals("/tmp/a", merged.media.first().localPath)
    }

    @Test
    fun apiMessageMapsDirectionsAndMedia() {
        val incoming = ApiMessage(
            id = "in1", createdAt = "2026-01-01T00:00:00Z", sender = "+12025550123",
            contentPreview = "hi", type = "MMS",
            attachments = listOf(ApiAttachment(partId = 3, name = "x.jpg", contentType = "image/jpeg", size = 9)),
        ).messages().single()
        assertEquals("in", incoming.direction)
        assertEquals("+12025550123", incoming.number)
        assertEquals(3, incoming.media.single().partId)

        val outgoing = ApiMessage(
            id = "out1", createdAt = "2026-01-01T00:00:00Z", state = "Sent",
            type = "MMS", recipients = listOf(ApiRecipient("+18015582948", "Delivered")),
            mmsMessage = ApiMms(text = "caption", subject = "subject"),
            attachments = listOf(ApiAttachment(partId = 1, name = "y.png", contentType = "image/png")),
        ).messages().single()
        assertEquals("out", outgoing.direction)
        assertEquals("Delivered", outgoing.state)
        assertEquals("caption", outgoing.text)
        assertEquals("subject", outgoing.subject)
        assertEquals(1, outgoing.media.size)
    }

    @Test
    fun javaHashMatchesJavaSemantics() {
        assertEquals(96354, javaHash("abc"))
    }

    @Test
    fun configValidationRejectsBadIntervals() {
        val account = Account(id = "a", name = "A", server = "http://10.0.0.1:8080", username = "u")
        val base = Config(accounts = listOf(account), activeAccount = "a")
        assertNull(base.validate())
        assertEquals("Poll must be 15 to 3600 seconds", base.copy(pollSeconds = 5).validate())
        assertEquals("Reconcile must be 60 to 86400 seconds", base.copy(reconcileSeconds = 5).validate())
        assertEquals("Timeout must be 3 to 120 seconds", base.copy(timeoutSeconds = 500).validate())
        assertEquals("Theme must be auto, light, or dark", base.copy(theme = "gray").validate())
    }

    @Test
    fun messageIdsAreUrlSafeAndUnique() {
        val id = messageID()
        assertEquals(20, id.length)
        assertTrue(id.none { it == '+' || it == '/' || it == '=' })
        assertTrue(messageID() != id)
    }

    @Test
    fun groupMessagesParseAndShareAConversation() {
        val incoming = ApiMessage(
            id = "gin", createdAt = "2026-01-01T00:00:00Z", sender = "+12025550123",
            contentPreview = "Subject: Weekend\nhello all", type = "MMS",
            groupId = "g.Group 1", participants = listOf("+12025550123", "+12025550124"),
            mmsMessage = ApiMms(text = "hello all", subject = "Weekend"),
        ).messages().single()
        assertEquals("g.Group 1", incoming.conversationKey)
        assertEquals("+12025550123", incoming.sender)
        assertEquals("", incoming.groupName)
        assertEquals("hello all", incoming.text)
        assertEquals(2, incoming.participants.size)

        val outgoing = ApiMessage(
            id = "gout", createdAt = "2026-01-01T00:01:00Z", state = "Sent",
            recipients = listOf(ApiRecipient("+12025550123"), ApiRecipient("+12025550124")),
            type = "MMS", mmsMessage = ApiMms(text = "hi group", subject = "Weekend"),
        ).messages().single()
        assertEquals(localGroupKey(listOf("+12025550124", "+12025550123")), outgoing.conversationKey)
        assertEquals("", outgoing.groupName)
        assertEquals(2, outgoing.participants.size)

        val named = ApiMessage(
            id = "greal", createdAt = "2026-01-01T00:02:00Z", state = "Sent",
            recipients = listOf(ApiRecipient("+12025550123"), ApiRecipient("+12025550124")),
            groupId = "g.Real", participants = listOf("+12025550123", "+12025550124"),
            type = "MMS", mmsMessage = ApiMms(text = "hi group", subject = "Weekend"),
        ).messages().single()
        assertEquals("g.Real", named.conversationKey)
    }

    @Test
    fun latestByConversationPicksNewestRecord() {
        val older = Message(id = "a", number = "+1", direction = "in", text = "old", timestamp = "2026-01-01T00:00:00Z")
        val newer = Message(id = "b", number = "+1", direction = "in", text = "new", timestamp = "2026-01-02T00:00:00Z")
        val other = Message(id = "c", number = "+2", direction = "in", text = "x", timestamp = "2026-01-03T00:00:00Z")
        val latest = latestByConversation(listOf(older, newer, other))
        assertEquals("b", latest["+1"]?.id)
        assertEquals("c", latest["+2"]?.id)
    }
}

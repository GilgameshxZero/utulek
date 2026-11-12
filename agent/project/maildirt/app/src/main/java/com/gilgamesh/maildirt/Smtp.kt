package com.gilgamesh.maildirt

import android.content.Context
import android.net.Uri
import java.io.ByteArrayOutputStream
import java.util.Properties
import javax.mail.Message
import javax.mail.Session
import javax.mail.Transport
import javax.mail.internet.InternetAddress
import javax.mail.internet.MimeBodyPart
import javax.mail.internet.MimeMessage
import javax.mail.internet.MimeMultipart
import javax.mail.internet.MimeUtility
import javax.mail.util.ByteArrayDataSource

data class OutgoingAttachment(
    val uri: Uri,
    val name: String,
    val mimeType: String,
)

data class OutgoingMail(
    val identity: SenderIdentity,
    val to: String,
    val cc: String,
    val bcc: String,
    val subject: String,
    val body: String,
    val attachments: List<OutgoingAttachment>,
    val inReplyTo: String?,
    val references: String?,
)

class SmtpClient(private val credentials: CredentialStore) {

    fun send(identity: SenderIdentity, message: MimeMessage) {
        val transport = connect(identity)
        try {
            val recipients = message.allRecipients
                ?: throw IllegalStateException("No recipients")
            transport.sendMessage(message, recipients)
        } finally {
            runCatching { transport.close() }
        }
    }

    /** Connects and authenticates without sending; used by the identity editor's test action. */
    fun test(identity: SenderIdentity, secret: String = credentials.read(identity.id)
        ?: throw IllegalStateException("No stored credential for ${identity.email}")): String {
        val transport = connect(identity, secret)
        try {
            return "SMTP authentication succeeded"
        } finally {
            runCatching { transport.close() }
        }
    }

    private fun connect(
        identity: SenderIdentity,
        secret: String = credentials.read(identity.id)
            ?: throw IllegalStateException("No stored credential for ${identity.email}"),
    ): Transport {
        val authPassword = when (identity.authMode) {
            AuthMode.XOAUTH2 -> GmailTokenProvider(identity, secret).accessToken()
            AuthMode.STANDARD -> secret
        }

        val props = Properties()
        props["mail.transport.protocol"] = "smtp"
        props["mail.smtp.host"] = identity.smtpHost
        props["mail.smtp.port"] = identity.smtpPort.toString()
        props["mail.smtp.auth"] = "true"
        props["mail.smtp.connectiontimeout"] = "20000"
        props["mail.smtp.timeout"] = "30000"
        props["mail.smtp.writetimeout"] = "30000"
        props["mail.mime.charset"] = "UTF-8"
        props["mail.mime.decodetext.strict"] = "false"
        when (identity.security) {
            SmtpSecurity.STARTTLS -> {
                props["mail.smtp.starttls.enable"] = "true"
                props["mail.smtp.starttls.required"] = "true"
            }
            SmtpSecurity.SSL -> props["mail.smtp.ssl.enable"] = "true"
            SmtpSecurity.NONE -> {
                props["mail.smtp.starttls.enable"] = "false"
                props["mail.smtp.ssl.enable"] = "false"
            }
        }
        when (identity.authMode) {
            AuthMode.XOAUTH2 -> {
                props["mail.smtp.auth.mechanisms"] = "XOAUTH2"
                props["mail.smtp.auth.login.disable"] = "true"
            }
            AuthMode.STANDARD -> if (identity.security == SmtpSecurity.NONE) {
                props["mail.smtp.auth.mechanisms"] = "LOGIN"
            }
        }

        val session = Session.getInstance(props)
        val transport = session.getTransport("smtp")
        transport.connect(identity.smtpHost, identity.smtpPort, identity.username, authPassword)
        return transport
    }
}

object MailComposer {
    fun newSession(): Session = Session.getInstance(
        Properties().apply {
            put("mail.mime.charset", "UTF-8")
            put("mail.mime.decodetext.strict", "false")
            put("mail.mime.address.strict", "false")
        },
    )

    fun build(context: Context, mail: OutgoingMail): MimeMessage {
        val session = newSession()
        val message = MimeMessage(session)

        val from = if (mail.identity.displayName.isBlank()) {
            InternetAddress(mail.identity.email)
        } else {
            InternetAddress(mail.identity.email, mail.identity.displayName, "UTF-8")
        }
        message.setFrom(from)

        val to = parse(mail.to)
        val cc = parse(mail.cc)
        val bcc = parse(mail.bcc)
        if (to.isNotEmpty()) message.setRecipients(Message.RecipientType.TO, to)
        if (cc.isNotEmpty()) message.setRecipients(Message.RecipientType.CC, cc)
        if (bcc.isNotEmpty()) message.setRecipients(Message.RecipientType.BCC, bcc)

        message.setSubject(mail.subject, "UTF-8")
        message.setSentDate(java.util.Date())
        mail.inReplyTo?.takeIf { it.isNotBlank() }?.let {
            message.setHeader("In-Reply-To", it)
        }
        mail.references?.takeIf { it.isNotBlank() }?.let {
            message.setHeader("References", it)
        }

        if (mail.attachments.isEmpty()) {
            message.setText(mail.body, "UTF-8")
        } else {
            val multipart = MimeMultipart("mixed")
            multipart.addBodyPart(MimeBodyPart().apply { setText(mail.body, "UTF-8") })
            for (attachment in mail.attachments) {
                val bytes = context.contentResolver.openInputStream(attachment.uri)?.use {
                    it.readBytes()
                } ?: throw IllegalStateException("Could not read attachment ${attachment.name}")
                val part = MimeBodyPart().apply {
                    setFileName(MimeUtility.encodeText(attachment.name))
                    dataHandler = javax.activation.DataHandler(
                        ByteArrayDataSource(bytes, attachment.mimeType),
                    )
                }
                multipart.addBodyPart(part)
            }
            message.setContent(multipart)
        }

        message.saveChanges()
        return message
    }

    fun buildCalendar(
        identity: SenderIdentity,
        to: String,
        subject: String,
        calendar: String,
    ): MimeMessage {
        val message = MimeMessage(newSession())
        val from = if (identity.displayName.isBlank()) {
            InternetAddress(identity.email)
        } else {
            InternetAddress(identity.email, identity.displayName, "UTF-8")
        }
        message.setFrom(from)
        val recipients = parse(to)
        if (recipients.isEmpty()) throw IllegalStateException("No organizer address")
        message.setRecipients(Message.RecipientType.TO, recipients)
        message.setSubject(subject, "UTF-8")
        message.setSentDate(java.util.Date())
        message.setHeader("MIME-Version", "1.0")

        // neomutt_xp replies are a single text/calendar part with method=REPLY;
        // other clients key their RSVP ingestion off these parameters.
        val type = "text/calendar; charset=UTF-8; method=REPLY; component=VEVENT; name=reply.ics"
        message.dataHandler = javax.activation.DataHandler(
            ByteArrayDataSource(calendar.toByteArray(Charsets.UTF_8), type),
        )
        message.setHeader("Content-Type", type)
        message.setHeader("Content-Disposition", "inline; filename=reply.ics")
        message.saveChanges()
        return message
    }

    fun toBytes(message: MimeMessage): ByteArray =
        ByteArrayOutputStream().use { output ->
            message.writeTo(output)
            output.toByteArray()
        }

    fun parse(value: String): Array<InternetAddress> {
        if (value.isBlank()) return emptyArray()
        val parsed = InternetAddress.parse(value, true)
        for (address in parsed) {
            val personal = address.personal
            if (!personal.isNullOrBlank() && personal.any { it.code > 127 }) {
                runCatching { address.setPersonal(personal, "UTF-8") }
            }
        }
        return parsed
    }

    fun isValidAddressList(value: String): Boolean {
        if (value.isBlank()) return true
        return runCatching {
            InternetAddress.parse(value, true).all { address ->
                val mailbox = address.address.orEmpty()
                mailbox.contains('@') &&
                    mailbox.substringBefore('@').isNotBlank() &&
                    mailbox.substringAfter('@').isNotBlank()
            }
        }.getOrDefault(false)
    }
}

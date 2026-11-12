package com.gilgamesh.maildirt

import java.io.ByteArrayOutputStream
import java.io.File
import java.util.Date
import org.bouncycastle.jce.provider.BouncyCastleProvider
import org.bouncycastle.bcpg.SymmetricKeyAlgorithmTags
import org.bouncycastle.openpgp.PGPCompressedData
import org.bouncycastle.openpgp.PGPEncryptedDataGenerator
import org.bouncycastle.openpgp.PGPEncryptedDataList
import org.bouncycastle.openpgp.PGPLiteralData
import org.bouncycastle.openpgp.PGPLiteralDataGenerator
import org.bouncycastle.openpgp.PGPPBEEncryptedData
import org.bouncycastle.openpgp.PGPUtil
import org.bouncycastle.openpgp.jcajce.JcaPGPObjectFactory
import org.bouncycastle.openpgp.operator.jcajce.JcePBEKeyEncryptionMethodGenerator
import org.bouncycastle.openpgp.operator.jcajce.JcePBEDataDecryptorFactoryBuilder
import org.bouncycastle.openpgp.operator.jcajce.JcePGPDataEncryptorBuilder
import java.security.SecureRandom
import java.security.Security

/** Symmetric OpenPGP (AES256, no compression, literal data) used by the Gmail token file. */
object GpgToken {
    // Android ships a provider named "BC"; lookups by name can hit it instead of
    // BouncyCastle's, so pass this instance explicitly to every builder.
    private val provider = BouncyCastleProvider()

    fun decrypt(file: File, passphrase: String): String {
        val decoder = PGPUtil.getDecoderStream(file.inputStream())
        decoder.use { input ->
            val factory = JcaPGPObjectFactory(input)
            val first = factory.nextObject()
            val encrypted = first as? PGPEncryptedDataList
                ?: throw IllegalArgumentException("Not a symmetrically encrypted OpenPGP file")
            val pbe = encrypted.encryptedDataObjects
                .asSequence()
                .filterIsInstance<PGPPBEEncryptedData>()
                .firstOrNull()
                ?: throw IllegalArgumentException("No symmetric encrypted data found")

            val decryptor = JcePBEDataDecryptorFactoryBuilder()
                .setProvider(provider)
                .build(passphrase.toCharArray())
            val clear = pbe.getDataStream(decryptor)
            var objects = JcaPGPObjectFactory(clear)
            var payload = objects.nextObject()
            while (payload is PGPCompressedData) {
                objects = JcaPGPObjectFactory(payload.dataStream)
                payload = objects.nextObject()
            }
            val literal = payload as? PGPLiteralData
                ?: throw IllegalArgumentException("No literal data in token file")
            return literal.inputStream.bufferedReader(Charsets.UTF_8).use { it.readText() }
        }
    }

    fun encrypt(text: String, passphrase: String): ByteArray {
        val target = ByteArrayOutputStream()
        val encryptor = PGPEncryptedDataGenerator(
            JcePGPDataEncryptorBuilder(SymmetricKeyAlgorithmTags.AES_256)
                .setWithIntegrityPacket(true)
                .setSecureRandom(SecureRandom())
                .setProvider(provider),
        )
        encryptor.addMethod(
            JcePBEKeyEncryptionMethodGenerator(passphrase.toCharArray())
                .setProvider(provider),
        )

        val output = encryptor.open(target, ByteArray(1 shl 16))
        val literalGen = PGPLiteralDataGenerator()
        val data = text.toByteArray(Charsets.UTF_8)
        val literal = literalGen.open(
            output,
            PGPLiteralData.BINARY,
            PGPLiteralData.CONSOLE,
            data.size.toLong(),
            Date(),
        )
        literal.write(data)
        literal.close()
        literalGen.close()
        output.close()
        encryptor.close()
        return target.toByteArray()
    }
}

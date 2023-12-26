#include <rain.hpp>

#include <cassert>
#include <iomanip>
#include <iostream>

/*
Resources:
* <https://wiki.osdev.org/TLS_Handshake>.
* <https://tls.ulfheim.net/>.
* <https://datatracker.ietf.org/doc/html/rfc5246#section-7.4.1.2>.
* <https://datatracker.ietf.org/doc/html/rfc5246#section-7.2.2>.
* <https://www.howsmyssl.com/s/about.html#insecure-cipher-suites>.
* <https://www.ssllabs.com/ssltest/analyze.html?d=www.howsmyssl.com>.
* <https://www.ssllabs.com/ssltest/analyze.html?d=wikipedia.org&s=208.80.153.224>.
*/

int main() {
	using namespace Rain::Literal;
	using namespace Rain::Networking;
	using namespace Rain::Time;

	{
		ReqRes::Client<
			ReqRes::Request,
			ReqRes::Response,
			1_zu << 10,
			1_zu << 10,
			15000,
			15000,
			Ipv4FamilyInterface,
			StreamTypeInterface,
			TcpProtocolInterface,
			NoLingerSocketOption>
			// client("www.howsmyssl.com:443");
			// client("www.wikipedia.com:443");
			client("www.google.com:443");
		std::stringstream tlsPlaintext;
		tlsPlaintext
			<< "\x16"s	// ContentType type;
			<< "\x03\x03"s	// ProtocolVersion version;
			// << "\x00\x68"s	// uint16 length;
			<< "\x00\x65"s	// uint16 length;
			<< "\x01"s	// HandshakeType msg_type;
			// << "\x00\x00\x64"s	// uint24 length;
			<< "\x00\x00\x61"s	// uint24 length;
			<< "\x03\x03"s	// ProtocolVersion version;
			<< "\x00\x87\x20\xB9"s	// uint32 gmt_unix_time;
			<< "0123456789012345678901234567"s	// opaque random_bytes[28];
			<< "\x00"s	// SessionID session_id;
			// Most handshake_failure (0x28) errors are due to unsupported
			// cipher suites, or a EC cipher suite without a corresponding extension
			// (notably, EC point formats and elliptic curves). C030
			// (TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384), C02F, CCA8
			// (TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256) recommended for maximum
			// coverage and security (wikipedia, google, pixiv, usaco, nyaa).
			// << "\x00\x02\x00\x0A"s	// CipherSuite cipher_suites<2..2^16-2>;
			<< "\x00\x02\xCC\xA8"s	// CipherSuite cipher_suites<2..2^16-2>;
			<< "\x01\x00"s	// CompressionMethod compression_methods<1..2^8-1>;
			// << "\x00\x39"s	// Extension extensions<0..2^16-1>;
			<< "\x00\x36"s	// Extension extensions<0..2^16-1>;
			// Extension - Server Name for `www.wikipedia.org`.
			// << "\x00\x00\x00\x16\x00\x14\x00\x00\x11www.howsmyssl.com"s
			// << "\x00\x00\x00\x16\x00\x14\x00\x00\x11www.wikipedia.com"s
			<< "\x00\x00\x00\x13\x00\x11\x00\x00\x0Ewww.google.com"s
			<< "\x00\x12\x00\x00"s	// Extension - SCT.
			<< "\xFF\x01\x00\x01\x00"s	// Extension - Renegotiation info.
			// Extension - Signature algorithms.
			<< "\x00\x0D\x00\x04\x00\x02\x05\x01"s
			<< "\x00\x0B\x00\x02\x01\x00"s	// Extension - EC point formats.
			<< "\x00\x0A\x00\x04\x00\x02\x00\x1D"s	// Extension - Elliptic curves.
			;
		client.send(tlsPlaintext.str());
		std::this_thread::sleep_for(2s);

		std::string buffer(8192, '\0');
		client.recv(buffer);
		std::cout << buffer.length() << std::endl << std::hex << std::setfill('0');
		for (char const &c : buffer) {
			std::cout << std::setw(2)
								<< static_cast<int>(static_cast<unsigned char>(c)) << ' ';
		}
		std::cout << std::endl;

		// Response stored as raw string so it won’t be formatted.
		std::string _ = R"(
		16 03 03 00 57 
		02 00 00 53 
		03 03 
		65 8a e8 ad 92 97 6e 48 99 f7 cf 6e b2 ab 39 4c 3e b4 01 fc 60 33 1f 24 44 4f 57 4e 47 52 44 01 
		20 f3 c7 51 0a 65 b9 ef 72 91 73 0b 75 30 a7 99 6d 3d fd e1 6a eb 0b 87 38 ac 32 d6 c4 d2 cf eb 85 
		cc a8 
		00 
		00 0b 
		ff 01 00 01 00 
		00 0b 00 02 01 00
		
		16 03 03 0f 9a 
		0b 00 0f 96 
		00 0f 93 
		00 04 8a 
		30 82 04 86 30 82 03 6e a0 03 02 01 02 02 10 31 22 c5 f3 b7 e2 a9 db 09 e1 3a 37 c5 4c a7 4d 30 0d 06 09 2a 86 48 86 f7 0d 01 01 0b 05 00 30 46 31 0b 30 09 06 03 55 04 06 13 02 55 53 31 22 30 20 06 03 55 04 0a 13 19 47 6f 6f 67 6c 65 20 54 72 75 73 74 20 53 65 72 76 69 63 65 73 20 4c 4c 43 31 13 30 11 06 03 55 04 03 13 0a 47 54 53 20 43 41 20 31 43 33 30 1e 17 0d 32 33 31 31 32 30 30 38 30 39 34 37 5a 17 0d 32 34 30 32 31 32 30 38 30 39 34 36 5a 30 19 31 17 30 15 06 03 55 04 03 13 0e 77 77 77 2e 67 6f 6f 67 6c 65 2e 63 6f 6d 30 59 30 13 06 07 2a 86 48 ce 3d 02 01 06 08 2a 86 48 ce 3d 03 01 07 03 42 00 04 32 38 74 92 35 5a 1d 03 f4 46 29 07 a0 87 2a ec 14 d4 da 91 5d da bb c3 5f 47 43 98 f5 78 62 c7 61 3f 30 48 6f 52 31 df ab 9d 4c 5c dd 3f 97 ee ad e0 a9 35 ab 9c 34 41 68 88 6a 9e c7 52 bb 26 a3 82 02 66 30 82 02 62 30 0e 06 03 55 1d 0f 01 01 ff 04 04 03 02 07 80 30 13 06 03 55 1d 25 04 0c 30 0a 06 08 2b 06 01 05 05 07 03 01 30 0c 06 03 55 1d 13 01 01 ff 04 02 30 00 30 1d 06 03 55 1d 0e 04 16 04 14 37 c4 63 a2 97 d1 56 79 bf 20 d7 d8 34 92 38 97 53 05 d2 50 30 1f 06 03 55 1d 23 04 18 30 16 80 14 8a 74 7f af 85 cd ee 95 cd 3d 9c d0 e2 46 14 f3 71 35 1d 27 30 6a 06 08 2b 06 01 05 05 07 01 01 04 5e 30 5c 30 27 06 08 2b 06 01 05 05 07 30 01 86 1b 68 74 74 70 3a 2f 2f 6f 63 73 70 2e 70 6b 69 2e 67 6f 6f 67 2f 67 74 73 31 63 33 30 31 06 08 2b 06 01 05 05 07 30 02 86 25 68 74 74 70 3a 2f 2f 70 6b 69 2e 67 6f 6f 67 2f 72 65 70 6f 2f 63 65 72 74 73 2f 67 74 73 31 63 33 2e 64 65 72 30 19 06 03 55 1d 11 04 12 30 10 82 0e 77 77 77 2e 67 6f 6f 67 6c 65 2e 63 6f 6d 30 21 06 03 55 1d 20 04 1a 30 18 30 08 06 06 67 81 0c 01 02 01 30 0c 06 0a 2b 06 01 04 01 d6 79 02 05 03 30 3c 06 03 55 1d 1f 04 35 30 33 30 31 a0 2f a0 2d 86 2b 68 74 74 70 3a 2f 2f 63 72 6c 73 2e 70 6b 69 2e 67 6f 6f 67 2f 67 74 73 31 63 33 2f 66 56 4a 78 62 56 2d 4b 74 6d 6b 2e 63 72 6c 30 82 01 03 06 0a 2b 06 01 04 01 d6 79 02 04 02 04 81 f4 04 81 f1 00 ef 00 76 00 76 ff 88 3f 0a b6 fb 95 51 c2 61 cc f5 87 ba 34 b4 a4 cd bb 29 dc 68 42 0a 9f e6 67 4c 5a 3a 74 00 00 01 8b eb fe 39 20 00 00 04 03 00 47 30 45 02 21 00 ef 3e 86 86 bb 04 bf 49 58 4b 53 63 51 c4 cd 78 45 22 b6 d3 ea ef 9b 78 a2 f0 f5 c2 ac 4a 8b 83 02 20 66 a4 7a 77 9f 14 bc 96 c5 a8 22 3a 0a 76 c9 94 ac c0 45 83 47 d9 07 bf ab e5 58 1a 79 d1 17 90 00 75 00 55 81 d4 c2 16 90 36 01 4a ea 0b 9b 57 3c 53 f0 c0 e4 38 78 70 25 08 17 2f a3 aa 1d 07 13 d3 0c 00 00 01 8b eb fe 39 07 00 00 04 03 00 46 30 44 02 20 2b 19 09 75 58 4e 3e ab 02 4c 44 44 a3 4e 29 52 94 13 05 27 f4 07 12 74 04 da 9e 11 85 81 6d ee 02 20 0f bc d8 01 8a d7 13 2c 70 5c 04 6b 57 a7 fd eb bd 22 bd 47 f3 b8 1a 85 a0 95 12 d6 6c bb 67 c2 30 0d 06 09 2a 86 48 86 f7 0d 01 01 0b 05 00 03 82 01 01 00 7e 77 aa a0 d4 6c 90 af bd 45 b6 b7 46 ef 31 6d 4d ec dd 71 9e 0f d5 85 74 c6 38 6c 1b 34 5e 47 1d e9 85 c6 90 cb eb eb 39 9f 08 f6 dd 4d 32 df 18 0e 3d 15 cf 39 91 45 c2 41 80 e6 a1 47 61 f4 d1 49 4b a9 1f 46 d1 f7 e8 33 aa d9 f5 e1 53 8e db 7c 2e 9d 8b 53 08 c3 8a 01 28 20 61 c1 53 10 ba dd ac c8 19 d5 01 4c 24 3d f9 4a 80 58 be e6 d6 a0 aa 93 23 ed 87 60 e6 41 3a f4 18 9e 7b ac 0a 9e 4e 33 62 5b d4 ce 80 56 85 e7 98 6a 24 0d b0 a7 9b 70 26 1a 04 cb 67 76 20 65 f5 b1 77 5b 94 a1 02 25 d8 95 9a bf 90 df 41 f6 e7 28 d2 fd 0e 76 f6 48 1f 47 39 bf 09 72 55 71 16 e9 78 3c a9 bf 3d a7 7e fd cb 67 76 7d 75 0d ea d8 db 67 58 04 d4 b5 8c 6b 33 70 91 28 5d 3e f0 ce c7 58 4b e4 c0 64 92 1a 11 a5 4b 8a 29 fb 88 f4 3a 3d 47 9e ce c2 3d 78 a6 6c 67 45 f4 77 4a c4 d1 d7 
		00 05 9a 
		30 82 05 96 30 82 03 7e a0 03 02 01 02 02 0d 02 03 bc 53 59 6b 34 c7 18 f5 01 50 66 30 0d 06 09 2a 86 48 86 f7 0d 01 01 0b 05 00 30 47 31 0b 30 09 06 03 55 04 06 13 02 55 53 31 22 30 20 06 03 55 04 0a 13 19 47 6f 6f 67 6c 65 20 54 72 75 73 74 20 53 65 72 76 69 63 65 73 20 4c 4c 43 31 14 30 12 06 03 55 04 03 13 0b 47 54 53 20 52 6f 6f 74 20 52 31 30 1e 17 0d 32 30 30 38 31 33 30 30 30 30 34 32 5a 17 0d 32 37 30 39 33 30 30 30 30 30 34 32 5a 30 46 31 0b 30 09 06 03 55 04 06 13 02 55 53 31 22 30 20 06 03 55 04 0a 13 19 47 6f 6f 67 6c 65 20 54 72 75 73 74 20 53 65 72 76 69 63 65 73 20 4c 4c 43 31 13 30 11 06 03 55 04 03 13 0a 47 54 53 20 43 41 20 31 43 33 30 82 01 22 30 0d 06 09 2a 86 48 86 f7 0d 01 01 01 05 00 03 82 01 0f 00 30 82 01 0a 02 82 01 01 00 f5 88 df e7 62 8c 1e 37 f8 37 42 90 7f 6c 87 d0 fb 65 82 25 fd e8 cb 6b a4 ff 6d e9 5a 23 e2 99 f6 1c e9 92 03 99 13 7c 09 0a 8a fa 42 d6 5e 56 24 aa 7a 33 84 1f d1 e9 69 bb b9 74 ec 57 4c 66 68 93 77 37 55 53 fe 39 10 4d b7 34 bb 5f 25 77 37 3b 17 94 ea 3c e5 9d d5 bc c3 b4 43 eb 2e a7 47 ef b0 44 11 63 d8 b4 41 85 dd 41 30 48 93 1b bf b7 f6 e0 45 02 21 e0 96 42 17 cf d9 2b 65 56 34 07 26 04 0d a8 fd 7d ca 2e ef ea 48 7c 37 4d 3f 00 9f 83 df ef 75 84 2e 79 57 5c fc 57 6e 1a 96 ff fc 8c 9a a6 99 be 25 d9 7f 96 2c 06 f7 11 2a 02 80 80 eb 63 18 3c 50 49 87 e5 8a ca 5f 19 2b 59 96 81 00 a0 fb 51 db ca 77 0b 0b c9 96 4f ef 70 49 c7 5c 6d 20 fd 99 b4 b4 e2 ca 2e 77 fd 2d dc 0b b6 6b 13 0c 8c 19 2b 17 96 98 b9 f0 8b f6 a0 27 bb b6 e3 8d 51 8f bd ae c7 9b b1 89 9d 02 03 01 00 01 a3 82 01 80 30 82 01 7c 30 0e 06 03 55 1d 0f 01 01 ff 04 04 03 02 01 86 30 1d 06 03 55 1d 25 04 16 30 14 06 08 2b 06 01 05 05 07 03 01 06 08 2b 06 01 05 05 07 03 02 30 12 06 03 55 1d 13 01 01 ff 04 08 30 06 01 01 ff 02 01 00 30 1d 06 03 55 1d 0e 04 16 04 14 8a 74 7f af 85 cd ee 95 cd 3d 9c d0 e2 46 14 f3 71 35 1d 27 30 1f 06 03 55 1d 23 04 18 30 16 80 14 e4 af 2b 26 71 1a 2b 48 27 85 2f 52 66 2c ef f0 89 13 71 3e 30 68 06 08 2b 06 01 05 05 07 01 01 04 5c 30 5a 30 26 06 08 2b 06 01 05 05 07 30 01 86 1a 68 74 74 70 3a 2f 2f 6f 63 73 70 2e 70 6b 69 2e 67 6f 6f 67 2f 67 74 73 72 31 30 30 06 08 2b 06 01 05 05 07 30 02 86 24 68 74 74 70 3a 2f 2f 70 6b 69 2e 67 6f 6f 67 2f 72 65 70 6f 2f 63 65 72 74 73 2f 67 74 73 72 31 2e 64 65 72 30 34 06 03 55 1d 1f 04 2d 30 2b 30 29 a0 27 a0 25 86 23 68 74 74 70 3a 2f 2f 63 72 6c 2e 70 6b 69 2e 67 6f 6f 67 2f 67 74 73 72 31 2f 67 74 73 72 31 2e 63 72 6c 30 57 06 03 55 1d 20 04 50 30 4e 30 38 06 0a 2b 06 01 04 01 d6 79 02 05 03 30 2a 30 28 06 08 2b 06 01 05 05 07 02 01 16 1c 68 74 74 70 73 3a 2f 2f 70 6b 69 2e 67 6f 6f 67 2f 72 65 70 6f 73 69 74 6f 72 79 2f 30 08 06 06 67 81 0c 01 02 01 30 08 06 06 67 81 0c 01 02 02 30 0d 06 09 2a 86 48 86 f7 0d 01 01 0b 05 00 03 82 02 01 00 89 7d ac 20 5c 0c 3c be 9a a8 57 95 1b b4 ae fa ab a5 72 71 b4 36 95 fd df 40 11 03 4c c2 46 14 bb 14 24 ab f0 50 71 22 db ad c4 6e 7f cf f1 6a 6f c8 83 1b d8 ce 89 5f 87 6c 87 b8 a9 0c a3 9b a1 62 94 93 95 df 5b ae 66 19 0b 02 96 9e fc b5 e7 10 69 3e 7a cb 46 49 5f 46 e1 41 b1 d7 98 4d 65 34 00 80 1a 3f 4f 9f 6c 7f 49 00 81 53 41 a4 92 21 82 82 1a f1 a3 44 5b 2a 50 12 13 4d c1 53 36 f3 42 08 af 54 fa 8e 77 53 1b 64 38 27 17 09 bd 58 c9 1b 7c 39 2d 5b f3 ce d4 ed 97 db 14 03 bf 09 53 24 1f c2 0c 04 79 98 26 f2 61 f1 53 52 fd 42 8c 1b 66 2b 3f 15 a1 bb ff f6 9b e3 81 9a 01 06 71 89 35 28 24 dd e1 bd eb 19 2d e1 48 cb 3d 59 83 51 b4 74 c6 9d 7c c6 b1 86 5b af cc 34 c4 d3 cc d4 81 11 95 00 a1 f4 12 22 01 fa b4 83 71 af 8c b7 8c 73 24 ac 37 53 c2 00 90 3f 11 fe 5c ed 36 94 10 3b bd 29 ae e2 c7 3a 62 3b 6c 63 d9 80 bf 59 71 ac 63 27 b9 4c 17 a0 da f6 73 15 bf 2a de 8f f3 a5 6c 32 81 33 03 d0 86 51 71 99 34 ba 93 8d 5d b5 51 58 f7 b2 93 e8 01 f6 59 be 71 9b fd 4d 28 ce cf 6d c7 16 dc f7 d1 d6 46 9b a7 ca 6b e9 77 0f fd a0 b6 1b 23 83 1d 10 1a d9 09 00 84 e0 44 d3 a2 75 23 b3 34 86 f6 20 b0 a4 5e 10 1d e0 52 46 00 9d b1 0f 1f 21 70 51 f5 9a dd 06 fc 55 f4 2b 0e 33 77 c3 4b 42 c2 f1 77 13 fc 73 80 94 eb 1f bb 37 3f ce 02 2a 66 b0 73 1d 32 a5 32 6c 32 b0 8e e0 c4 23 ff 5b 7d 4d 65 70 ac 2b 9b 3d ce db e0 6d 8e 32 80 be 96 9f 92 63 bc 97 bb 5d b9 f4 e1 71 5e 2a e4 ef 03 22 b1 8a 65 3a 8f c0 93 65 d4 85 cd 0f 0f 5b 83 59 16 47 16 2d 9c 24 3a c8 80 a6 26 14 85 9b f6 37 9b ac 6f f9 c5 c3 06 51 f3 e2 7f c5 b1 10 ba 51 f4 dd 
		00 05 66 
		30 82 05 62 30 82 04 4a a0 03 02 01 02 02 10 77 bd 0d 6c db 36 f9 1a ea 21 0f c4 f0 58 d3 0d 30 0d 06 09 2a 86 48 86 f7 0d 01 01 0b 05 00 30 57 31 0b 30 09 06 03 55 04 06 13 02 42 45 31 19 30 17 06 03 55 04 0a 13 10 47 6c 6f 62 61 6c 53 69 67 6e 20 6e 76 2d 73 61 31 10 30 0e 06 03 55 04 0b 13 07 52 6f 6f 74 20 43 41 31 1b 30 19 06 03 55 04 03 13 12 47 6c 6f 62 61 6c 53 69 67 6e 20 52 6f 6f 74 20 43 41 30 1e 17 0d 32 30 30 36 31 39 30 30 30 30 34 32 5a 17 0d 32 38 30 31 32 38 30 30 30 30 34 32 5a 30 47 31 0b 30 09 06 03 55 04 06 13 02 55 53 31 22 30 20 06 03 55 04 0a 13 19 47 6f 6f 67 6c 65 20 54 72 75 73 74 20 53 65 72 76 69 63 65 73 20 4c 4c 43 31 14 30 12 06 03 55 04 03 13 0b 47 54 53 20 52 6f 6f 74 20 52 31 30 82 02 22 30 0d 06 09 2a 86 48 86 f7 0d 01 01 01 05 00 03 82 02 0f 00 30 82 02 0a 02 82 02 01 00 b6 11 02 8b 1e e3 a1 77 9b 3b dc bf 94 3e b7 95 a7 40 3c a1 fd 82 f9 7d 32 06 82 71 f6 f6 8c 7f fb e8 db bc 6a 2e 97 97 a3 8c 4b f9 2b f6 b1 f9 ce 84 1d b1 f9 c5 97 de ef b9 f2 a3 e9 bc 12 89 5e a7 aa 52 ab f8 23 27 cb a4 b1 9c 63 db d7 99 7e f0 0a 5e eb 68 a6 f4 c6 5a 47 0d 4d 10 33 e3 4e b1 13 a3 c8 18 6c 4b ec fc 09 90 df 9d 64 29 25 23 07 a1 b4 d2 3d 2e 60 e0 cf d2 09 87 bb cd 48 f0 4d c2 c2 7a 88 8a bb ba cf 59 19 d6 af 8f b0 07 b0 9e 31 f1 82 c1 c0 df 2e a6 6d 6c 19 0e b5 d8 7e 26 1a 45 03 3d b0 79 a4 94 28 ad 0f 7f 26 e5 a8 08 fe 96 e8 3c 68 94 53 ee 83 3a 88 2b 15 96 09 b2 e0 7a 8c 2e 75 d6 9c eb a7 56 64 8f 96 4f 68 ae 3d 97 c2 84 8f c0 bc 40 c0 0b 5c bd f6 87 b3 35 6c ac 18 50 7f 84 e0 4c cd 92 d3 20 e9 33 bc 52 99 af 32 b5 29 b3 25 2a b4 48 f9 72 e1 ca 64 f7 e6 82 10 8d e8 9d c2 8a 88 fa 38 66 8a fc 63 f9 01 f9 78 fd 7b 5c 77 fa 76 87 fa ec df b1 0e 79 95 57 b4 bd 26 ef d6 01 d1 eb 16 0a bb 8e 0b b5 c5 c5 8a 55 ab d3 ac ea 91 4b 29 cc 19 a4 32 25 4e 2a f1 65 44 d0 02 ce aa ce 49 b4 ea 9f 7c 83 b0 40 7b e7 43 ab a7 6c a3 8f 7d 89 81 fa 4c a5 ff d5 8e c3 ce 4b e0 b5 d8 b3 8e 45 cf 76 c0 ed 40 2b fd 53 0f b0 a7 d5 3b 0d b1 8a a2 03 de 31 ad cc 77 ea 6f 7b 3e d6 df 91 22 12 e6 be fa d8 32 fc 10 63 14 51 72 de 5d d6 16 93 bd 29 68 33 ef 3a 66 ec 07 8a 26 df 13 d7 57 65 78 27 de 5e 49 14 00 a2 00 7f 9a a8 21 b6 a9 b1 95 b0 a5 b9 0d 16 11 da c7 6c 48 3c 40 e0 7e 0d 5a cd 56 3c d1 97 05 b9 cb 4b ed 39 4b 9c c4 3f d2 55 13 6e 24 b0 d6 71 fa f4 c1 ba cc ed 1b f5 fe 81 41 d8 00 98 3d 3a c8 ae 7a 98 37 18 05 95 02 03 01 00 01 a3 82 01 38 30 82 01 34 30 0e 06 03 55 1d 0f 01 01 ff 04 04 03 02 01 86 30 0f 06 03 55 1d 13 01 01 ff 04 05 30 03 01 01 ff 30 1d 06 03 55 1d 0e 04 16 04 14 e4 af 2b 26 71 1a 2b 48 27 85 2f 52 66 2c ef f0 89 13 71 3e 30 1f 06 03 55 1d 23 04 18 30 16 80 14 60 7b 66 1a 45 0d 97 ca 89 50 2f 7d 04 cd 34 a8 ff fc fd 4b 30 60 06 08 2b 06 01 05 05 07 01 01 04 54 30 52 30 25 06 08 2b 06 01 05 05 07 30 01 86 19 68 74 74 70 3a 2f 2f 6f 63 73 70 2e 70 6b 69 2e 67 6f 6f 67 2f 67 73 72 31 30 29 06 08 2b 06 01 05 05 07 30 02 86 1d 68 74 74 70 3a 2f 2f 70 6b 69 2e 67 6f 6f 67 2f 67 73 72 31 2f 67 73 72 31 2e 63 72 74 30 32 06 03 55 1d 1f 04 2b 30 29 30 27 a0 25 a0 23 86 21 68 74 74 70 3a 2f 2f 63 72 6c 2e 70 6b 69 2e 67 6f 6f 67 2f 67 73 72 31 2f 67 73 72 31 2e 63 72 6c 30 3b 06 03 55 1d 20 04 34 30 32 30 08 06 06 67 81 0c 01 02 01 30 08 06 06 67 81 0c 01 02 02 30 0d 06 0b 2b 06 01 04 01 d6 79 02 05 03 02 30 0d 06 0b 2b 06 01 04 01 d6 79 02 05 03 03 30 0d 06 09 2a 86 48 86 f7 0d 01 01 0b 05 00 03 82 01 01 00 34 a4 1e b1 28 a3 d0 b4 76 17 a6 31 7a 21 e9 d1 52 3e c8 db 74 16 41 88 b8 3d 35 1d ed e4 ff 93 e1 5c 5f ab bb ea 7c cf db e4 0d d1 8b 57 f2 26 6f 5b be 17 46 68 94 37 6f 6b 7a c8 c0 18 37 fa 25 51 ac ec 68 bf b2 c8 49 fd 5a 9a ca 01 23 ac 84 80 2b 02 8c 99 97 eb 49 6a 8c 75 d7 c7 de b2 c9 97 9f 58 48 57 0e 35 a1 e4 1a d6 fd 6f 83 81 6f ef 8c cf 97 af c0 85 2a f0 f5 4e 69 09 91 2d e1 68 b8 c1 2b 73 e9 d4 d9 fc 22 c0 37 1f 0b 66 1d 49 ed 02 55 8f 67 e1 32 d7 d3 26 bf 70 e3 3d f4 67 6d 3d 7c e5 34 88 e3 32 fa a7 6e 06 6a 6f bd 8b 91 ee 16 4b e8 3b a9 b3 37 e7 c3 44 a4 7e d8 6c d7 c7 46 f5 92 9b e7 d5 21 be 66 92 19 94 55 6c d4 29 b2 0d c1 66 5b e2 77 49 48 28 ed 9d d7 1a 33 72 53 b3 82 35 cf 62 8b c9 24 8b a5 b7 39 0c bb 7e 2a 41 bf 52 cf fc a2 96 b6 c2 82 3f 

		16 03 03 01 2c 
		0c 00 01 28 
		03 00 1d 
		20 60 05 ef 77 5e 41 b8 21 ac eb c0 26 cc cb 38 cd d4 ec 1c 8b 85 72 c4 2c 50 73 d2 83 de 68 5e 01 
		05 01 01 00 20 a8 f5 73 4f 7c 3b 5c 7f 74 2f 1b 19 79 6b 8a b1 ba b5 8b 83 d5 33 c7 03 c0 fd 4e 7d 43 7e 0d 58 fb 9d 09 7f 05 a6 15 ce 69 31 2b 98 38 22 0c e5 1a d4 cb 91 eb 57 70 45 64 c6 b6 2b d0 79 0c ba f0 a1 63 7e ea ac 6f ae 6c 4a f2 a7 01 a3 60 f4 c0 44 6d 1c 2c 07 23 c4 70 71 48 36 83 43 76 01 c4 76 cc 87 77 66 40 2d e2 29 66 01 ef 04 4c b1 a1 bb 3c 64 61 e8 d4 09 34 aa 74 16 72 d3 96 56 86 9a a0 20 5a d0 ef cd b6 36 83 d7 76 61 f2 f0 87 fd a5 91 60 66 17 fa ed fb 36 21 a0 62 62 40 bc 33 52 9e 03 d1 ca ff 6e 91 21 3e b1 0a eb ef df 31 a0 8e a5 85 f0 e9 37 67 4c 4c 14 22 40 7a 5e 1d f1 42 7b f0 bd f5 1a b1 7c da c6 d9 c7 18 e1 d6 eb fe e5 a9 26 75 bb 3a 42 f5 19 f4 1f 0e 82 78 f7 85 7d 3a 29 e2 ba be 89 59 bd bc 0f 6b 28 d4 80 00 15 f3 17 b2 47 bd 4f d3 32 80 a2 
		
		16 03 03 00 04 
		0e 00 00 00 
		)";
		tlsPlaintext.str("");
		tlsPlaintext
			<< "\x16\x03\x03\x00\x25"s
			<< "\x10\x00\x00\x21"s
			<< "\x20\x35\x80\x72\xd6\x36\x58\x80\xd1\xae\xea\x32\x9a\xdf\x91\x21\x38\x38\x51\xed\x21\xa2\x8e\x3b\x75\xe9\x65\xd0\xd2\xcd\x16\x62\x54"s	// Public key.

			<< "\x14\x03\x03\x00\x01"s
			<< "\x01"s

			// ...
			;

		client.send(tlsPlaintext.str());
		std::this_thread::sleep_for(2s);

		client.recv(buffer);
		std::cout << buffer.length() << std::endl << std::hex << std::setfill('0');
		for (char const &c : buffer) {
			std::cout << std::setw(2)
								<< static_cast<int>(static_cast<unsigned char>(c)) << ' ';
		}
		std::cout << std::endl;
	}
	return 0;
}

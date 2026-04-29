#include "rain/networking/client.hpp"
#include "rain/networking/resolve.hpp"
#include <rain.hpp>

// <https://tls12.xargs.org/#client-hello/annotated>.

using namespace Rain;
using Networking::Smtp::Command;
using Networking::Smtp::StatusCode;
using namespace std;

class Client :
	public Networking::Smtp::Client<
		Networking::Smtp::Request,
		Networking::Smtp::Response,
		1 << 10,
		1 << 10,
		60000,
		60000,
		Networking::Ipv4FamilyInterface,
		Networking::StreamTypeInterface,
		Networking::TcpProtocolInterface,
		Networking::NoLingerSocketOption> {
	private:
	using SuperClient = Networking::Smtp::Client<
		Networking::Smtp::Request,
		Networking::Smtp::Response,
		1 << 10,
		1 << 10,
		60000,
		60000,
		Networking::Ipv4FamilyInterface,
		Networking::StreamTypeInterface,
		Networking::TcpProtocolInterface,
		Networking::NoLingerSocketOption>;

	public:
	Client(
		std::vector<std::pair<std::size_t, std::string>> const
			&mxRecords,
		std::size_t port) :
		SuperClient(mxRecords, port) {}

	using SuperClient::send;
	using SuperClient::recv;
	virtual void send(Request &req) override {
		std::cout << "SMTP to " << this->peerHost() << ":\n"
							<< req << std::endl;
		SuperClient::send(req);
	}
	virtual Client::Response &recv(Response &res) override {
		SuperClient::recv(res);
		std::cout << "SMTP from " << this->peerHost() << ":\n"
							<< res << std::endl;
		return res;
	}
};

string hexStrToStr(string const &hex) {
	string r;
	for (size_t i{0}; i < hex.length(); i += 3) {
		r.push_back(
			(char)strtol(hex.substr(i, 2).c_str(), NULL, 16));
	}
	return r;
}

void showHexStr(string &res) {
	cout << "[" << hex << setw(4) << setfill('0')
			 << res.length() << "] ";
	for (auto &i : res) {
		cout << hex << setw(2) << (int)(uint8_t)i << " ";
	}
	cout << endl;
}

optional<Networking::Smtp::Response> sendMail() {
	auto records{Networking::getMxRecords("gmail.com")};
	for (auto &i : records) {
		cout << i.first << " " << i.second << endl;
	}
	Client client(records, 25);
	cout << "Connected!" << endl;

	{
		auto res = client.recv();
		if (res.statusCode != StatusCode::SERVICE_READY) {
			return res;
		}
	}
	{
		client.send({Command::EHLO, "gilgamesh.cc"});
		auto res = client.recv();
		if (res.statusCode != StatusCode::REQUEST_COMPLETED) {
			return res;
		}
	}
	{
		client.send("STARTTLS\r\n");
		auto res = client.recv();
		if (res.statusCode != StatusCode::SERVICE_READY) {
			return res;
		}
	}
	{
		string req{hexStrToStr(
			// // HANDSHAKE RECORD: TLS 1.0, DATALEN (47).
			// "16 03 01 00 31 "
			// // HANDSHAKE HEADER: CLIENT HELLO, DATALEN (43).
			// "01 00 00 2d "
			// // TLS 1.2.
			// "03 03 "
			// // 32 BYTES OF RANDOM DATA.
			// "00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 "
			// "00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 "
			// // SESSION ID.
			// "00 "
			// // DATALEN (2), CIPHERSUITES
			// (TLS_RSA_WITH_NULL_MD5). "00 04 00 01 c0 0a "
			// // DATALEN (2), COMPRESSION METHODS.
			// "01 00 "
			// // EXTENSIONS LEN.
			// "00 00 ")},
			// "16 03 01 00 87 "
			// "01 00 00 83 "
			// "03 03 00 01 02 03 04 05 "
			// "06 07 08 09 0a 0b 0c 0d 0e 0f 10 11 12 13 14 15 16
			// " "17 18 19 1a 1b 1c 1d 1e 1f 00 "
			// // RSA_WITH_AES_256_CBC_SHA.
			// "00 02 00 35 "
			// "01 00 "
			// "00 58 "
			// "00 00 00 "
			// "18 00 16 00 00 13 65 78 61 6d 70 6c 65 2e 75 6c 66
			// " "68 65 69 6d 2e 6e 65 74 00 05 00 05 01 00 00 00
			// 00 " "00 0a 00 0a 00 08 00 1d 00 17 00 18 00 19 00
			// 0b 00 " "02 01 00 00 0d 00 12 00 10 04 01 04 03 05
			// 01 05 03 " "06 01 06 03 02 01 02 03 ff 01 00 01 00
			// 00 12 00 " "00 ")},
			"16 03 01 00 2f "
			"01 00 00 2b "
			"03 03 "
			"00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 "
			"00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 "
			"00 "
			// TLS_RSA_WITH_AES_256_CBC_SHA. WORKS
			// TLS_RSA_WITH_AES_128_CBC_SHA. WORKS (002f)
			"00 02 00 2f "
			"01 00 "
			"00 00 ")},
			res, t;
		client.send(req);
		showHexStr(req);
		do {
			client.recv(t, 3s);
			res += t;
		} while (!t.empty());
		showHexStr(res);
		// <https://datatracker.ietf.org/doc/html/rfc5246#appendix-A.3>.
		// [0007] 15 03 01 00 02 02 32 :
		// ALERT RECORD, FATAL, DECODE_ERROR.
		// [0007] 15 03 03 00 02 02 28 :
		// ALERT RECORD, FATAL, HANDSHAKE_FAILURE. Usually
		// because cipher negotation failed (?)
		// [1279] 16 03 03 00 2a
		// HANDSHAKE RECORD, TLS 1.2, DATALEN (42)
		// 02 00 00 26
		// SERVER HELLO, DATALEN (38)
		// 03 03
		// SERVER VERSION (TLS 1.2)
		// 69 f1 9e c3 55 ed 9d 2f c9 65 d0 cd ea 71 8a 82
		// 7b f1 d7 7a 02 85 a4 9f 44 4f 57 4e 47 52 44 01
		// SERVER RANDOM
		// 00
		// SESSION ID
		// 00 35
		// CIPHER SUITE (RSA_WITH_AES_256_CBC_SHA)
		// 00
		// COMPRESSION
		// NO EXTENSIONS?
		// 16 03 03 12 1a
		// HANDSHAKE RECORD, TLS 1.2, 4634
		// 0b 00 12 16
		// CERTIFICATE TYPE, 4630
		// 00 12 13 00 07 95 30 82 07 91 30 82 06
		// 79 a0 03 02 01 02 02 11 00 8b a9 d7 a7 25 00 5e 23 10
		// d2 b8 53 1c fc 35 bd 30 0d 06 09 2a 86 48 86 f7 0d 01
		// 01 0b 05 00 30 3b 31 0b 30 09 06 03 55 04 06 13 02 55
		// 53 31 1e 30 1c 06 03 55 04 0a 13 15 47 6f 6f 67 6c 65
		// 20 54 72 75 73 74 20 53 65 72 76 69 63 65 73 31 0c 30
		// 0a 06 03 55 04 03 13 03 57 52 32 30 1e 17 0d 32 36 30
		// 33 33 30 30 38 33 36 34 36 5a 17 0d 32 36 30 36 32 32
		// 30 38 33 36 34 35 5a 30 18 31 16 30 14 06 03 55 04 03
		// 13 0d 6d 78 2e 67 6f 6f 67 6c 65 2e 63 6f 6d 30 82 01
		// 22 30 0d 06 09 2a 86 48 86 f7 0d 01 01 01 05 00 03 82
		// 01 0f 00 30 82 01 0a 02 82 01 01 00 d3 96 eb 28 f6 04
		// f0 d5 9b a8 0d 47 6c b8 60 39 a7 d0 36 08 41 c4 52 9e
		// 20 26 9b 9e c2 26 f8 b9 16 31 17 23 a5 0c 8e 59 57 c7
		// 34 0b d4 95 89 18 ae d4 06 9f 18 05 d4 ef 49 2c f3 95
		// 98 25 6d 0c 11 3d e4 04 d7 b6 0f 3c f8 ab c8 c7 4e 60
		// 6b ac 94 9c f9 10 91 5d 90 8d 0c 54 bc f7 a3 e2 49 f0
		// d3 53 d9 83 48 b1 16 b6 6e 0c 3b 45 46 fe f6 51 11 e5
		// 47 77 c0 74 7c 2a 0b 0b 40 fa a2 fd 94 46 54 0a 3f 9b
		// cc 67 1e 36 a4 95 b1 2f 6d b9 00 e7 1b 6d fc c7 d1 44
		// c3 33 6c 3f 5b 20 92 a4 c9 7c 5a 77 33 57 b1 5e c6 53
		// 19 4d 58 30 91 a7 df ef 61 b0 ee 7d bf f2 29 cd b5 fc
		// 65 c4 e3 57 ea 78 d9 e4 06 9e 50 e5 dd 57 8c 0f 00 4a
		// 4d d3 7c 14 fa cb d4 24 ab b5 fd 4c 62 70 a5 3f 2e 68
		// e9 7e c9 86 be db 30 98 6e d9 3d 8c 4b 1c c5 e8 53 5f
		// 07 b0 90 0e 7e ec 4e 08 9e c7 e9 79 cd eb 36 05 02 03
		// 01 00 01 a3 82 04 b1 30 82 04 ad 30 0e 06 03 55 1d 0f
		// 01 01 ff 04 04 03 02 05 a0 30 13 06 03 55 1d 25 04 0c
		// 30 0a 06 08 2b 06 01 05 05 07 03 01 30 0c 06 03 55 1d
		// 13 01 01 ff 04 02 30 00 30 1d 06 03 55 1d 0e 04 16 04
		// 14 d4 14 ed 6a 35 09 fc eb 38 1a ec a0 e1 7a 6b 9c 82
		// 91 71 13 30 1f 06 03 55 1d 23 04 18 30 16 80 14 de 1b
		// 1e ed 79 15 d4 3e 37 24 c3 21 bb ec 34 39 6d 42 b2 30
		// 30 58 06 08 2b 06 01 05 05 07 01 01 04 4c 30 4a 30 21
		// 06 08 2b 06 01 05 05 07 30 01 86 15 68 74 74 70 3a 2f
		// 2f 6f 2e 70 6b 69 2e 67 6f 6f 67 2f 77 72 32 30 25 06
		// 08 2b 06 01 05 05 07 30 02 86 19 68 74 74 70 3a 2f 2f
		// 69 2e 70 6b 69 2e 67 6f 6f 67 2f 77 72 32 2e 63 72 74
		// 30 82 02 86 06 03 55 1d 11 04 82 02 7d 30 82 02 79 82
		// 0d 6d 78 2e 67 6f 6f 67 6c 65 2e 63 6f 6d 82 0f 73 6d
		// 74 70 2e 67 6f 6f 67 6c 65 2e 63 6f 6d 82 12 61 73 70
		// 6d 78 2e 6c 2e 67 6f 6f 67 6c 65 2e 63 6f 6d 82 17 61
		// 6c 74 31 2e 61 73 70 6d 78 2e 6c 2e 67 6f 6f 67 6c 65
		// 2e 63 6f 6d 82 17 61 6c 74 32 2e 61 73 70 6d 78 2e 6c
		// 2e 67 6f 6f 67 6c 65 2e 63 6f 6d 82 17 61 6c 74 33 2e
		// 61 73 70 6d 78 2e 6c 2e 67 6f 6f 67 6c 65 2e 63 6f 6d
		// 82 17 61 6c 74 34 2e 61 73 70 6d 78 2e 6c 2e 67 6f 6f
		// 67 6c 65 2e 63 6f 6d 82 1a 67 6d 61 69 6c 2d 73 6d 74
		// 70 2d 69 6e 2e 6c 2e 67 6f 6f 67 6c 65 2e 63 6f 6d 82
		// 1f 61 6c 74 31 2e 67 6d 61 69 6c 2d 73 6d 74 70 2d 69
		// 6e 2e 6c 2e 67 6f 6f 67 6c 65 2e 63 6f 6d 82 1f 61 6c
		// 74 32 2e 67 6d 61 69 6c 2d 73 6d 74 70 2d 69 6e 2e 6c
		// 2e 67 6f 6f 67 6c 65 2e 63 6f 6d 82 1f 61 6c 74 33 2e
		// 67 6d 61 69 6c 2d 73 6d 74 70 2d 69 6e 2e 6c 2e 67 6f
		// 6f 67 6c 65 2e 63 6f 6d 82 1f 61 6c 74 34 2e 67 6d 61
		// 69 6c 2d 73 6d 74 70 2d 69 6e 2e 6c 2e 67 6f 6f 67 6c
		// 65 2e 63 6f 6d 82 18 67 6d 72 2d 73 6d 74 70 2d 69 6e
		// 2e 6c 2e 67 6f 6f 67 6c 65 2e 63 6f 6d 82 1d 61 6c 74
		// 31 2e 67 6d 72 2d 73 6d 74 70 2d 69 6e 2e 6c 2e 67 6f
		// 6f 67 6c 65 2e 63 6f 6d 82 1d 61 6c 74 32 2e 67 6d 72
		// 2d 73 6d 74 70 2d 69 6e 2e 6c 2e 67 6f 6f 67 6c 65 2e
		// 63 6f 6d 82 1d 61 6c 74 33 2e 67 6d 72 2d 73 6d 74 70
		// 2d 69 6e 2e 6c 2e 67 6f 6f 67 6c 65 2e 63 6f 6d 82 1d
		// 61 6c 74 34 2e 67 6d 72 2d 73 6d 74 70 2d 69 6e 2e 6c
		// 2e 67 6f 6f 67 6c 65 2e 63 6f 6d 82 0d 6d 78 31 2e 73
		// 6d 74 70 2e 67 6f 6f 67 82 0d 6d 78 32 2e 73 6d 74 70
		// 2e 67 6f 6f 67 82 0d 6d 78 33 2e 73 6d 74 70 2e 67 6f
		// 6f 67 82 0d 6d 78 34 2e 73 6d 74 70 2e 67 6f 6f 67 82
		// 15 61 73 70 6d 78 32 2e 67 6f 6f 67 6c 65 6d 61 69 6c
		// 2e 63 6f 6d 82 15 61 73 70 6d 78 33 2e 67 6f 6f 67 6c
		// 65 6d 61 69 6c 2e 63 6f 6d 82 15 61 73 70 6d 78 34 2e
		// 67 6f 6f 67 6c 65 6d 61 69 6c 2e 63 6f 6d 82 15 61 73
		// 70 6d 78 35 2e 67 6f 6f 67 6c 65 6d 61 69 6c 2e 63 6f
		// 6d 82 11 67 6d 72 2d 6d 78 2e 67 6f 6f 67 6c 65 2e 63
		// 6f 6d 30 13 06 03 55 1d 20 04 0c 30 0a 30 08 06 06 67
		// 81 0c 01 02 01 30 36 06 03 55 1d 1f 04 2f 30 2d 30 2b
		// a0 29 a0 27 86 25 68 74 74 70 3a 2f 2f 63 2e 70 6b 69
		// 2e 67 6f 6f 67 2f 77 72 32 2f 37 35 72 34 5a 79 41 33
		// 76 41 30 2e 63 72 6c 30 82 01 05 06 0a 2b 06 01 04 01
		// d6 79 02 04 02 04 81 f6 04 81 f3 00 f1 00 77 00 0e 57
		// 94 bc f3 ae a9 3e 33 1b 2c 99 07 b3 f7 90 df 9b c2 3d
		// 71 32 25 dd 21 a9 25 ac 61 c5 4e 21 00 00 01 9d 3e 1a
		// 58 95 00 00 04 03 00 48 30 46 02 21 00 cf 91 10 34 4d
		// 7b 1e a3 71 f1 c4 bc e2 25 8a 84 c4 f2 5d 5d 00 3e de
		// fa 75 c1 8e 29 3e eb ee 68 02 21 00 aa db a1 b5 b0 4f
		// 67 52 d0 e6 59 48 05 6a a0 64 2a 62 9e 5a 91 12 29 5c
		// 27 6f 65 95 a3 83 e5 e7 00 76 00 16 83 2d ab f0 a9 25
		// 0f 0f f0 3a a5 45 ff c8 bf c8 23 d0 87 4b f6 04 29 27
		// f8 e7 1f 33 13 f5 fa 00 00 01 9d 3e 1a 5c 93 00 00 04
		// 03 00 47 30 45 02 20 7c c2 df 42 1e b4 af fd 34 e0 de
		// 27 69 19 68 07 87 03 44 4d ce bb 40 ed b2 de 3b fc 91
		// 45 e1 7e 02 21 00 b6 b9 ad 7c 43 00 02 0c 4a 4a 17 41
		// 55 14 9f d2 cd 21 34 0e bc 86 07 2b 7c 5f 29 d1 32 93
		// a0 a6 30 0d 06 09 2a 86 48 86 f7 0d 01 01 0b 05 00 03
		// 82 01 01 00 80 ff dc ab 2a a1 9d 1b 5c 1a 4d 0e c2 b9
		// a5 90 67 fe a6 aa fc e9 4e c2 87 32 2a 4a 7f 97 37 40
		// c7 aa af 68 bd 90 e5 1f 5c 7c 78 59 6a dd fe 51 93 31
		// fb b2 39 17 7e 56 3b e1 cc 4b 32 d4 a8 54 1f c2 b2 79
		// 81 3d eb c1 a6 2c 6b df 3c da ba 30 b5 04 8d be fe d6
		// 39 8f c4 55 f9 5b 35 17 95 6b b0 73 68 7a 69 26 95 9e
		// 41 f8 0e f2 3d 7d 6f e4 7d aa c4 89 83 42 1a ef 8a b0
		// 32 02 8f 92 4d 8a 16 a5 ba 9b 2e b2 a9 9c 8a e8 44 3c
		// d5 6f 70 f2 bc 3b 6d e8 19 21 fd 1b b6 07 5a ba 03 7e
		// 19 ca ba e2 db b9 ad e3 2c ce ac 74 5b 67 2f a3 58 86
		// 9b de bc d9 e6 39 d9 57 80 79 a4 eb db 7d 0a fe 30 1a
		// bc d0 36 90 1b f7 7d a6 2a 69 7a 16 27 32 01 79 1b d9
		// e0 31 93 da bf da 68 4c 3e cf 30 89 72 98 09 b4 33 71
		// cb 14 b7 9f ce 84 1c db 61 47 26 8d 85 77 e3 3c c7 e4
		// 01 78 42 24 fe 9f f8 f6 00 05 0f 30 82 05 0b 30 82 02
		// f3 a0 03 02 01 02 02 10 7f f0 05 a0 7c 4c de d1 00 ad
		// 9d 66 a5 10 7b 98 30 0d 06 09 2a 86 48 86 f7 0d 01 01
		// 0b 05 00 30 47 31 0b 30 09 06 03 55 04 06 13 02 55 53
		// 31 22 30 20 06 03 55 04 0a 13 19 47 6f 6f 67 6c 65 20
		// 54 72 75 73 74 20 53 65 72 76 69 63 65 73 20 4c 4c 43
		// 31 14 30 12 06 03 55 04 03 13 0b 47 54 53 20 52 6f 6f
		// 74 20 52 31 30 1e 17 0d 32 33 31 32 31 33 30 39 30 30
		// 30 30 5a 17 0d 32 39 30 32 32 30 31 34 30 30 30 30 5a
		// 30 3b 31 0b 30 09 06 03 55 04 06 13 02 55 53 31 1e 30
		// 1c 06 03 55 04 0a 13 15 47 6f 6f 67 6c 65 20 54 72 75
		// 73 74 20 53 65 72 76 69 63 65 73 31 0c 30 0a 06 03 55
		// 04 03 13 03 57 52 32 30 82 01 22 30 0d 06 09 2a 86 48
		// 86 f7 0d 01 01 01 05 00 03 82 01 0f 00 30 82 01 0a 02
		// 82 01 01 00 a9 ff 9c 7f 45 1e 70 a8 53 9f ca d9 e5 0d
		// de 46 57 57 7d bc 8f 9a 5a ac 46 f1 84 9a bb 91 db c9
		// fb 2f 01 fb 92 09 00 16 5e a0 1c f8 c1 ab f9 78 2f 4a
		// cc d8 85 a2 d8 59 3c 0e d3 18 fb b1 f5 24 0d 26 ee b6
		// 5b 64 76 7c 14 c7 2f 7a ce a8 4c b7 f4 d9 08 fc df 87
		// 23 35 20 a8 e2 69 e2 8c 4e 3f b1 59 fa 60 a2 1e b3 c9
		// 20 53 19 82 ca 36 53 6d 60 4d e9 00 91 fc 76 8d 5c 08
		// 0f 0a c2 dc f1 73 6b c5 13 6e 0a 4f 7a c2 f2 02 1c 2e
		// b4 63 83 da 31 f6 2d 75 30 b2 fb ab c2 6e db a9 c0 0e
		// b9 f9 67 d4 c3 25 57 74 eb 05 b4 e9 8e b5 de 28 cd cc
		// 7a 14 e4 71 03 cb 4d 61 2e 61 57 c5 19 a9 0b 98 84 1a
		// e8 79 29 d9 b2 8d 2f ff 57 6a 66 e0 ce ab 95 a8 29 96
		// 63 70 12 67 1e 3a e1 db b0 21 71 d7 7c 9e fd aa 17 6e
		// fe 2b fb 38 17 14 d1 66 a7 af 9a b5 70 cc c8 63 81 3a
		// 8c c0 2a a9 76 37 ce e3 02 03 01 00 01 a3 81 fe 30 81
		// fb 30 0e 06 03 55 1d 0f 01 01 ff 04 04 03 02 01 86 30
		// 1d 06 03 55 1d 25 04 16 30 14 06 08 2b 06 01 05 05 07
		// 03 01 06 08 2b 06 01 05 05 07 03 02 30 12 06 03 55 1d
		// 13 01 01 ff 04 08 30 06 01 01 ff 02 01 00 30 1d 06 03
		// 55 1d 0e 04 16 04 14 de 1b 1e ed 79 15 d4 3e 37 24 c3
		// 21 bb ec 34 39 6d 42 b2 30 30 1f 06 03 55 1d 23 04 18
		// 30 16 80 14 e4 af 2b 26 71 1a 2b 48 27 85 2f 52 66 2c
		// ef f0 89 13 71 3e 30 34 06 08 2b 06 01 05 05 07 01 01
		// 04 28 30 26 30 24 06 08 2b 06 01 05 05 07 30 02 86 18
		// 68 74 74 70 3a 2f 2f 69 2e 70 6b 69 2e 67 6f 6f 67 2f
		// 72 31 2e 63 72 74 30 2b 06 03 55 1d 1f 04 24 30 22 30
		// 20 a0 1e a0 1c 86 1a 68 74 74 70 3a 2f 2f 63 2e 70 6b
		// 69 2e 67 6f 6f 67 2f 72 2f 72 31 2e 63 72 6c 30 13 06
		// 03 55 1d 20 04 0c 30 0a 30 08 06 06 67 81 0c 01 02 01
		// 30 0d 06 09 2a 86 48 86 f7 0d 01 01 0b 05 00 03 82 02
		// 01 00 45 75 8b e5 1f 3b 44 13 96 1a ab 58 f1 35 c9 6f
		// 3d d2 d0 33 4a 86 33 ba 57 51 4f ee c4 34 da 16 12 4c
		// bf 13 9f 0d d4 54 e9 48 79 c0 30 3c 94 25 f2 1a f4 ba
		// 32 94 b6 33 72 0b 85 ee 09 11 25 34 94 e1 6f 42 db 82
		// 9b 7b 7f 2a 9a a9 ff 7f a9 d2 de 4a 20 cb b3 fb 03 03
		// b8 f8 07 05 da 59 92 2f 18 46 98 ce af 72 be 24 26 b1
		// 1e 00 4d bd 08 ad 93 41 44 0a bb c7 d5 01 85 bf 93 57
		// e3 df 74 12 53 0e 11 25 d3 9b dc de cb 27 6e b3 c2 b9
		// 33 62 39 c2 e0 35 e1 5b a7 09 2e 19 cb 91 2a 76 5c f1
		// df ca 23 84 40 a5 6f ff 9a 41 e0 b5 ef 32 d1 85 ae af
		// 25 09 f0 62 c5 6e c2 c8 6e 32 fd b8 da e2 ce 4a 91 4a
		// f3 85 55 4e b1 75 d6 48 33 2f 6f 84 d9 12 5c 9f d4 71
		// 98 63 25 8d 69 5c 0a 6b 7d f2 41 bd e8 bb 8f e4 22 d7
		// 9d 65 45 e8 4c 0a 87 da e9 60 66 88 0e 1f c7 e1 4e 56
		// c5 76 ff b4 7a 57 69 f2 02 22 09 26 41 1d da 74 a2 e5
		// 29 f3 c4 9a e5 5d d6 aa 7a fd e1 b7 2b 66 38 fb e8 29
		// 66 ba ef a0 13 2f f8 73 7e f0 da 40 11 1c 5d dd 8f a6
		// fc be db be 56 f8 32 9c 1f 41 41 6d 7e b6 c5 eb c6 8b
		// 36 b7 17 8c 9d cf 19 7a 34 9f 21 93 c4 7e 74 35 d2 aa
		// fd 4c 6d 14 f5 c9 b0 79 5b 49 3c f3 bf 17 48 e8 ef 9a
		// 26 13 0c 87 f2 73 d6 9c c5 52 6b 63 f7 32 90 78 a9 6b
		// eb 5e d6 93 a1 bf bc 18 3d 8b 59 f6 8a c6 05 5e 52 18
		// e2 66 e0 da c1 dc ad 5a 25 aa f4 45 fc f1 0b 78 a4 af
		// b0 f2 73 a4 30 a8 34 c1 53 7f 42 96 e5 48 41 eb 90 46
		// 0c 06 dc cb 92 c6 5e f3 44 44 43 46 29 46 a0 a6 fc b9
		// 8e 39 27 39 b1 5a e2 b1 ad fc 13 ff 8e fc 26 e1 d4 fe
		// 84 f1 50 5a 8e 97 6b 2d 2a 79 fb 40 64 ea f3 3d bd 5b
		// e1 a0 04 b0 97 48 1c 42 f5 ea 5a 1c cd 26 c8 51 ff 14
		// 99 67 89 72 5f 1d ec ad 5a dd 00 05 66 30 82 05 62 30
		// 82 04 4a a0 03 02 01 02 02 10 77 bd 0d 6c db 36 f9 1a
		// ea 21 0f c4 f0 58 d3 0d 30 0d 06 09 2a 86 48 86 f7 0d
		// 01 01 0b 05 00 30 57 31 0b 30 09 06 03 55 04 06 13 02
		// 42 45 31 19 30 17 06 03 55 04 0a 13 10 47 6c 6f 62 61
		// 6c 53 69 67 6e 20 6e 76 2d 73 61 31 10 30 0e 06 03 55
		// 04 0b 13 07 52 6f 6f 74 20 43 41 31 1b 30 19 06 03 55
		// 04 03 13 12 47 6c 6f 62 61 6c 53 69 67 6e 20 52 6f 6f
		// 74 20 43 41 30 1e 17 0d 32 30 30 36 31 39 30 30 30 30
		// 34 32 5a 17 0d 32 38 30 31 32 38 30 30 30 30 34 32 5a
		// 30 47 31 0b 30 09 06 03 55 04 06 13 02 55 53 31 22 30
		// 20 06 03 55 04 0a 13 19 47 6f 6f 67 6c 65 20 54 72 75
		// 73 74 20 53 65 72 76 69 63 65 73 20 4c 4c 43 31 14 30
		// 12 06 03 55 04 03 13 0b 47 54 53 20 52 6f 6f 74 20 52
		// 31 30 82 02 22 30 0d 06 09 2a 86 48 86 f7 0d 01 01 01
		// 05 00 03 82 02 0f 00 30 82 02 0a 02 82 02 01 00 b6 11
		// 02 8b 1e e3 a1 77 9b 3b dc bf 94 3e b7 95 a7 40 3c a1
		// fd 82 f9 7d 32 06 82 71 f6 f6 8c 7f fb e8 db bc 6a 2e
		// 97 97 a3 8c 4b f9 2b f6 b1 f9 ce 84 1d b1 f9 c5 97 de
		// ef b9 f2 a3 e9 bc 12 89 5e a7 aa 52 ab f8 23 27 cb a4
		// b1 9c 63 db d7 99 7e f0 0a 5e eb 68 a6 f4 c6 5a 47 0d
		// 4d 10 33 e3 4e b1 13 a3 c8 18 6c 4b ec fc 09 90 df 9d
		// 64 29 25 23 07 a1 b4 d2 3d 2e 60 e0 cf d2 09 87 bb cd
		// 48 f0 4d c2 c2 7a 88 8a bb ba cf 59 19 d6 af 8f b0 07
		// b0 9e 31 f1 82 c1 c0 df 2e a6 6d 6c 19 0e b5 d8 7e 26
		// 1a 45 03 3d b0 79 a4 94 28 ad 0f 7f 26 e5 a8 08 fe 96
		// e8 3c 68 94 53 ee 83 3a 88 2b 15 96 09 b2 e0 7a 8c 2e
		// 75 d6 9c eb a7 56 64 8f 96 4f 68 ae 3d 97 c2 84 8f c0
		// bc 40 c0 0b 5c bd f6 87 b3 35 6c ac 18 50 7f 84 e0 4c
		// cd 92 d3 20 e9 33 bc 52 99 af 32 b5 29 b3 25 2a b4 48
		// f9 72 e1 ca 64 f7 e6 82 10 8d e8 9d c2 8a 88 fa 38 66
		// 8a fc 63 f9 01 f9 78 fd 7b 5c 77 fa 76 87 fa ec df b1
		// 0e 79 95 57 b4 bd 26 ef d6 01 d1 eb 16 0a bb 8e 0b b5
		// c5 c5 8a 55 ab d3 ac ea 91 4b 29 cc 19 a4 32 25 4e 2a
		// f1 65 44 d0 02 ce aa ce 49 b4 ea 9f 7c 83 b0 40 7b e7
		// 43 ab a7 6c a3 8f 7d 89 81 fa 4c a5 ff d5 8e c3 ce 4b
		// e0 b5 d8 b3 8e 45 cf 76 c0 ed 40 2b fd 53 0f b0 a7 d5
		// 3b 0d b1 8a a2 03 de 31 ad cc 77 ea 6f 7b 3e d6 df 91
		// 22 12 e6 be fa d8 32 fc 10 63 14 51 72 de 5d d6 16 93
		// bd 29 68 33 ef 3a 66 ec 07 8a 26 df 13 d7 57 65 78 27
		// de 5e 49 14 00 a2 00 7f 9a a8 21 b6 a9 b1 95 b0 a5 b9
		// 0d 16 11 da c7 6c 48 3c 40 e0 7e 0d 5a cd 56 3c d1 97
		// 05 b9 cb 4b ed 39 4b 9c c4 3f d2 55 13 6e 24 b0 d6 71
		// fa f4 c1 ba cc ed 1b f5 fe 81 41 d8 00 98 3d 3a c8 ae
		// 7a 98 37 18 05 95 02 03 01 00 01 a3 82 01 38 30 82 01
		// 34 30 0e 06 03 55 1d 0f 01 01 ff 04 04 03 02 01 86 30
		// 0f 06 03 55 1d 13 01 01 ff 04 05 30 03 01 01 ff 30 1d
		// 06 03 55 1d 0e 04 16 04 14 e4 af 2b 26 71 1a 2b 48 27
		// 85 2f 52 66 2c ef f0 89 13 71 3e 30 1f 06 03 55 1d 23
		// 04 18 30 16 80 14 60 7b 66 1a 45 0d 97 ca 89 50 2f 7d
		// 04 cd 34 a8 ff fc fd 4b 30 60 06 08 2b 06 01 05 05 07
		// 01 01 04 54 30 52 30 25 06 08 2b 06 01 05 05 07 30 01
		// 86 19 68 74 74 70 3a 2f 2f 6f 63 73 70 2e 70 6b 69 2e
		// 67 6f 6f 67 2f 67 73 72 31 30 29 06 08 2b 06 01 05 05
		// 07 30 02 86 1d 68 74 74 70 3a 2f 2f 70 6b 69 2e 67 6f
		// 6f 67 2f 67 73 72 31 2f 67 73 72 31 2e 63 72 74 30 32
		// 06 03 55 1d 1f 04 2b 30 29 30 27 a0 25 a0 23 86 21 68
		// 74 74 70 3a 2f 2f 63 72 6c 2e 70 6b 69 2e 67 6f 6f 67
		// 2f 67 73 72 31 2f 67 73 72 31 2e 63 72 6c 30 3b 06 03
		// 55 1d 20 04 34 30 32 30 08 06 06 67 81 0c 01 02 01 30
		// 08 06 06 67 81 0c 01 02 02 30 0d 06 0b 2b 06 01 04 01
		// d6 79 02 05 03 02 30 0d 06 0b 2b 06 01 04 01 d6 79 02
		// 05 03 03 30 0d 06 09 2a 86 48 86 f7 0d 01 01 0b 05 00
		// 03 82 01 01 00 34 a4 1e b1 28 a3 d0 b4 76 17 a6 31 7a
		// 21 e9 d1 52 3e c8 db 74 16 41 88 b8 3d 35 1d ed e4 ff
		// 93 e1 5c 5f ab bb ea 7c cf db e4 0d d1 8b 57 f2 26 6f
		// 5b be 17 46 68 94 37 6f 6b 7a c8 c0 18 37 fa 25 51 ac
		// ec 68 bf b2 c8 49 fd 5a 9a ca 01 23 ac 84 80 2b 02 8c
		// 99 97 eb 49 6a 8c 75 d7 c7 de b2 c9 97 9f 58 48 57 0e
		// 35 a1 e4 1a d6 fd 6f 83 81 6f ef 8c cf 97 af c0 85 2a
		// f0 f5 4e 69 09 91 2d e1 68 b8 c1 2b 73 e9 d4 d9 fc 22
		// c0 37 1f 0b 66 1d 49 ed 02 55 8f 67 e1 32 d7 d3 26 bf
		// 70 e3 3d f4 67 6d 3d 7c e5 34 88 e3 32 fa a7 6e 06 6a
		// 6f bd 8b 91 ee 16 4b e8 3b a9 b3 37 e7 c3 44 a4 7e d8
		// 6c d7 c7 46 f5 92 9b e7 d5 21 be 66 92 19 94 55 6c d4
		// 29 b2 0d c1 66 5b e2 77 49 48 28 ed 9d d7 1a 33 72 53
		// b3 82 35 cf 62 8b c9 24 8b a5 b7 39 0c bb 7e 2a 41 bf
		// 52 cf fc a2 96 b6 c2 82 3f
		// CERTIFICATE
		// 16 03 03 00 1d
		// HANDSHAKE RECORD, TLS 1.2, 29
		// 0d 00 00 19
		// CERTIFICATE_REQUEST (HandshakeType), 25
		// 02
		// DSS_SIGN (ClientCertificateType)
		// 01 40
		// SIGNATUREANDHASHALGORITHM, MD5, ?
		// 00 12
		// ?
		// 04 03 08 04 04 01 05 03 08 05 05 01 08
		// 06 06 01 02 01 00 00
		// ?
		// 16 03 03 00 04 0e 00 00 00
		// SERVER_HELLO_DONE
	}
	// client.send(
	// 	{Command::MAIL,
	// 		"FROM:<" + static_cast<std::string>(from) +
	// ">"});
	// {
	// 	auto res = client.recv();
	// 	if (res.statusCode !=
	// StatusCode::REQUEST_COMPLETED) { 		return res;
	// 	}
	// }
	// client.send(
	// 	{Command::RCPT,
	// 		"TO:<" + static_cast<std::string>(to) + ">"});
	// {
	// 	auto res = client.recv();
	// 	if (res.statusCode !=
	// StatusCode::REQUEST_COMPLETED) { 		return res;
	// 	}
	// }
	// client.send({Command::DATA, ""});
	// {
	// 	auto res = client.recv();
	// 	if (res.statusCode !=
	// StatusCode::START_MAIL_INPUT) { 		return res;
	// 	}
	// }
	// std::ifstream dataFile(envelope.data,
	// std::ios::binary); client << dataFile.rdbuf();
	// client.flush();
	// {
	// 	auto res = client.recv();
	// 	if (res.statusCode !=
	// StatusCode::REQUEST_COMPLETED) { 		return res;
	// 	}
	// }
	// client.send({Command::QUIT, ""});
	// {
	// 	auto res = client.recv();
	// 	if (res.statusCode != StatusCode::SERVICE_CLOSING)
	// { 		return res;
	// 	}
	// }
	Rain::Error::consumeThrowable(
		[&client]() { client.shutdown(); });
	return {};
}

int main() {
	auto res{sendMail()};
	if (res) {
		for (auto &i : res.value().lines) {
			cout << i << endl;
		}
		return -1;
	}
	return 0;
}

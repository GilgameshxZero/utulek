#include <rain.hpp>

using namespace Rain;
using namespace Networking;
using namespace Tls;
using namespace std;

void showHexStr(string const &res) {
	cout << "[" << hex << setw(4) << setfill('0')
			 << res.length() << "]\t";
	for (auto &i : res) {
		cout << hex << setw(2) << (int)(uint8_t)i << "\t";
	}
	cout << dec << setfill(' ');
}

// PP-chain by deriving Plaintext/* classes.
class MyPlaintext : public Plaintext {
	using Plaintext::Plaintext;

	public:
	// PP-chains.
	MyPlaintext(std::istream &stream) : Plaintext(stream) {
		std::stringstream ss;
		Plaintext::sendWith(ss);
		cout << "RECV ";
		showHexStr(ss.str());
		cout << endl;
	}

	virtual void sendWith(ostream &stream) const override {
		std::stringstream ss;
		Plaintext::sendWith(ss);
		cout << "SEND ";
		showHexStr(ss.str());
		cout << endl;
		Plaintext::sendWith(stream);
	}
};

// `openssl s_client -connect tiktok.com:443 -tls1_2 -status
// -msg -debug`.

// This set of ciphersuites is sufficient, any variation is
// due to variations in extension blocks.

/*
std::string ext{
	// Extension: renegotiation info.
	"\xff\x01\x00\x01\x00"s +
	// Extension: server name.
	"\x00\x00\x00\x0f\x00\x0d\x00\x00\x0a\x74\x69\x6b\x74\x6f\x6b\x2e\x63\x6f\x6d"s
+
	// Extension: EC point formats.
	"\x00\x0b\x00\x02\x01\x00"s +
	// Extension: supported groups.
	"\x00\x0a\x00\x0c\x00\x0a\x00\x1d\x00\x17\x00\x1e\x00\x18\x00\x19"s
+
	// ?
	"\x00\x23\x00\x00"s +
	// Extension: status request.
	"\x00\x05\x00\x05\x01\x00\x00\x00\x00"s +
	// ?
	"\x00\x16\x00\x00"s +
	// ?
	"\x00\x17\x00\x00"s +
	// Extension: signature algorithms.
	"\x00\x0d\x00\x2a\x00\x28\x04\x03\x05\x03\x06\x03\x08\x07\x08\x08\x08\x09\x08\x0a\x08\x0b\x08\x04\x08\x05\x08\x06\x04\x01\x05\x01\x06\x01\x03\x03\x03\x01\x03\x02\x04\x02\x05\x02\x06\x02"s};

std::string ext{
	// Extension: renegotiation info.
	"\xff\x01\x00\x01\x00"s +
	// Extension: server name.
	"\x00\x00\x00\x13\x00\x11\x00\x00\x0e\x63\x6f\x64\x65\x66\x6f\x72\x63\x65\x73\x2e\x63\x6f\x6d"s
+
	// Extension: EC point formats.
	"\x00\x0b\x00\x02\x01\x00"s +
	// Extension: supported groups.
	"\x00\x0a\x00\x0c\x00\x0a\x00\x1d\x00\x17\x00\x1e\x00\x18\x00\x19"s
+
	// Extension: status request.
	"\x00\x05\x00\x05\x01\x00\x00\x00\x00"s +
	// Extension: signature algorithms.
	"\x00\x0d\x00\x2a\x00\x28\x04\x03\x05\x03\x06\x03\x08\x07\x08\x08\x08\x09\x08\x0a\x08\x0b\x08\x04\x08\x05\x08\x06\x04\x01\x05\x01\x06\x01\x03\x03\x03\x01\x03\x02\x04\x02\x05\x02\x06\x02"s};


std::uint16_t extensionsLength(ext.size());


Algorithm::writeBytes(
	stream,
	std::uint16_t(ext.size()),
	std::endian::big);
stream.write(ext.c_str(), ext.size());
*/

int main() {
	vector<string> hostNodes{
		{"google.com",
			"wikipedia.org",
			"facebook.com",
			"github.com",
			"youtube.com",
			"bandcamp.com",
			"pixiv.net",
			"myanimelist.net",
			"startpage.com",
			"reddit.com",
			"projecteuler.net",
			"amazon.com",
			"visualstudio.com",
			"twitch.tv",
			"leagueoflegends.com",
			"x.com",
			"discord.com",
			"stackoverflow.com",
			"usaco.training",
			"usaco.org",
			"codeforces.com",
			"tiktok.com",
			"hp.com"}};
	size_t passed{};
	for (auto const &hostNode : hostNodes) {
		cout << hostNode << endl;
		Tls::Client<Http::Client<>> client(
			1024, 1024, 1000, 1000, Host{hostNode, 443});
		client << MyPlaintext(
			ProtocolVersion::_1_0,
			new Handshake(new ClientHello(
				ProtocolVersion::_1_2,
				{{}},
				0,
				{CipherSuite::
						TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
					CipherSuite::
						TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					CipherSuite::TLS_DHE_RSA_WITH_AES_256_GCM_SHA384,
					CipherSuite::
						TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
					CipherSuite::
						TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
					CipherSuite::
						TLS_DHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
					CipherSuite::
						TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,

					CipherSuite::TLS_RSA_WITH_AES_128_CBC_SHA},
				{0},
				{{new Extension::RenegotiationInfo(),
					new Extension::ServerName({hostNode}),
					new Extension::EcPointFormats(),
					new Extension::SupportedGroups(),
					new Extension::StatusRequest(),
					new Extension::SignatureAlgorithms()}})));
		string s;
		while (true) {
			string r;
			client.recv(r, 1s);
			s += r;
			if (r.empty()) {
				break;
			}
		}
		showHexStr(s.substr(0, 96));
		cout << endl;
		stringstream ss;
		ss << s;

		// Replay it back.
		MyPlaintext response(ss);
		if (
			response.fragment->contentType() ==
			ContentType::HANDSHAKE) {
			auto handshake{
				dynamic_cast<Handshake *>(response.fragment.get())};
			if (
				handshake->body->handshakeType() ==
				HandshakeType::SERVER_HELLO) {
				auto serverHello{dynamic_cast<ServerHello *>(
					handshake->body.get())};
				cout << "Cipher: " << hex << setfill('0') << setw(4)
						 << static_cast<uint16_t>(
									serverHello->cipherSuite)
						 << dec << setfill(' ') << endl;
				passed++;
			}
		}
	}
	cout << "Passed " << passed << " of " << hostNodes.size()
			 << "." << endl;
	return 0;
}

#include <rain.hpp>

using namespace Rain;
using namespace Networking;
using namespace Tls;
using namespace std;

void showHexStr(string const &res, ostream &stream) {
	stream << "[" << hex << setw(4) << setfill('0')
				 << res.length() << "]\t";
	for (auto &i : res) {
		stream << hex << setw(2) << (int)(uint8_t)i << "\t";
	}
	stream << dec << setfill(' ');
}

// PP-chain by deriving Plaintext/* classes.
class MyPlaintext : public Plaintext {
	using Plaintext::Plaintext;

	public:
	// PP-chains.
	MyPlaintext(istream &stream) : Plaintext(stream) {
		std::stringstream ss;
		Plaintext::sendWith(ss);
		cout << "RECV ";
		showHexStr(ss.str(), cout);
		cout << endl;
	}

	virtual void sendWith(ostream &stream) const override {
		std::stringstream ss;
		Plaintext::sendWith(ss);
		cout << "SEND ";
		showHexStr(ss.str(), cout);
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
			"hp.com",
			"quora.com",
			"bilibili.com",
			"baidu.com",
			"mihoyo.com",
			"usa.gov",
			"whitehouse.gov",
			"adobe.com",
			"alpaca.markets",
			"archive.org",
			"archlinux.org",
			"armorgames.com",
			"artofproblemsolving.com",
			"chase.com",
			"numer.ai",
			"parsec.app",
			"cs.rin.ru",
			"spoj.com",
			"zoom.us",
			"qq.com",
			"nytimes.com",
			"mitfcu.org",
			"mega.nz",
			"lichess.org",
			"jlist.com",
			"xnxx.com",
			"pornhub.com",
			"nhentai.net",
			"xvideos.com",
			"hellopoetry.com",
			"goodreads.com",
			"ebay.com",
			"deshaw.com",
			"boox.com",
			"atcoder.jp",
			"www.gov.cn",
			"empireblue.com",
			"cses.fi"}};
	atomic_size_t passed{};
	mutex ssm;
	stringstream failedss;
	auto makeTask{[&](string const &hostNode) {
		return [&]() {
			stringstream ss;
			ss << hostNode << endl;
			Tls::Client<Http::Client<>> client(
				Host{hostNode, 443});
			auto plaintext{Plaintext(
				ProtocolVersion::_1_0,
				new Handshake(new ClientHello(
					ProtocolVersion::_1_2,
					{{}},
					0,
					// Just three ciphersuites is enough.
					// Increasing order of perceived difficulty to
					// implement.
					{CipherSuite::TLS_RSA_WITH_AES_128_CBC_SHA,
						CipherSuite::
							TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
						CipherSuite::
							TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384},
					{0},
					{{new Extension::ServerName({hostNode}),
						new Extension::SupportedGroups(),
						new Extension::SignatureAlgorithms()}})))};
			{
				stringstream sss;
				plaintext.sendWith(sss);
				showHexStr(sss.str(), ss);
				ss << endl;
			}
			client << plaintext;
			string s;
			while (true) {
				string r;
				client.recv(r, 2s);
				s += r;
				if (r.empty()) {
					break;
				}
			}
			showHexStr(s.substr(0, 96), ss);
			ss << endl;
			stringstream sss;
			sss << s;

			// Replay it back.
			Plaintext response(sss);
			{
				stringstream ssss;
				response.sendWith(ssss);
				showHexStr(ssss.str(), ss);
				ss << endl;
			}

			bool pass{false};
			if (
				response.fragment->contentType() ==
				ContentType::HANDSHAKE) {
				auto handshake{dynamic_cast<Handshake *>(
					response.fragment.get())};
				if (
					handshake->body->handshakeType() ==
					HandshakeType::SERVER_HELLO) {
					auto serverHello{dynamic_cast<ServerHello *>(
						handshake->body.get())};
					ss << "Cipher: " << hex << setfill('0') << setw(4)
						 << static_cast<uint16_t>(
									serverHello->cipherSuite)
						 << dec << setfill(' ') << endl;
					pass = true;
				}
			}
			passed += pass;
			if (!pass) {
				lock_guard lg(ssm);
				failedss << hostNode << endl;
			}

			cout << ss.str() << endl;
		};
	}};

	Multithreading::ThreadPool tp;
	for (auto &hostNode : hostNodes) {
		tp.queueTask(makeTask(hostNode));
	}

	tp.blockForTasks();
	cout << "Passed " << passed << " of " << hostNodes.size()
			 << "." << endl
			 << failedss.str() << endl;
	return 0;
}

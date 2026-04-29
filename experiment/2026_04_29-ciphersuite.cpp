#include <rain.hpp>

// <https://tls12.xargs.org/#client-hello/annotated>.

using namespace Rain;
using namespace Networking;
using namespace Http;
using namespace std;

string hexStrToStr(string const &hex) {
	string r;
	for (size_t i{0}; i < hex.length(); i += 3) {
		r.push_back(
			(char)strtol(hex.substr(i, 2).c_str(), NULL, 16));
	}
	return r;
}

void showHexStr(string const &res) {
	cout << "[" << hex << setw(4) << setfill('0')
			 << res.length() << "] ";
	for (auto &i : res) {
		cout << hex << setw(2) << (int)(uint8_t)i << " ";
	}
}

bool testCipherSuite(string const &hostNode) {
	Http::Client<
		Http::Request,
		Http::Response,
		1024,
		1024,
		15000,
		15000,
		Ipv4FamilyInterface,
		StreamTypeInterface,
		TcpProtocolInterface,
		NoLingerSocketOption>
		client(getAddressInfos({hostNode, 443}));
	string req{hexStrToStr(
		R"""(16 03 01 00 d0 01 00 00 cc 03 03 d8 08 3e 33 2d 77 61 fa 09 51 4a d8 0d 3f 2f 6b 4c b3 27 5f e4 63 24 73 7f 3b 4f 8a c6 55 d4 5e 00 00 36 c0 2c c0 30 00 9f cc a9 cc a8 cc aa c0 2b c0 2f 00 9e c0 24 c0 28 00 6b c0 23 c0 27 00 67 c0 0a c0 14 00 39 c0 09 c0 13 00 33 00 9d 00 9c 00 3d 00 3c 00 35 00 2f 01 00 00 6d ff 01 00 01 00 00 00 00 0b 00 09 00 00 06 68 70 2e 63 6f 6d 00 0b 00 02 01 00 00 0a 00 0c 00 0a 00 1d 00 17 00 1e 00 18 00 19 00 23 00 00 00 05 00 05 01 00 00 00 00 00 16 00 00 00 17 00 00 00 0d 00 2a 00 28 04 03 05 03 06 03 08 07 08 08 08 09 08 0a 08 0b 08 04 08 05 08 06 04 01 05 01 06 01 03 03 03 01 03 02 04 02 05 02 06 02 )""")},
		res, t;
	client.send(req);
	do {
		client.recv(t, 1s);
		res += t;
	} while (!t.empty());
	bool pass{res.length() > 16 && res[0] == '\x16'};
	cout << dec << setfill(' ') << "[" << pass << "] ";
	showHexStr(res.substr(0, 7));
	cout << dec << setfill(' ') << res.length() << " "
			 << hostNode << endl;
	// Failures:
	// 02 28 : handshake_failure
	// 02 2f : illegal_parameter
	// 02 32 : decode_error
	// 02 50 : internal_error
	// `openssl s_client -connect codeforces.com:443 -tls1_2
	// -status -msg -debug`.
	return pass;
}

int main() {
	vector<string> hostNodes{
		{"google.com",
			"wikipedia.org",
			"facebook.com",
			"codeforces.com",
			"discord.com",
			"github.com",
			"youtube.com",
			"x.com",
			"stackoverflow.com",
			"usaco.training",
			"usaco.org",
			"bandcamp.com",
			"pixiv.net",
			"myanimelist.net",
			"startpage.com",
			"reddit.com",
			"projecteuler.net",
			"hp.com",
			"amazon.com",
			"visualstudio.com",
			"twitch.tv",
			"leagueoflegends.com",
			"tiktok.com"}};
	size_t passed{};
	for (auto &hostNode : hostNodes) {
		passed += testCipherSuite(hostNode);
	}
	cout << "Passed " << passed << " of " << hostNodes.size()
			 << "." << endl;
	return 0;
}

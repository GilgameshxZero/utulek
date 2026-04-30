#include <rain.hpp>

using namespace Rain;
using namespace Networking;
using namespace std;

int main() {
	Tls::Client<
		Http::ClientSocketSpecInterface<
			Http::Request,
			Http::Response>,
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
			NoLingerSocketOption>>
		client({"facebook.com", 443});
	return 0;
}

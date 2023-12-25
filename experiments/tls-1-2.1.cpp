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

template <template <typename> class... BlockMembers>
class TlsBlockBasic : public std::tuple<BlockMembers...> {
	public:
	using std::tuple<BlockMembers...>::tuple;
};
template <std::size_t sizeRepBytes, template <typename> class... BlockMembers>
class TlsBlockSized : public TlsBlockBasic<BlockMembers...> {
	public:
	using TlsBlockBasic<BlockMembers...>::TlsBlockBasic;
	operator std::string() const {
		std::string basicStr{static_cast<TlsBlockBasic<BlockMembers...>>(*this)};
		return basicStr;
	}
};
class TlsBlockArray;
class TlsBlockVector;

class TlsPlaintextFragment;
class TlsPlaintext;

// Serialize byte data at `block` into `buffer`. Guaranteed to write
// `sizeof(Block)` bytes. Returns number of bytes written.
template <typename Block>
std::size_t serializeTlsBlock(Block const &block, char *buffer) {
	memcpy(buffer, &block, sizeof(Block));
	return sizeof(Block);
}

template <
	template <typename>
	class BlockMember,
	template <typename>
	class... BlockMembers>
std::size_t serializeTlsBlock(
	TlsBlockBasic<BlockMember, BlockMembers...> const &block,
	char *buffer) {
	buffer += serializeTlsBlock(block.template get<BlockMember>(), buffer);
	serializeTlsBlock(
		TlsBlockBasic<BlockMembers...>(block.template get<BlockMembers>()...),
		buffer);
	return sizeof(BlockMember) +
		serializeTlsBlock(
					 TlsBlockBasic<BlockMembers...>(
						 block.template get<BlockMembers>()...),
					 buffer);
}

int main() {
	using namespace Rain::Literal;
	using namespace Rain::Networking;

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
			<< "\x00\x02\xC0\x2C"s	// CipherSuite cipher_suites<2..2^16-2>;
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
			<< "\x00\x0D\x00\x04\x00\x02\x05\x03"s
			<< "\x00\x0B\x00\x02\x01\x00"s	// Extension - EC point formats.
			<< "\x00\x0A\x00\x04\x00\x02\x00\x17"s	// Extension - Elliptic curves.
			;
		client.send(tlsPlaintext.str());
		std::string buffer(128, '\0');
		client.recv(buffer);
		std::cout << std::hex << std::setfill('0');
		for (char const &c : buffer) {
			std::cout << "\\x" << std::setw(2)
								<< static_cast<int>(
										 *reinterpret_cast<unsigned char const *>(&c));
		}
		std::cout << std::endl;
	}
	return 0;
}

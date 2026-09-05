// Standard.
#include <rain.hpp>

#include "platform.hpp"

using namespace Rain;
using namespace Algorithm;
using namespace Data;
using namespace Math;
using namespace Neural;
using namespace Multithreading;
using namespace Error;

using namespace Utulek;

using namespace std;

using LL = long long;
using LD = long double;
using CF = Clamped<float>;

#define RF(x, from, to) \
	for ( \
		LL x(from), _to(to), _delta{x < _to ? 1LL : -1LL}; \
		x != _to; \
		x += _delta)

template<typename Value>
char pixelToChar(Value pixel) {
	if (pixel < 0x80) {
		return pixel < 0x40 ? ' ' : '-';
	} else {
		return pixel < 0xc0 ? '=' : '#';
	}
}

template<typename Value>
void showImg(
	Tensor<Value, 2> const &x,
	ostream &stream = cout) {
	x.template applyOver<1>([&](Tensor<Value, 1> const &row) {
		row.template applyOver<0>([&](Value const &pixel) {
			auto pixelChar{pixelToChar(pixel)};
			stream << pixelChar << pixelChar;
		});
		stream << '\n';
	});
}

size_t constexpr C_CLASS{10}, C_EPOCH{128};
CF constexpr STEP_SIZE{1e-2};

int main(int, char const *const *const) {
	auto assetPath{getAssetPath(__FILE__)};
	filesystem::create_directories(assetPath / ".data");
	cout << "Asset path: " << assetPath << '.' << endl;

	mt19937 gen(0);
	uniform_real_distribution<LD> dist(0.0L, 1.0L);

	Network::FeedForward<CF> network(
		{make_shared<Activation::Linear<CF>>(),
			make_shared<Activation::Relu<CF>>(),
			make_shared<Activation::Linear<CF>>(),
			make_shared<Activation::Relu<CF>>(),
			make_shared<Activation::Linear<CF>>(),
			make_shared<Activation::Softmax<CF>>()});
	{
		// ~98% accuracy.
		auto asset{
			assetPath.parent_path() /
			"2026_05_17-mnist.cpp.asset" / ".data" /
			"network.4.epoch.31.hfm"};
		cout << "Loading " << asset << "..." << endl;
		ifstream fStream(asset, ios::binary);
		HuffmanStreamBuf decoderBuf(*fStream.rdbuf());
		istream decoderStream(&decoderBuf);
		Deserializer deserializer(decoderStream);
		deserializer >> network;
	}
	Loss::CrossEntropy<CF> L;

	Tensor<CF, 1> X({784}, gen, dist), Y({C_CLASS});
	Y[0] = 1;
	RF(k, 0, C_EPOCH) {
		auto activation{network.asApply(X)};
		auto activationGradient{
			network.getActivationGradient(L, Y, activation)};
		auto loss{L.asApply(Y, activation.back())};
		X -= activationGradient[0] * STEP_SIZE;
		cout << "Loss: " << loss << '.' << endl;
		showImg(X.asReshape<2>({0, 28}) * 256);
	}

	// Probably need a GAN-like architecture, or, simpler,
	// perhaps constraints on X, like described in
	// <https://github.com/jcjohnson/cnn-vis>'s linked
	// blogpost.

	return 0;
}

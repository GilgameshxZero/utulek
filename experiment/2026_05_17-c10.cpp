#include <rain.hpp>

#include "platform.hpp"

using namespace Rain;
using namespace Algorithm;
using namespace Data;
using namespace Math;

using namespace Utulek;

using namespace std;

using LL = long long;
using LD = long double;
using CF = Clamped<float>;

#define RF(x, from, to)                                    \
	for (                                                    \
		LL x(from), _to(to), _delta{x < _to ? 1LL : -1LL};     \
		x != _to;                                              \
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

void loadFile(
	filesystem::path const &file,
	Tensor<uint8_t, 2> X,
	Tensor<uint8_t, 1> Y) {
	ifstream dataStream(file);
	RF(i, 0, 10000) {
		Y[i] = readBytes<uint8_t>(dataStream);
		RF(j, 0, 3072) {
			X[i][j] = readBytes<uint8_t>(dataStream);
		}
	}
}

int main() {
	auto assetPath{getAssetPath(__FILE__)};
	filesystem::create_directories(assetPath / ".data");
	cout << "Asset path: " << assetPath << '.' << endl;

	Tensor<uint8_t, 2> trainXRaw({50000, 3072}),
		testXRaw({10000, 3072});
	Tensor<uint8_t, 1> trainY({50000}), testY({10000});
	for (size_t i{1}; i < 6; i++) {
		loadFile(
			assetPath / ".data" / "cifar-10-batches-bin" /
				(string("data_batch_") + to_string(i) + ".bin"),
			trainXRaw.asSlice(
				{{{(i - 1) * 10000, i * 10000}, {}}}),
			trainY.asSlice({{{(i - 1) * 10000, i * 10000}}}));
	}
	loadFile(
		assetPath / ".data" / "cifar-10-batches-bin" /
			"test_batch.bin",
		testXRaw,
		testY);

	auto trainX{trainXRaw.asReshape<4>({0, 3, 32, 32})},
		testX{testXRaw.asReshape<4>({0, 3, 32, 32})};

	showImg(testX[13][0]);
	cout << (int)testY[13] << endl;

	{
		stringstream ss;
		{
			Serializer serializer(ss);
			serializer << trainX << trainY << testX << testY;
		}
		ofstream encoderStream(
			assetPath / ".data" / "cifar10.hfm");
		HuffmanStreamBuf encoderBuf(
			*encoderStream.rdbuf(), ss.str());
		ostream encoder(&encoderBuf);
		encoder << ss.rdbuf();
		encoder.flush();
	}
	return 0;
}

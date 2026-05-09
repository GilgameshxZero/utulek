#include <rain.hpp>

#include "platform.hpp"

using namespace Rain;
using namespace Algorithm;
using namespace Math;
using namespace Error;
using namespace std;

using LL = long long;
using LD = long double;

#define RF(x, from, to)                                    \
	for (                                                    \
		LL x(from), _to(to), _delta{x < _to ? 1LL : -1LL};     \
		x != _to;                                              \
		x += _delta)

char pixelToBlock(uint8_t pixel) {
	if (pixel < 0x80) {
		return pixel < 0x40 ? ' ' : '-';
	} else {
		return pixel < 0xc0 ? '=' : '#';
	}
}

template<typename Value, std::size_t ORDER>
auto readTensor(filesystem::path file) {
	ifstream fTensor(file);
	// Ignore type.
	fTensor.ignore(3);
	// Tensor order.
	releaseAssert(fTensor.get() == ORDER);

	array<size_t, ORDER> shape;
	RF(i, 0, ORDER) {
		shape[i] = readBytes<uint32_t>(fTensor, endian::big);
	}

	Tensor<uint8_t, ORDER> tensor(shape);
	// Special case where data is well-formatted so we don't
	// need applyOver.
	fTensor.read(
		reinterpret_cast<char *>(tensor.data().get()),
		tensor.sizeProduct());
	return tensor;
}

int main(int, char const *const *const) {
	auto assetPath{getAssetPath(__FILE__)};
	cout << "Asset path: " << assetPath << endl;

	random_device rd;
	mt19937 gen(rd());

	auto trainX{readTensor<uint8_t, 3>(
		assetPath / ".data/train-images-idx3-ubyte")};
	cout << "trainX shape: " << trainX.size() << '.' << endl;
	auto trainY{readTensor<uint8_t, 1>(
		assetPath / ".data/train-labels-idx1-ubyte")};
	cout << "trainY shape: " << trainY.size() << '.' << endl;
	auto testX{readTensor<uint8_t, 3>(
		assetPath / ".data/t10k-images-idx3-ubyte")};
	cout << "testX shape: " << testX.size() << '.' << endl;
	auto testY{readTensor<uint8_t, 1>(
		assetPath / ".data/t10k-labels-idx1-ubyte")};
	cout << "testY shape: " << testY.size() << '.' << endl;

	{
		auto sampleIdx{uniform_int_distribution<size_t>(
			0, trainY.size()[0])(gen)};
		cout << "Train sample: " << sampleIdx << '.' << endl
				 << "X: " << endl;
		trainX[sampleIdx].applyOver<1>(
			[&](Tensor<uint8_t, 1> &row) {
				row.applyOver<0>([&](uint8_t &pixel) {
					auto block{pixelToBlock(pixel)};
					cout << block << block;
				});
				cout << endl;
			});
		cout << "Y: " << (int)trainY[sampleIdx] << '.' << endl;
	}

	auto trainXFlat{
		trainX.asReshape<2>({trainX.size()[0], 0})};
	cout << trainXFlat.size() << endl;

	return 0;
}

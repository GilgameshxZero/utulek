#include <rain.hpp>

#include "platform.hpp"

using namespace Rain;
using namespace Algorithm;
using namespace Data;
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
	// Need `template` because it is a template function
	// dependent on a template (`typename Value`).
	x.template applyOver<1>([&](Tensor<Value, 1> const &row) {
		row.template applyOver<0>([&](Value const &pixel) {
			auto pixelChar{pixelToChar(pixel)};
			stream << pixelChar << pixelChar;
		});
		stream << '\n';
	});
}

int main(int, char const *const *const) {
	auto assetPath{getAssetPath(__FILE__)};
	cout << "Asset path: " << assetPath << '.' << endl;

	random_device rd;
	// mt19937 gen(rd());
	mt19937 gen(0);

	// Load from original files and compress with Huffman to a
	// pack.
	//
	// auto trainX{readTensor<uint8_t, 3>( 	assetPath /
	// ".data/train-images-idx3-ubyte")}; cout << "trainX
	// shape: " << trainX.size() << '.' << endl; auto
	// trainY{readTensor<uint8_t, 1>( 	assetPath /
	// ".data/train-labels-idx1-ubyte")}; cout << "trainY
	// shape: " << trainY.size() << '.' << endl; auto
	// testX{readTensor<uint8_t, 3>( 	assetPath /
	// ".data/t10k-images-idx3-ubyte")};
	// cout << "testX shape: " << testX.size() << '.' << endl;
	// auto testY{readTensor<uint8_t, 1>( 	assetPath /
	// ".data/t10k-labels-idx1-ubyte")};
	// cout << "testY shape: " << testY.size() << '.' << endl;

	// {
	// 	stringstream ss;
	// 	{
	// 		Serializer serializer(ss.rdbuf());
	// 		serializer << trainX << trainY << testX << testY;
	// 	}
	// 	Serializer serializer(assetPath / ".data/mnist.hfm");
	// 	HuffmanStreamBuf encoderBuf(
	// 		*serializer.rdbuf(), ss.str());
	// 	ostream encoder(&encoderBuf);
	// 	encoder << ss.rdbuf();
	// 	encoder.flush();
	// }

	Tensor<uint8_t, 3> trainX, testX;
	Tensor<uint8_t, 1> trainY, testY;

	{
		ifstream fStream(assetPath / ".data/mnist.hfm");
		HuffmanStreamBuf decoderBuf(*fStream.rdbuf());
		Deserializer deserializer(&decoderBuf);
		deserializer >> trainX >> trainY >> testX >> testY;
	}

	size_t constexpr C_CLASSES{10};
	// Used for clamping later. Clamp mins are scaled by
	// double of C_CLASSES due to sum.
	LD constexpr CLAMP_MIN{
		numeric_limits<LD>::lowest() / (C_CLASSES)},
		CLAMP_MAX{numeric_limits<LD>::max() / (C_CLASSES)};

	auto trainXDbl{trainX.asReshape<2>({trainX.size()[0], 0})
			.asRetype<LD>()},
		testXDbl{trainX.asReshape<2>({testX.size()[0], 0})
				.asRetype<LD>()};
	Tensor<LD, 2> trainYOneHot({trainY.size()[0], C_CLASSES}),
		testYOneHot({testY.size()[0], C_CLASSES});
	trainYOneHot.applyOver<1>(
		[&](Tensor<LD, 1> &left, uint8_t const &right) {
			left[right] = 1;
		},
		trainY);
	// Failing to flush the end of Huffman stream will corrupt
	// the final bytes.
	cout << "Last few labels: "
			 << testY.asSlice({{{testY.size()[0] - 6}}})
						.asRetype<uint32_t>()
			 << '.' << endl;
	testYOneHot.applyOver<1>(
		[&](Tensor<LD, 1> &left, uint8_t const &right) {
			left[right] = 1;
		},
		testY);

	{
		auto sampleIdx{uniform_int_distribution<size_t>(
			0, trainY.size()[0])(gen)};
		auto x{trainXDbl[sampleIdx]},
			y{trainYOneHot[sampleIdx]};
		Tensor<LD, 2> w({x.size()[0], C_CLASSES});
		Tensor<LD, 1> b({C_CLASSES});
		auto yHat{x.product<1>(w, {0}, {0}) + b};
		cout << "Train sample: " << sampleIdx << '.' << endl
				 << "x = " << endl;
		showImg(x.asReshape<2>({Math::sqrt(x.size()[0]), 0}));
		cout << "y = " << y.asRetype<uint32_t>() << '.' << endl;
		cout << "w.size() = " << w.size() << endl;
		cout << "yHat = " << yHat << '.' << endl;
		Tensor<LD, 2> pW(w.size());
		pW.applyOver<1>(
			[&](Tensor<LD, 1> &pWI, LD const &xI) {
				pWI.applyOver<0>(
					[&](LD &pWIJ, LD const &yJ, LD const &yHatJ) {
						pWIJ = (yJ - yHatJ) * xI;
					},
					y,
					yHat);
			},
			x);
		LD constexpr S{1e-9};
		auto wMod{w + pW * S};
		auto yHatMod{x.product<1>(wMod, {0}, {0}) + b};
		cout << "yHatMod = " << yHatMod << '.' << endl;
	}

	{
		auto sampleIdx{uniform_int_distribution<size_t>(
			0, trainY.size()[0])(gen)};
		auto x{trainXDbl[sampleIdx]},
			y{trainYOneHot[sampleIdx]};
		Tensor<LD, 2> w({x.size()[0], C_CLASSES});
		Tensor<LD, 1> b({C_CLASSES});
		auto yHat{x.product<1>(w, {0}, {0}) + b};
		cout << "Train sample: " << sampleIdx << '.' << endl
				 << "x = " << endl;
		showImg(x.asReshape<2>({Math::sqrt(x.size()[0]), 0}));
		cout << "y = " << y.asRetype<uint32_t>() << '.' << endl;
		cout << "yHat = " << yHat << '.' << endl;
		// Apply (probability distribution) normalization
		// function g ("softmax").
		//
		// Important to clamp value so we dont get +/- inf.
		auto yHatG{yHat.asExp().clamp()};
		cout << "exp(yHat) = " << yHatG << '.' << endl;
		auto g{yHatG.sum()};
		cout << "g = " << g << '.' << endl;
		yHatG /= g;
		cout << "yHatG = " << yHatG << '.' << endl;
		// Loss is a "step-size scaler".
		LD loss{
			-(y * yHatG.asLog().clamp() +
				(1 - y) * (1 - yHatG).asLog().clamp())};
		cout << "loss = " << loss << '.' << endl;
		// "pressure" is to be scaled by step-size and loss.
		auto pW{x.productOuter(y - yHat)};
		auto pB{y - yHat};
		LD constexpr S{1e-4};
		auto wMod{w + S * loss * pW};
		auto bMod{b + S * loss * pB};
		auto yHatMod{x.product<1>(wMod, {0}, {0}) + bMod};
		cout << "bMod = " << bMod << '.' << endl;
		cout << "yHatMod = " << yHatMod << '.' << endl;
		auto yHatModG{yHatMod.asExp().clamp()};
		cout << "exp(yHatMod) = " << yHatModG << '.' << endl;
		auto gMod{yHatModG.sum()};
		cout << "gMod = " << gMod << '.' << endl;
		yHatModG /= gMod;
		cout << "yHatModG = " << yHatModG << '.' << endl;
		LD lossMod{
			-(y * yHatModG.asLog().clamp() +
				(1 - y) * (1 - yHatModG).asLog().clamp())};
		cout << "lossMod = " << lossMod << '.' << endl;
	}

	{
		// Step size.
		LD constexpr S{1e-4};
		auto sampleIdx{uniform_int_distribution<size_t>(
			0, trainY.size()[0])(gen)};
		auto x{trainXDbl[sampleIdx]},
			y{trainYOneHot[sampleIdx]};
		Tensor<LD, 2> w({x.size()[0], C_CLASSES});
		Tensor<LD, 1> b({C_CLASSES});
		// Random initialization.
		w.applyOver<0>([&](auto &value) {
			value = uniform_real_distribution<LD>()(gen);
		});
		b.applyOver<0>([&](auto &value) {
			value = uniform_real_distribution<LD>()(gen);
		});
		// Clamp raw regression so that we don't have +/- INF.
		auto yHat{(x.product<1>(w, {0}, {0}) + b)
				.clamp(CLAMP_MIN, CLAMP_MAX)};
		cout << "Train sample: " << sampleIdx << '.' << endl
				 << "x = " << endl;
		showImg(x.asReshape<2>({Math::sqrt(x.size()[0]), 0}));
		cout << "y = " << y.asRetype<uint32_t>() << '.' << endl;
		cout << "yHat = " << yHat << '.' << endl;
		// Apply (probability distribution) normalization
		// function g ("softmax").
		//
		// Important to clamp value so we dont get +/- inf.
		// Clamp limits are scaled down a bit based on C_CLASSES
		// to protect `sum`.
		//
		// To keep values in range, normalize first before exp.
		// Normalize by subtracting the max value, because that
		// is the one that influences the prediction the most.
		yHat -= yHat.max();
		yHat.clamp(CLAMP_MIN, CLAMP_MAX);
		cout << "normalized(yHat) = " << yHat << '.' << endl;
		auto yHatG{yHat.asExp().clamp(CLAMP_MIN, CLAMP_MAX)};
		cout << "exp(yHat) = " << yHatG << '.' << endl;
		auto g{yHatG.sum()};
		cout << "g = " << g << '.' << endl;
		yHatG /= g;
		cout << "yHatG = " << yHatG << '.' << endl;
		// Loss is a "step-size scaler".
		LD loss{
			-(y * yHatG.asLog().clamp(CLAMP_MIN, CLAMP_MAX) +
				(1 - y) *
					(1 - yHatG).asLog().clamp(CLAMP_MIN, CLAMP_MAX))};
		cout << "loss = " << loss << '.' << endl;
		// "pressure" is to be scaled by step-size and loss.
		auto pW{x.productOuter(y - yHatG)};
		auto pB{y - yHatG};
		// Loss step size scalar needs to be clamped; sometimes
		// loss is ~INF because the original prediction is close
		// to (absolutely wrong).
		//
		// This may not be necessary, but INFs makes the math
		// hard to wrap my head around.
		auto scaledStepSize{
			clamp(loss, CLAMP_MIN, CLAMP_MAX) * S};
		cout << "scaledStepSize = " << scaledStepSize << '.'
				 << endl;
		auto wMod{w + scaledStepSize * pW};
		auto bMod{b + scaledStepSize * pB};
		auto yHatMod{(x.product<1>(wMod, {0}, {0}) + bMod)
				.clamp(CLAMP_MIN, CLAMP_MAX)};
		cout << "bMod = " << bMod << '.' << endl;
		cout << "yHatMod = " << yHatMod << '.' << endl;
		yHatMod -= yHatMod.max();
		yHatMod.clamp(CLAMP_MIN, CLAMP_MAX);
		cout << "normalized(yHatMod) = " << yHatMod << '.'
				 << endl;
		auto yHatModG{
			yHatMod.asExp().clamp(CLAMP_MIN, CLAMP_MAX)};
		cout << "exp(yHatMod) = " << yHatModG << '.' << endl;
		auto gMod{yHatModG.sum()};
		cout << "gMod = " << gMod << '.' << endl;
		yHatModG /= gMod;
		cout << "yHatModG = " << yHatModG << '.' << endl;
		LD lossMod{
			-(y * yHatModG.asLog().clamp(CLAMP_MIN, CLAMP_MAX) +
				(1 - y) *
					(1 - yHatModG)
						.asLog()
						.clamp(CLAMP_MIN, CLAMP_MAX))};
		cout << "lossMod = " << lossMod << '.' << endl;
	}

	return 0;
}

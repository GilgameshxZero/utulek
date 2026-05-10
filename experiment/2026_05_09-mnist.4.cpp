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

size_t constexpr C_CLASSES{10};
LD constexpr STEP_SIZE{1e-3},
	EPS{numeric_limits<LD>::min()};

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

pair<Tensor<LD, 2>, Tensor<LD, 1>> update(
	Tensor<LD, 2> const &X,
	Tensor<LD, 2> const &Y,
	Tensor<LD, 2> const &W,
	Tensor<LD, 1> const &B,
	bool showX = false) {
	auto yHat{X.product<1>(W, {1}, {0})
			.applyOver<1>([&](Tensor<LD, 1> &left) { left += B; })
			.clamp()};
	if (showX) {
		cout << "X[:4] = " << endl;
		X.asSlice({{{0, 4}, {}}})
			.applyOver<1>([&](Tensor<LD, 1> const &left) {
				showImg(left.asReshape<2>(
					{Math::sqrt(left.size()[0]), 0}));
			});
	}
	cout << "Y[:4] = "
			 << Y.asSlice({{{0, 4}, {}}}).asRetype<uint32_t>()
			 << '.' << endl;
	cout << "B = " << B << '.' << endl;
	cout << "yHat[:4] = " << yHat.asSlice({{{0, 4}, {}}})
			 << '.' << endl;

	// Prediction.
	// TODO: Activation f via clamp working?
	auto yHatNorm{yHat.clamp(0).asApplyOver<1>(
		[](Tensor<LD, 1> &left) { left -= left.max(); })};
	cout << "yHatNorm[:4] = "
			 << yHatNorm.asSlice({{{0, 4}, {}}}) << '.' << endl;
	auto yHatG{yHatNorm.asExp().clamp()};
	cout << "exp(yHatNorm)[:4] = "
			 << yHatG.asSlice({{{0, 4}, {}}}) << '.' << endl;
	auto G{
		yHatG.asContract<>(1, [](Tensor<LD, 1> const &right) {
			return right.sum();
		})};
	cout << "G[:4] = " << G.asSlice({{{0, 4}}}) << '.'
			 << endl;
	yHatG.applyOver<1>(
		[](Tensor<LD, 1> &left, LD const &right) {
			left /= right;
		},
		G);
	cout << "yHatG[:4] = " << yHatG.asSlice({{{0, 4}, {}}})
			 << '.' << endl;

	// Loss.
	auto L{-Y.asContract<>(
		1,
		[](Tensor<LD, 1> const &r1, Tensor<LD, 1> const &r2) {
			return r1 * r2.asLog().clamp() +
				(1 - r1) * (1 - r2).asLog().clamp();
		},
		yHatG)};
	LD logLoss{clamp(log(clamp(L.mean())))};
	cout << "L[:4] = " << L.asSlice({{{0, 4}}}) << '.'
			 << endl;
	cout << "logLoss = " << logLoss << '.' << endl;
	auto scaledStepSize{logLoss * STEP_SIZE};
	cout << "scaledStepSize = " << scaledStepSize << '.'
			 << endl;

	// Pressure.
	auto pB{(Y - yHatG)
			.divideElementWise(
				yHatG.asMultiplyElementWise(1 - yHatG) + EPS)
			.asContract<>(
				0,
				[](Tensor<LD, 1> const &right) {
					return right.mean();
				})
			.clamp()};
	auto pW{
		X.asContract<>(
			 0,
			 [](Tensor<LD, 1> const &right) {
				 return right.mean();
			 })
			.productOuter(pB)
			.clamp()};
	auto WMod{(W + scaledStepSize * pW).clamp()};
	auto BMod{(B + scaledStepSize * pB).clamp()};
	cout << "BMod = " << BMod << '.' << endl;
	return {WMod, BMod};
}

int main(int, char const *const *const) {
	auto assetPath{getAssetPath(__FILE__)};
	cout << "Asset path: " << assetPath << '.' << endl;

	random_device rd;
	mt19937 gen(0);

	Tensor<uint8_t, 3> trainX, testX;
	Tensor<uint8_t, 1> trainY, testY;

	{
		ifstream fStream(assetPath / ".data/mnist.hfm");
		HuffmanStreamBuf decoderBuf(*fStream.rdbuf());
		Deserializer deserializer(&decoderBuf);
		deserializer >> trainX >> trainY >> testX >> testY;
	}

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
	testYOneHot.applyOver<1>(
		[&](Tensor<LD, 1> &left, uint8_t const &right) {
			left[right] = 1;
		},
		testY);

	Tensor<LD, 2> W({trainXDbl.size()[1], C_CLASSES});
	Tensor<LD, 1> B({C_CLASSES});
	W.applyOver<0>([&](auto &value) {
		value = uniform_real_distribution<LD>()(gen);
	});
	B.applyOver<0>([&](auto &value) {
		value = uniform_real_distribution<LD>()(gen);
	});

	{
		size_t constexpr BATCH_SIZE{64};
		auto X{trainXDbl.asSlice({{{0, BATCH_SIZE}, {}}})},
			Y{trainYOneHot.asSlice({{{0, BATCH_SIZE}, {}}})};

		RF(i, 0, 64) {
			auto [WMod, BMod] = update(X, Y, W, B);
			W = WMod;
			B = BMod;
		}
	}

	{
		auto sampleIdx{uniform_int_distribution<size_t>(
			0, trainY.size()[0])(gen)};
		size_t constexpr BATCH_SIZE{4};
		auto X{trainXDbl.asSlice(
			{{{sampleIdx, sampleIdx + BATCH_SIZE}, {}}})},
			Y{trainYOneHot.asSlice(
				{{{sampleIdx, sampleIdx + BATCH_SIZE}, {}}})};
		update(X, Y, W, B, true);
	}

	return 0;
}

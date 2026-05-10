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
LD constexpr CLAMP_MIN{numeric_limits<LD>::lowest()},
	CLAMP_MAX{numeric_limits<LD>::max()};
LD constexpr STEP_SIZE{1e-4},
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

pair<Tensor<LD, 2>, Tensor<LD, 1>> propagate(
	Tensor<LD, 2> const &X,
	Tensor<LD, 2> const &Y,
	Tensor<LD, 2> const &W,
	Tensor<LD, 1> const &B) {
	auto yHat{X.product<1>(W, {1}, {0})
			.applyOver<1>(
				[&](Tensor<LD, 1> &yHatI) { yHatI += B; })
			.clamp()};
	cout << "X[:4] = " << endl;
	X.asSlice({{{0, 4}, {}}})
		.applyOver<1>([&](Tensor<LD, 1> const &left) {
			showImg(
				left.asReshape<2>({Math::sqrt(left.size()[0]), 0}));
		});
	cout << "Y[:4] = "
			 << Y.asSlice({{{0, 4}, {}}}).asRetype<uint32_t>()
			 << '.' << endl;
	cout << "B = " << B << '.' << endl;
	cout << "yHat[:4] = " << yHat.asSlice({{{0, 4}, {}}})
			 << '.' << endl;

	// Prediction.
	auto yHatNorm{yHat
			.asApplyOver<1>(
				[](Tensor<LD, 1> &left) { left -= left.max(); })
			.clamp()};
	cout << "yHatNorm[:4] = "
			 << yHatNorm.asSlice({{{0, 4}, {}}}) << '.' << endl;
	auto yHatG{yHatNorm.asExp().clamp()};
	cout << "exp(yHatNorm)[:4] = "
			 << yHatG.asSlice({{{0, 4}, {}}}) << '.' << endl;
	Tensor<LD, 1> G({yHatG.size()[0]});
	G.applyOver<0>(
		[](LD &left, Tensor<LD, 1> const &right) {
			left = right.sum();
		},
		yHatG);
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
	Tensor<LD, 1> L({X.size()[0]});
	L.applyOver<0>(
		[&](
			LD &left,
			Tensor<LD, 1> const &r1,
			Tensor<LD, 1> const &r2) {
			left =
				-(r1 * r2.asLog().clamp() +
					(1 - r1) * (1 - r2).asLog().clamp());
		},
		Y,
		yHatG);
	LD loss{clamp(L.mean(), CLAMP_MIN, CLAMP_MAX)};
	cout << "L[:4] = " << L.asSlice({{{0, 4}}}) << '.'
			 << endl;
	cout << "loss = " << loss << '.' << endl;
	auto scaledStepSize{
		clamp(loss, 0.0L, 128.0L) * STEP_SIZE};
	cout << "scaledStepSize = " << scaledStepSize << '.'
			 << endl;

	// Pressure.
	auto PB{(Y - yHatG)
			.applyOver<1>(
				[](
					Tensor<LD, 1> &left, Tensor<LD, 1> const &right) {
					left.divideElementWise(right);
				},
				yHatG.asMultiplyElementWise(1 - yHatG) + EPS)
			.clamp()};
	Tensor<LD, 1> pB({Y.size()[1]});
	pB.applyOver<0>(
		[&](LD &left, Tensor<LD, 1> const &right) {
			left = clamp(right.mean(), CLAMP_MIN, CLAMP_MAX);
		},
		PB.asTranspose({1, 0}));
	Tensor<LD, 1> XCentroid({X.size()[1]});
	XCentroid.applyOver<0>(
		[](LD &left, Tensor<LD, 1> const &right) {
			left = right.sum();
		},
		X.asTranspose({1, 0}));
	XCentroid /= X.size()[0];
	auto pW{XCentroid.productOuter(pB).clamp()};
	auto WMod{W + scaledStepSize * pW};
	auto BMod{B + scaledStepSize * pB};
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

	{
		size_t BATCH_SIZE{4};
		auto X{trainXDbl.asSlice({{{0, BATCH_SIZE}, {}}})},
			Y{trainYOneHot.asSlice({{{0, BATCH_SIZE}, {}}})};
		Tensor<LD, 2> W({X.size()[1], C_CLASSES});
		Tensor<LD, 1> B({C_CLASSES});
		W.applyOver<0>([&](auto &value) {
			value = uniform_real_distribution<LD>()(gen);
		});
		B.applyOver<0>([&](auto &value) {
			value = uniform_real_distribution<LD>()(gen);
		});

		RF(i, 0, 8) {
			auto [WMod, BMod] = propagate(X, Y, W, B);
			W = WMod;
			B = BMod;
		}
	}

	return 0;
}

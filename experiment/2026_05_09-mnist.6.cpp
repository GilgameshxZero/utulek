#include <rain.hpp>

#include "platform.hpp"

using namespace Rain;
using namespace Algorithm;
using namespace Data;
using namespace Math;
using namespace Error;
using namespace Neural;
using namespace std;

using LL = long long;
using LD = long double;

#define RF(x, from, to)                                    \
	for (                                                    \
		LL x(from), _to(to), _delta{x < _to ? 1LL : -1LL};     \
		x != _to;                                              \
		x += _delta)
size_t constexpr C_CLASSES{10};
LD STEP_SIZE{1e-3};

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

int main(int, char const *const *const) {
	auto assetPath{getAssetPath(__FILE__)};
	cout << "Asset path: " << assetPath << '.' << endl;

	random_device rd;
	mt19937 gen(0);
	uniform_real_distribution<LD> dist;

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
		Network::FeedForward<LD> network(
			{make_shared<Activation::Linear<LD>>(
				 Tensor<LD, 2>({784, 64}, gen, dist),
				 Tensor<LD, 1>({64}, gen, dist)),
				make_shared<Activation::Relu<LD>>(),
				make_shared<Activation::Linear<LD>>(
					Tensor<LD, 2>({64, 10}, gen, dist),
					Tensor<LD, 1>({10}, gen, dist)),
				make_shared<Activation::Normalization<LD>>(),
				make_shared<Activation::Softmax<LD>>()});

		RF(j, 0, 32) {
			RF(i, 0, 16384) {
				auto x{trainXDbl[i]}, y{trainYOneHot[i]};
				// cout << y << endl;
				Loss::CrossEntropy<LD> l(y);
				auto artifact{network.asApplyWithArtifact(x)};
				// cout << artifact << endl;
				// cout << artifact.back() << endl;
				cout << l.asApply(artifact.back()) << endl;
				network.stepWithGradient(l, artifact, STEP_SIZE);
			}
			STEP_SIZE *= 0.98;
		}
		cout << STEP_SIZE << endl;

		RF(i, 59995, 60000) {
			auto x{trainXDbl[i]}, y{trainYOneHot[i]};
			showImg(x.asReshape<2>({28, 28}));
			cout << y << endl;
			Loss::CrossEntropy<LD> l(y);
			auto artifact{network.asApplyWithArtifact(x)};
			cout << artifact.back() << endl;
			cout << l.asApply(artifact.back()) << endl;
		}
	}
	return 0;
}

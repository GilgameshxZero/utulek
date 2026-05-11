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
size_t constexpr C_CLASS{10}, C_EPOCH{32}, BATCH_SIZE{32};
LD constexpr STEP_SIZE{1e-3};

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
		testXDbl{testX.asReshape<2>({testX.size()[0], 0})
				.asRetype<LD>()};
	Tensor<LD, 2> trainYOneHot({trainY.size()[0], C_CLASS}),
		testYOneHot({testY.size()[0], C_CLASS});
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

		LD stepSizeScaler{1.0L};
		Loss::CrossEntropy<LD> L;
		RF(j, 0, C_EPOCH) {
			LD lossMean{};
			size_t c_batch{30000 / BATCH_SIZE};
			RF(i, 0, c_batch) {
				auto X{trainXDbl.asSlice(
					{{{i * BATCH_SIZE, (i + 1) * BATCH_SIZE}, {}}})},
					Y{trainYOneHot.asSlice(
						{{{i * BATCH_SIZE, (i + 1) * BATCH_SIZE},
							{}}})};
				auto artifact{network.asApplyWithArtifact(X)};
				// cout << artifact.back() << endl;
				auto loss{L.asApply(Y, artifact.back())};
				// cout << "LOSS " << loss << endl;
				lossMean += loss;
				network.stepWithGradient(
					L, Y, artifact, STEP_SIZE * stepSizeScaler);
				// return 0;
			}
			cout << j << ": " << lossMean / c_batch << '.'
					 << endl;
			stepSizeScaler *= 0.98;
		}

		RF(i, 0, 4) {
			auto x{testXDbl[i]}, y{testYOneHot[i]};
			showImg(x.asReshape<2>({Math::sqrt(x.size()[0]), 0}));
			cout << y << endl;
			auto artifact{network.asApplyWithArtifact(x)};
			cout << artifact.back() << endl;
			cout << L.asApply(y, artifact.back()) << endl;
		}

		{
			stringstream ss;
			{
				Serializer serializer(ss.rdbuf());
				serializer << network;
			}
			Serializer serializer(
				assetPath / ".data/network.7.hfm");
			HuffmanStreamBuf encoderBuf(
				*serializer.rdbuf(), ss.str());
			ostream encoder(&encoderBuf);
			encoder << ss.rdbuf();
			encoder.flush();
		}
	}

	return 0;
}

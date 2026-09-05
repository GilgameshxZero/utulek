#include <rain.hpp>

#include "platform.hpp"

using namespace Rain;
using namespace Algorithm;
using namespace Data;
using namespace Math;
using namespace Neural;
using namespace Multithreading;

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

size_t constexpr C_CLASS{10}, C_EPOCH{128};
CF constexpr STEP_SIZE{2e-1};

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

int main() {
	size_t C_THREAD{
		max(1_zu, (size_t)thread::hardware_concurrency())},
		BATCH_SIZE{C_THREAD * 32_zu},
		MINI_BATCH_SIZE{BATCH_SIZE / C_THREAD};
	cout << "Threads: " << C_THREAD << '.' << endl;

	mt19937 gen(0);
	// Variance is dependent on network depth.
	normal_distribution<LD> dist(0.0L, 4e-2);

	auto assetPath{getAssetPath(__FILE__)};
	filesystem::create_directories(assetPath / ".data");
	cout << "Asset path: " << assetPath << '.' << endl;

	Tensor<uint8_t, 4> trainXRaw, testXRaw;
	Tensor<uint8_t, 1> trainYRaw, testYRaw;
	{
		ifstream fStream(
			assetPath / ".data" / "cifar10.hfm", ios::binary);
		HuffmanStreamBuf decoderBuf(*fStream.rdbuf());
		istream decoderStream(&decoderBuf);
		Deserializer deserializer(decoderStream);
		deserializer >> trainXRaw >> trainYRaw >> testXRaw >>
			testYRaw;

		showImg(testXRaw[13][0]);
		cout << (int)testYRaw[13] << endl;
	}
	auto trainX{
		trainXRaw.asReshape<2>({trainXRaw.size()[0], 0})
			.asRetype<CF>()},
		testX{testXRaw.asReshape<2>({testXRaw.size()[0], 0})
				.asRetype<CF>()};
	Tensor<CF, 2> trainY({trainYRaw.size()[0], C_CLASS}),
		testY({testYRaw.size()[0], C_CLASS});
	trainY.applyOver<1>(
		[&](Tensor<CF, 1> &left, uint8_t const &right) {
			left[right] = 1;
		},
		trainYRaw);
	testY.applyOver<1>(
		[&](Tensor<CF, 1> &left, uint8_t const &right) {
			left[right] = 1;
		},
		testYRaw);
	trainX /= 256;
	testX /= 256;
	cout << trainY[0] << endl;

	Network::FeedForward<CF> network(
		{make_shared<Activation::Linear<CF>>(
			 Tensor<CF, 2>({3072, 512}, gen, dist),
			 Tensor<CF, 1>({512}, gen, dist)),
			make_shared<Activation::Relu<CF>>(),
			make_shared<Activation::Linear<CF>>(
				Tensor<CF, 2>({512, 86}, gen, dist),
				Tensor<CF, 1>({86}, gen, dist)),
			make_shared<Activation::Relu<CF>>(),
			make_shared<Activation::Linear<CF>>(
				Tensor<CF, 2>({86, 10}, gen, dist),
				Tensor<CF, 1>({10}, gen, dist)),
			make_shared<Activation::Softmax<CF>>()});
	Loss::CrossEntropy<CF> L;

	vector<CF> lossV, scoreV;
	{
		ThreadPool tp(C_THREAD);

		size_t cBatchTrain{trainX.size()[0] / BATCH_SIZE};
		vector<LL> mbp(cBatchTrain * C_THREAD);
		RF(i, 0, mbp.size()) { mbp[i] = i; }

		RF(k, 0, C_EPOCH) {
			// Shuffle mini-batches each epoch.
			shuffle(mbp.begin(), mbp.end(), gen);

			{
				vector<vector<Tensor<CF, 2>>> activationV(C_THREAD),
					activationGradientV(C_THREAD);
				atomic<float> loss{};

				RF(i, 0, cBatchTrain) {
					atomic_size_t jOuter{};
					RF(j, 0, C_THREAD) {
						tp.queueTask([&]() {
							size_t jInner{jOuter++},
								mbi{i * C_THREAD + jInner},
								mbix{mbp[mbi] * MINI_BATCH_SIZE};
							auto X{trainX.asSlice(
								{{{mbix, mbix + MINI_BATCH_SIZE}, {}}})},
								Y{trainY.asSlice(
									{{{mbix, mbix + MINI_BATCH_SIZE}, {}}})};
							activationV[jInner] = network.asApply(X);
							activationGradientV[jInner] =
								network.getActivationGradient(
									L, Y, activationV[jInner]);

							loss +=
								L.asApply(Y, activationV[jInner].back());
						});
					}

					tp.blockForTasks();
					cout << "Mini-batch " << (i + 1) * C_THREAD
							 << " / " << cBatchTrain * C_THREAD
							 << ": loss = " << loss / ((i + 1) * C_THREAD)
							 << ".    \r" << flush;

					RF(j, 0, C_THREAD) {
						network.stepWithActivationGradient(
							activationV[j],
							activationGradientV[j],
							STEP_SIZE / (k + C_EPOCH));
					}
				}
			}

			CF lossSum{};
			Tensor<size_t, 1> score({testX.size()[0]});
			size_t cBatchTest{testY.size()[0] / BATCH_SIZE};
			{
				mutex lossSumMtx;

				RF(i, 0, cBatchTest) {
					atomic_size_t jOuter{};
					RF(j, 0, C_THREAD) {
						tp.queueTask([&]() {
							size_t jInner{jOuter++},
								mbi{i * C_THREAD + jInner},
								mbix{mbi * MINI_BATCH_SIZE};
							auto X{testX.asSlice(
								{{{mbix, mbix + MINI_BATCH_SIZE}, {}}})},
								Y{testY.asSlice(
									{{{mbix, mbix + MINI_BATCH_SIZE}, {}}})};
							auto activationBack{
								network.asApply(X).back()};
							auto loss{L.asApply(Y, activationBack)};
							score
								.asSlice({{{mbix, mbix + MINI_BATCH_SIZE}}})
								.applyOver<0>(
									[](
										size_t &left,
										uint8_t const &r1,
										Tensor<CF, 1> const &r2) {
										left = r1 == r2.argMax();
									},
									testYRaw.asSlice(
										{{{mbix, mbix + MINI_BATCH_SIZE}}}),
									activationBack);

							lock_guard lossSumLck(lossSumMtx);
							lossSum += loss;
						});
					}
					// Can also place outside i loop.
					tp.blockForTasks();
				}
			}

			lossV.push_back(lossSum / cBatchTest / C_THREAD);
			scoreV.push_back(
				(CF)score.sum() / cBatchTest / BATCH_SIZE);
			cout << "Epoch " << k << ": loss = " << lossV.back()
					 << ", score = " << scoreV.back() << '.' << endl;

			{
				stringstream ss;
				{
					Serializer serializer(ss);
					serializer << network;
				}
				ofstream encoderStream(
					assetPath / ".data" /
					(string("c10.1-network-epoch_") + to_string(k) +
						".hfm"));
				HuffmanStreamBuf encoderBuf(
					*encoderStream.rdbuf(), ss.str());
				ostream encoder(&encoderBuf);
				encoder << ss.rdbuf();
				encoder.flush();
			}
		}
	}
	return 0;
}

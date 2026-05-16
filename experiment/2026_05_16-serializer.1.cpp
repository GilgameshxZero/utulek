#include <rain.hpp>

#include "platform.hpp"

using namespace Rain;
using namespace Algorithm;
using namespace Data;
using namespace Math;
using namespace Neural;
using namespace Multithreading;
using namespace Error;
using namespace std;

using LL = long long;
using LD = long double;
using CLD = Clamped<LD>;

#define RF(x, from, to)                                    \
	for (                                                    \
		LL x(from), _to(to), _delta{x < _to ? 1LL : -1LL};     \
		x != _to;                                              \
		x += _delta)

size_t constexpr C_CLASS{10}, C_EPOCH{1};
CLD constexpr STEP_SIZE{3e-4};

int main(int, char const *const *const) {
	size_t C_THREAD{
		max(1_zu, (size_t)thread::hardware_concurrency())},
		BATCH_SIZE{C_THREAD * 32},
		MINI_BATCH_SIZE{BATCH_SIZE / C_THREAD};
	cout << "Threads: " << C_THREAD << '.' << endl;

	auto assetPath{getAssetPath(__FILE__)};
	filesystem::create_directories(assetPath / ".data");
	cout << "Asset path: " << assetPath << '.' << endl;

	mt19937 gen(0);
	uniform_real_distribution<LD> dist(-1.0L, 1.0L);

	Tensor<uint8_t, 3> trainX, testX;
	Tensor<uint8_t, 1> trainY, testY;
	{
		ifstream fStream(
			assetPath.parent_path().parent_path().parent_path() /
				"rain/test/"
				"math-neural-network-feed_forward.cpp.asset/"
				"mnist.hfm",
			ios::binary);
		HuffmanStreamBuf decoderBuf(*fStream.rdbuf());
		istream decoderStream(&decoderBuf);
		Deserializer deserializer(decoderStream);
		deserializer >> trainX >> trainY >> testX >> testY;
	}
	auto trainXDbl{trainX.asReshape<2>({trainX.size()[0], 0})
			.asRetype<CLD>()},
		testXDbl{testX.asReshape<2>({testX.size()[0], 0})
				.asRetype<CLD>()};
	Tensor<CLD, 2> trainYOneHot({trainY.size()[0], C_CLASS}),
		testYOneHot({testY.size()[0], C_CLASS});
	trainYOneHot.applyOver<1>(
		[&](Tensor<CLD, 1> &left, uint8_t const &right) {
			left[right] = 1;
		},
		trainY);
	testYOneHot.applyOver<1>(
		[&](Tensor<CLD, 1> &left, uint8_t const &right) {
			left[right] = 1;
		},
		testY);
	// trainXDbl.slice({{{0, 128}, {}}});
	// trainYOneHot.slice({{{0, 128}, {}}});

	Network::FeedForward<CLD> network(
		{make_shared<Activation::Linear<CLD>>(
			 Tensor<CLD, 2>({784, 256}, gen, dist),
			 Tensor<CLD, 1>({256}, gen, dist)),
			make_shared<Activation::Relu<CLD>>(),
			make_shared<Activation::Normalization<CLD>>(),
			make_shared<Activation::Linear<CLD>>(
				Tensor<CLD, 2>({256, 64}, gen, dist),
				Tensor<CLD, 1>({64}, gen, dist)),
			make_shared<Activation::Relu<CLD>>(),
			make_shared<Activation::Normalization<CLD>>(),
			make_shared<Activation::Linear<CLD>>(
				Tensor<CLD, 2>({64, 10}, gen, dist),
				Tensor<CLD, 1>({10}, gen, dist)),
			make_shared<Activation::Normalization<CLD>>(),
			make_shared<Activation::Softmax<CLD>>()});
	Loss::CrossEntropy<CLD> L;

	vector<CLD> lossV, scoreV;
	{
		ThreadPool tp(C_THREAD);

		size_t cBatchTrain{trainXDbl.size()[0] / BATCH_SIZE};
		vector<LL> mbp(cBatchTrain * C_THREAD);
		RF(i, 0, mbp.size()) { mbp[i] = i; }

		RF(k, 0, C_EPOCH) {
			// Shuffle mini-batches each epoch.
			shuffle(mbp.begin(), mbp.end(), gen);

			{
				vector<vector<Tensor<CLD, 2>>> activationV(C_THREAD),
					activationGradientV(C_THREAD);
				CLD loss{};

				RF(i, 0, cBatchTrain) {
					atomic_size_t jOuter{};
					RF(j, 0, C_THREAD) {
						tp.queueTask([&]() {
							size_t jInner{jOuter++},
								mbi{i * C_THREAD + jInner},
								mbix{mbp[mbi] * MINI_BATCH_SIZE};
							auto X{trainXDbl.asSlice(
								{{{mbix, mbix + MINI_BATCH_SIZE}, {}}})},
								Y{trainYOneHot.asSlice(
									{{{mbix, mbix + MINI_BATCH_SIZE}, {}}})};
							activationV[jInner] = network.asApply(X);
							activationGradientV[jInner] =
								network.getActivationGradient(
									L, Y, activationV[jInner]);

							loss =
								L.asApply(Y, activationV[jInner].back());
						});
					}

					tp.blockForTasks();
					cout << "Mini-batch " << (i + 1) * C_THREAD
							 << " / " << cBatchTrain * C_THREAD
							 << ": loss = " << loss << ".    \r" << flush;

					RF(j, 0, C_THREAD) {
						network.stepWithActivationGradient(
							activationV[j],
							activationGradientV[j],
							STEP_SIZE);
					}
				}
			}

			CLD lossSum{};
			Tensor<size_t, 1> score({testXDbl.size()[0]});
			size_t cBatchTest{testXDbl.size()[0] / BATCH_SIZE};
			{
				mutex lossSumMtx;

				RF(i, 0, cBatchTest) {
					atomic_size_t jOuter{};
					RF(j, 0, C_THREAD) {
						tp.queueTask([&]() {
							size_t jInner{jOuter++},
								mbi{i * C_THREAD + jInner},
								mbix{mbi * MINI_BATCH_SIZE};
							auto X{testXDbl.asSlice(
								{{{mbix, mbix + MINI_BATCH_SIZE}, {}}})},
								Y{testYOneHot.asSlice(
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
										Tensor<CLD, 1> const &r2) {
										left = r1 == r2.argMax();
									},
									testY.asSlice(
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
				(CLD)score.sum() / cBatchTest / BATCH_SIZE);
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
						(string("network.layer.") + to_string(k) +
							".hfm"),
					ios::binary);
				Serializer serializer(encoderStream);
				HuffmanStreamBuf encoderBuf(
					*serializer.stream.rdbuf(), ss.str());
				ostream encoder(&encoderBuf);
				encoder << ss.rdbuf();
				encoder.flush();
			}
		}
	}

	return 0;
}

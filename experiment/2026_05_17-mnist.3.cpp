// Mini.
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
using CF = Clamped<float>;

#define RF(x, from, to)                                    \
	for (                                                    \
		LL x(from), _to(to), _delta{x < _to ? 1LL : -1LL};     \
		x != _to;                                              \
		x += _delta)

size_t constexpr C_CLASS{10}, C_EPOCH{16};
CF constexpr STEP_SIZE{1e-1};

int main(int, char const *const *const) {
	size_t C_THREAD{
		max(1_zu, (size_t)thread::hardware_concurrency())},
		// 1_zu},
		BATCH_SIZE{C_THREAD * 32_zu},
		// BATCH_SIZE{C_THREAD * 1_zu},
		MINI_BATCH_SIZE{BATCH_SIZE / C_THREAD};
	cout << "Threads: " << C_THREAD << '.' << endl;

	auto assetPath{getAssetPath(__FILE__)};
	filesystem::create_directories(assetPath / ".data");
	cout << "Asset path: " << assetPath << '.' << endl;

	mt19937 gen(0);
	normal_distribution<LD> dist(0.0L, 0.1L);
	// normal_distribution<LD> dist(1000.0L);

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
			.asRetype<CF>()},
		testXDbl{testX.asReshape<2>({testX.size()[0], 0})
				.asRetype<CF>()};
	Tensor<CF, 2> trainYOneHot({trainY.size()[0], C_CLASS}),
		testYOneHot({testY.size()[0], C_CLASS});
	trainYOneHot.applyOver<1>(
		[&](Tensor<CF, 1> &left, uint8_t const &right) {
			left[right] = 1;
		},
		trainY);
	testYOneHot.applyOver<1>(
		[&](Tensor<CF, 1> &left, uint8_t const &right) {
			left[right] = 1;
		},
		testY);
	// Either this or normalization layer.
	trainXDbl /= 256;
	testXDbl /= 256;

	Network::FeedForward<CF> network(
		// {make_shared<Activation::Normalization<CF>>(),
		{make_shared<Activation::Linear<CF>>(
			 Tensor<CF, 2>({784, 96}, gen, dist),
			 Tensor<CF, 1>({96}, gen, dist)),
			make_shared<Activation::Relu<CF>>(),
			make_shared<Activation::Linear<CF>>(
				Tensor<CF, 2>({96, 10}, gen, dist),
				Tensor<CF, 1>({10}, gen, dist)),
			make_shared<Activation::Softmax<CF>>()});
	Loss::CrossEntropy<CF> L;

	vector<CF> lossV, scoreV;
	{
		ThreadPool tp(C_THREAD);

		size_t cBatchTrain{trainXDbl.size()[0] / BATCH_SIZE};
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
							auto X{trainXDbl.asSlice(
								{{{mbix, mbix + MINI_BATCH_SIZE}, {}}})},
								Y{trainYOneHot.asSlice(
									{{{mbix, mbix + MINI_BATCH_SIZE}, {}}})};
							activationV[jInner] = network.asApply(X);
							activationGradientV[jInner] =
								network.getActivationGradient(
									L, Y, activationV[jInner]);
							// if (mbi >= 0) {
							// 	// cout
							// 	// 	<< dynamic_cast<Activation::Linear<CF>
							// 	// *>( 			 network.layer[0].get())
							// 	// 			 ->weight
							// 	// 	<< endl
							// 	// 	<< dynamic_cast<Activation::Linear<CF>
							// 	// *>( 			 network.layer[0].get())
							// 	// 			 ->bias
							// 	// 	<< endl;
							// 	cout
							// 		<< dynamic_cast<Activation::Linear<CF>
							// *>( 				 network.layer[2].get())
							// 				 ->weight
							// 		<< endl
							// 		<< dynamic_cast<Activation::Linear<CF>
							// *>( 				 network.layer[2].get())
							// 				 ->bias
							// 		<< endl;
							// 	RF(i, 1, 5) {
							// 		cout << activationV[jInner][i] << endl;
							// 	}
							// 	cout << Y << endl;
							// 	cout << "HERE" << endl;
							// 	cout << network.layer[3]
							// 						->getIncrementalGradient(
							// 							activationV[jInner][3],
							// 							activationV[jInner][4])
							// 			 << endl;
							// 	RF(
							// 		i,
							// 		1,
							// 		activationGradientV[jInner].size()) {
							// 		cout << activationGradientV[jInner][i]
							// 				 << endl;
							// 	}
							// }

							loss +=
								L.asApply(Y, activationV[jInner].back());
						});
					}

					tp.blockForTasks();
					cout << "Mini-batch " << (i + 1) * C_THREAD
							 << " / " << cBatchTrain * C_THREAD
							 << ": loss = " << loss / ((i + 1) * C_THREAD)
							 << ".    \r" << flush;
					// cout << endl;

					RF(j, 0, C_THREAD) {
						network.stepWithActivationGradient(
							activationV[j],
							activationGradientV[j],
							STEP_SIZE / (k + 1));
					}
				}
			}

			CF lossSum{};
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
										Tensor<CF, 1> const &r2) {
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
					(string("network.3.epoch.") + to_string(k) +
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

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

int main(int, char const *const *const) {
	{
		Tensor<LD, 1> z1({{{4}}, 1.0L, -2.0L, 3.0L, -4.0L});
		Neural::Activation::Relu<LD> relu;
		auto z2{relu.asApply(z1)};
		auto gradient{relu.getGradient(z2)};
		cout << z2.template asExpand<0>(
							z2.size()[0],
							[](
								Tensor<LD, 1> &left,
								Tensor<LD, 1> const &right) {
								left.deepCopyFrom(right);
							})
				 << endl;
		cout << z2.template asExpand<1>(
							z2.size()[0],
							[](LD &left, LD const &right) {
								left = right;
							})
				 << endl;
		cout << z1 << endl << z2 << endl << gradient << endl;
		relu.apply(z1);
	}
	{
		Tensor<LD, 1> z1({{{4}}, 1.0L, -2.0L, 3.0L, -4.0L});
		Neural::Activation::Softmax<LD> activation;
		auto z2{activation.asApply(z1)};
		auto gradient{activation.getGradient(z2)};
		cout << z1 << endl << z2 << endl << gradient << endl;
	}
	{
		Tensor<LD, 1> z1({{{4}}, 1.0L, -2.0L, 3.0L, -4.0L});
		Neural::Activation::Identity<LD> activation;
		auto z2{activation.asApply(z1)};
		auto gradient{activation.getGradient(z2)};
		cout << z1 << endl << z2 << endl << gradient << endl;
	}
	{
		Tensor<LD, 1> z1({{4}, 1.0L, -2.0L, 3.0L, -4.0L});
		Neural::Activation::Linear<LD> activation(
			Tensor<LD, 2>::identity(4),
			Tensor<LD, 1>({4}, -1.0L, -1.0L, 1.0L, 1.0L));
		{
			activation.weight[0][1]++;
			cout << activation.weight << endl
					 << activation.bias << endl;
			Neural::Activation::Linear<LD> a2(
				activation.weight, activation.bias);
			a2.weight[0][1]++;
			cout << activation.weight << endl
					 << activation.bias << endl;
		}
		auto z2{activation.asApply(z1)};
		auto gradient{activation.getGradient(z2)};
		cout << z1 << endl << z2 << endl << gradient << endl;
	}

	{
		random_device rd;
		mt19937 gen(0);
		uniform_real_distribution<LD> dist;

		Network::FeedForward<LD> network(
			{make_shared<Activation::Linear<LD>>(
				 Tensor<LD, 2>({8, 16}, gen, dist),
				 Tensor<LD, 1>({16}, gen, dist)),
				make_shared<Activation::Relu<LD>>(),
				make_shared<Activation::Linear<LD>>(
					Tensor<LD, 2>({16, 4}, gen, dist),
					Tensor<LD, 1>({4}, gen, dist)),
				make_shared<Activation::Softmax<LD>>()});
		Tensor<LD, 1> x(
			{8},
			1.0L,
			-2.0L,
			3.0L,
			-4.0L,
			5.0L,
			-6.0L,
			7.0L,
			-8.0L),
			y({4}, 0.0L, 0.0L, 1.0L, 0.0L);
		auto artifact{network.asApplyWithArtifact(x)};
		cout << artifact << endl;
		auto yHat{artifact.back()};
		cout << x << endl << yHat << endl << y << endl;
		auto const l{Loss::CrossEntropy<LD>(y)};
		auto loss{l.asApply(yHat)};
		// d(loss = l_0) / d(z_4) \in R^{n_4 = c = 4}.
		auto g4{l.getGradient(yHat)};
		cout << loss << endl << g4 << endl;
		// d(loss = l_0) / d(z_3) \in R^{n_3 = 4}.
		auto g3{g4.asMultiply<1>(
			network.layer[3].get()->getGradient(artifact[4]),
			{0},
			{0})};
		cout << g3 << endl;
		releaseAssert(g3.size() == array<size_t, 1>{{4}});
		// d(loss = l_0) / d(z_2) \in R^{n_2 = 16}.
		auto g2{g3.asMultiply<1>(
			network.layer[2].get()->getGradient(artifact[3]),
			{0},
			{0})};
		cout << g2 << endl;
		releaseAssert(g2.size() == array<size_t, 1>{{16}});
		// d(loss = l_0) / d(z_1) \in R^{n_1 = 16}.
		auto g1{g2.asMultiply<1>(
			network.layer[1].get()->getGradient(artifact[2]),
			{0},
			{0})};
		cout << g1 << endl;
		releaseAssert(g1.size() == array<size_t, 1>{{16}});
		// d(loss = l_0) / d(z_0 = x) \in R^{n_0 = 8}.
		auto g0{g1.asMultiply<1>(
			network.layer[0].get()->getGradient(artifact[1]),
			{0},
			{0})};
		cout << g0 << endl;
		releaseAssert(g0.size() == array<size_t, 1>{{8}});

		auto gradient{network.getGradient(l, artifact)};
		cout << gradient << endl;

		RF(i, 0, 256) {
			network.stepWithGradient(l, artifact, 1e-3);
			auto yHatMod{network.asApply(x)};
			auto loss{l.asApply(yHatMod)};
			cout << yHatMod << endl << loss << endl;
		}
	}
	return 0;
}

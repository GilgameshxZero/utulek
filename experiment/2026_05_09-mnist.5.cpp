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
		Tensor<LD, 1> z1({{{4}}, 1.0L, -2.0L, 3.0L, -4.0L});
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
	return 0;
}

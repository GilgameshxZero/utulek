#include <rain.hpp>

using namespace Rain;

using namespace std;

// template<auto &&...T>
// class Trait {};

// template<typename T>
// class Trait {};

template<typename T>
class A {};

int main() {
	{
		Math::Tensor<int, 2> t({2, 2}, 0, 1, 2, 3);
	}
	// {
	// 	Trait<A>;
	// }
	return 0;
}

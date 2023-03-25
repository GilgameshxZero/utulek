#include "rain.hpp"

using namespace Rain::Literal;

template <std::size_t INDEX>
class CTFibonacci;

template <>
class CTFibonacci<0> {
	public:
	static constexpr std::size_t get() { return 0_zu; }
};

template <>
class CTFibonacci<1> {
	public:
	static constexpr std::size_t get() { return 1_zu; }
};

template <std::size_t INDEX>
class CTFibonacci {
	public:
	static constexpr std::size_t get() {
		return CTFibonacci<INDEX - 1>::get() + CTFibonacci<INDEX - 2>::get();
	}
};

// CTCharFibonacci is an erroneous implementation, but the template should "wrap
// around". If the wrap-around is successful, then this should take no real time
// to compute. This may require optimizations to be enabled.
using namespace Rain::Literal;
template <unsigned char INDEX>
class CTCharFibonacci;

template <>
class CTCharFibonacci<3> {
	public:
	static constexpr std::size_t get() { return 3_zu; }
};

template <>
class CTCharFibonacci<4> {
	public:
	static constexpr std::size_t get() { return 5_zu; }
};

template <unsigned char INDEX>
class CTCharFibonacci {
	public:
	static constexpr std::size_t get() {
		return CTCharFibonacci<INDEX - 1>::get() +
			CTCharFibonacci<INDEX - 2>::get();
	}
};

// Essentially rebuilds integral_constant.
template <std::size_t VALUE>
class CTSizeT {
	public:
	static constexpr std::size_t get() { return VALUE; }
};

// More explicitly efficient compile-time recursion. Discussion:
// <https://stackoverflow.com/questions/22645551/calculate-the-fibonacci-number-recursive-approach-in-compile-time-constexpr>.
template <std::size_t INDEX>
class CTFibonacciExplicit;

template <>
class CTFibonacciExplicit<0> : public CTSizeT<0> {};

template <>
class CTFibonacciExplicit<1> : public CTSizeT<1> {};

template <std::size_t INDEX>
class CTFibonacciExplicit : public CTSizeT<
															CTFibonacciExplicit<INDEX - 1>::get() +
															CTFibonacciExplicit<INDEX - 2>::get()> {};

int main() {
	std::cout << CTFibonacci<0>::get() << '\n'
						<< CTFibonacci<1>::get() << '\n'
						<< CTFibonacci<2>::get() << '\n'
						<< CTFibonacci<3>::get() << '\n'
						<< CTFibonacci<4>::get() << '\n'
						<< CTFibonacci<5>::get() << '\n'
						<< CTFibonacci<6>::get() << '\n'
						<< CTFibonacci<7>::get() << '\n'
						<< CTFibonacci<8>::get() << '\n'
						<< CTFibonacci<9>::get() << '\n'
						<< CTFibonacci<10>::get() << '\n'
						<< '\n'
						// Only fast under optimizations.
						// << CTCharFibonacci<0>::get() << '\n'
						// << CTCharFibonacci<1>::get() << '\n'
						// << CTCharFibonacci<2>::get() << '\n'
						// << CTCharFibonacci<3>::get() << '\n'
						// << CTCharFibonacci<4>::get() << '\n'
						// << '\n'
						<< CTFibonacciExplicit<40>::get() << '\n';
	return 0;
}

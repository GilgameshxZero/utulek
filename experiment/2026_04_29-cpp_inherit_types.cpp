#include <rain.hpp>

using namespace std;

class A {
	protected:
	using Type = int;
};

class B : virtual public A {
	public:
	Type value;
};

// Types from a dependent superclass are not automatically
// injected.

// template<typename T>
// class C {
// 	protected:
// 	using Type = int;
// };

// template<typename T>
// class D : virtual public C<T> {
// 	public:
// 	Type value;
// };

int main() {
	B b;
	b.value = 3;
	cout << b.value;
	// D d;
	// d.value = 3;
	// cout << d.value;
	return 0;
}

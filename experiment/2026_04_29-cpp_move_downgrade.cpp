#include <rain.hpp>

using namespace std;

class A {
	public:
	A() {}
	A(A &&a) {
		cout << "A::move\n";
	}
};

class B : virtual public A {
	// Move constructors are not inherited.
	using A::A;

	public:
	B() : A() {}
};

int main() {
	A a;
	B b(std::move(a));
	return 0;
}

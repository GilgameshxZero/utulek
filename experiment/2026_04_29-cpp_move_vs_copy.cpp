#include <rain.hpp>

using namespace std;

class A {
	public:
	A() {}
	A(A const &) = delete;
	A &operator=(A const &) = delete;
	A(A &&) { cout << "A::move\n"; }
	A &operator=(A &&) { return *this; }
};

class B : virtual public A {
	public:
	B() : A() {}
	B(B const &) = delete;
	B &operator=(B const &) = delete;
	B(B &&b) : A(std::move(b)) { cout << "B::move\n"; }
	B &operator=(B &&) { return *this; }
};

int main() {
	A a;
	B b2;
	B b1(std::move(b2));
	// B b3(b2);
	return 0;
}

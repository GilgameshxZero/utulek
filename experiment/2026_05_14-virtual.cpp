#include <rain.hpp>

using namespace std;

class Base1 {
	public:
	virtual ~Base1() = default;

	int64_t mask;
	char a;
};

class Derived1 : public Base1 {
	public:
	bool flag;
};

class Base2 {
	public:
	int32_t mask;
	char a;
};

class Derived2 : public Base2 {
	public:
	bool flag;
};

class Base3 {
	public:
	int32_t mask;
	char a;

	virtual ~Base3() {}
};

class Middle31 : virtual public Base3 {
	public:
	bool flag;
};

class Middle32 : virtual public Base3 {
	public:
	bool flag;
};

class Derived3 : public Middle31, public Middle32 {
	public:
	bool b;
};

int main() {
	cout << "sizeof(Base1): " << sizeof(Base1) << endl
			 << "sizeof(Derived1): " << sizeof(Derived1) << endl
			 << "sizeof(Base2): " << sizeof(Base2) << endl
			 << "sizeof(Derived2): " << sizeof(Derived2) << endl
			 << endl;

	Derived1 d1;
	cout << "Derived1 d1;" << endl
			 << "&d1: " << &d1 << endl
			 << "&d1.mask: " << &d1.mask << endl
			 << "(void *)&d1.a: " << (void *)&d1.a << endl
			 << "&d1.flag: " << &d1.flag << endl
			 << endl;

	Derived2 d2;
	cout << "Derived2 d2;" << endl
			 << "&d2: " << &d2 << endl
			 << "&d2.mask: " << &d2.mask << endl
			 << "(void *)&d2.a: " << (void *)&d2.a << endl
			 << "&d2.flag: " << &d2.flag << endl
			 << endl;

	Derived3 d3;
	Middle31 *m31{&d3};
	Middle32 *m32{&d3};
	Base3 *b3{&d3}, *b31{m31}, *b32{m32};
	cout << "&d3: " << &d3 << endl
			 << "m31: " << m31 << endl
			 << "&m31->flag: " << &m31->flag << endl
			 << "m32: " << m32 << endl
			 << "&m32->flag: " << &m32->flag << endl
			 << "&d3.b: " << &d3.b << endl
			 << "b3: " << b3 << endl
			 << "b31: " << b31 << endl
			 << "b32: " << b32 << endl
			 << "&b3->mask: " << &b3->mask << endl
			 << "(void *)&b3->a: " << (void *)&b3->a << endl
			 << endl;
}

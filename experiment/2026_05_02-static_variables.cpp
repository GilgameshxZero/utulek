#include <rain.hpp>

using namespace std;

enum class B { A, B };

class A {
	public:
	template<B>
	static int &a() {
		static int value{};
		return value;
	}

	template<B>
	int &b() {
		// Static variables in member functions are shared
		// between all instances of the class, not just all
		// calls to the function within the same instance.
		//
		// However, templates can still retrieve a unique value
		// generated for each instance of the template.
		static int value{};
		return value;
	}

	inline static int counter{};
	int tc;
	A() : tc{counter++} {}

	template<B b, int>
	static int &cInner() {
		static int value;
		return value;
	}

	template<B b>
	int &c() {
		// // Doesn't work, templates must be determined at compile time.
		// return cInner<b, tc>();
		static int value;
		return value;
	}
};

int main() {
	A a, b;
	a.a<B::A>()++;
	a.b<B::A>()++;
	a.c<B::A>()++;
	b.a<B::A>()++;
	b.b<B::A>()++;
	b.c<B::A>()++;
	a.a<B::B>()++;
	a.b<B::B>()++;
	a.c<B::B>()++;
	b.a<B::B>()++;
	b.b<B::B>()++;
	b.c<B::B>()++;

	cout << a.a<B::A>() << a.b<B::A>() << a.c<B::A>()
			 << b.a<B::A>() << b.b<B::A>() << b.c<B::A>() << endl;
	cout << a.a<B::B>() << a.b<B::B>() << a.c<B::B>()
			 << b.a<B::B>() << b.b<B::B>() << b.c<B::B>() << endl;
}

#include <array>
#include <iostream>
#include <tuple>

using namespace std;

class Wrapper {
	public:
	int value;

	Wrapper(int value) {
		this->value = value;
		cout << "default-construct: " << this->value << ".\n";
	}
	Wrapper(Wrapper const &wrapper) {
		this->value = wrapper.value;
		cout << "copy-construct: " << this->value << ".\n";
	}

	Wrapper &operator=(Wrapper const &right) {
		cout << "copy-assign: " << this->value << " <- " << right.value << ".\n";
		this->value = right.value;
		return *this;
	}
	Wrapper &operator=(Wrapper &&right) {
		cout << "move-assign: " << this->value << " <- " << right.value << ".\n";
		this->value = right.value;
		return *this;
	}
};

ostream &operator<<(ostream &left, Wrapper &right) {
	return left << right.value;
}

class Throwable {
	public:
	Throwable() {}

	Throwable &operator=(Throwable const &) {
		cout << "[Throwable] copy-assign.\n";
		throw runtime_error("");
		return *this;
	}
};

class Noexcept {
	public:
	Noexcept() noexcept {}

	Noexcept &operator=(Noexcept const &) noexcept {
		cout << "[Noexcept] copy-assign.\n";
		return *this;
	}
};

int main() {
	{
		array<Wrapper, 3> X{0, 1, 2};
		tie(X[2], X[1], X[0]) = {X[0], X[1], X[2]};
		cout << X[0] << ' ' << X[1] << ' ' << X[2] << '\n';
	}

	{
		array<Wrapper, 3> X{0, 1, 2};
		tie(X[2], X[1], X[0]) = tuple{X[0], X[1], X[2]};
		cout << X[0] << ' ' << X[1] << ' ' << X[2] << '\n';
	}

	{
		array<Wrapper, 3> X{0, 1, 2};
		tie(X[2], X[1], X[0]) =
			tuple<Wrapper &, Wrapper &, Wrapper &>{X[0], X[1], X[2]};
		cout << X[0] << ' ' << X[1] << ' ' << X[2] << '\n';
	}

	{
		Throwable A;
		Noexcept B;
		try {
			tie(B, A) = tuple{B, A};
		} catch (...) {
			cout << "Thrown & caught.\n";
		}
		try {
			tie(A, B) = tuple{A, B};
		} catch (...) {
			cout << "Thrown & caught.\n";
		}
	}
	return 0;
}

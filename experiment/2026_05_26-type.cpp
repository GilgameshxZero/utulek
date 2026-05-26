#include <rain.hpp>

using namespace Rain;

using namespace std;

// TODO unify operator+ return type pointer vs. raw via
// better usage of stuff like alloca.
class ArithmeticTypeInterface {
	public:
	virtual ~ArithmeticTypeInterface() = default;

	virtual explicit operator int() const = 0;
	virtual explicit operator float() const = 0;

	// Arithmetic operators always returns type of LHS by
	// computing the result in heap, copy constructing the
	// return type, and de-allocating on return.
	//
	// Immediate de-allocation is accomplished via unique_ptr.
	auto operator+(
		ArithmeticTypeInterface const &right) const {
		return std::unique_ptr<ArithmeticTypeInterface>(
			this->addWith(right));
	}

	friend auto &operator<<(
		std::ostream &stream,
		ArithmeticTypeInterface const &right) {
		return right.streamWith(stream);
	}

	virtual ArithmeticTypeInterface *addWith(
		ArithmeticTypeInterface const &) const = 0;
	virtual std::ostream &streamWith(
		std::ostream &) const = 0;
};

class IntegerType : public ArithmeticTypeInterface {
	private:
	int value;

	public:
	IntegerType(int value = {}) : value{value} {}

	explicit operator int &() { return this->value; }
	explicit operator int const &() const {
		return this->value;
	}
	virtual explicit operator int() const override {
		return this->value;
	}
	virtual explicit operator float() const override {
		return static_cast<float>(this->value);
	}

	auto operator+(
		ArithmeticTypeInterface const &right) const {
		return *std::unique_ptr<IntegerType>(
			this->addWith(right));
	}

	virtual IntegerType *addWith(
		ArithmeticTypeInterface const &right) const override {
		return new IntegerType(
			this->value + static_cast<int>(right));
	}
	virtual std::ostream &streamWith(
		std::ostream &stream) const override {
		return stream << this->value;
	}
};

class FloatType : public ArithmeticTypeInterface {
	private:
	float value;

	public:
	FloatType(float value = {}) : value{value} {}

	explicit operator float &() { return this->value; }
	explicit operator float const &() const {
		return this->value;
	}
	virtual explicit operator int() const override {
		return static_cast<int>(this->value);
	}
	virtual explicit operator float() const override {
		return this->value;
	}

	auto operator+(
		ArithmeticTypeInterface const &right) const {
		return *std::unique_ptr<FloatType>(
			this->addWith(right));
	}

	friend auto &operator<<(
		std::ostream &stream,
		FloatType const &right) {
		return stream << right.value;
	}

	virtual FloatType *addWith(
		ArithmeticTypeInterface const &right) const override {
		return new FloatType(
			this->value + static_cast<float>(right));
	}
	virtual std::ostream &streamWith(
		std::ostream &stream) const override {
		return stream << this->value;
	}
};

int main() {
	{
		IntegerType xx(1);
		FloatType yy(2.2);
		ArithmeticTypeInterface *x{&xx}, *y{&yy};
		auto z1{*x + *y};
		auto z2{*y + *x};

		cout << *z1 << endl << *z2 << endl;
	}
	return 0;
}

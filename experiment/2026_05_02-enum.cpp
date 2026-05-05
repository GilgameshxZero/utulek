#include <rain.hpp>

using namespace std;

// Non-contiguous enum which stores also maps each enum
// value to a persistent value.
enum class Enum : std::size_t {
	ONE,
	THREE,
	FIVE,
	SEVEN,
	_COUNT
};
template<Enum e>
int const &value() {
	static array<
		int,
		static_cast<std::size_t>(Enum::_COUNT)> constexpr v{
		{1, 3, 5, 7}};
	return v[static_cast<size_t>(e)];
}

template<typename E>
auto const &resolve(E const &);

template<>
auto const &resolve(Enum const &e) {
	static array<
		int,
		static_cast<size_t>(Enum::_COUNT)> constexpr value{
		{1, 3, 5, 7}};
	return value[static_cast<size_t>(e)];
}

class F {
	public:
	enum Value : size_t { ONE, THREE, FIVE, SEVEN, _COUNT };
	static inline array<int, _COUNT> constexpr _V{
		{1, 3, 5, 7}};
	static inline array<string, _COUNT> constexpr _S{
		{"ONE", "THREE", "FIVE", "SEVEN"}};

	Value value;

	F(Value const &value) : value{value} {}
};

array<string, F::_COUNT> constexpr fStr{
	{"ONE", "THREE", "FIVE", "SEVEN"}};

	// Not great for code maintenance.
class G {
	public:
	enum Value { ONE = 1, THREE = 3, FIVE = 5, SEVEN = 7 };

	Value value;

	G(Value const &value) : value{value} {}

	size_t asIndex() {
		switch (this->value) {
			case ONE:
				return 0;
			case THREE:
				return 1;
			case FIVE:
				return 2;
			case SEVEN:
			default:
				return 3;
		}
	}
};

int main() {
	cout << resolve<>(Enum::FIVE) << endl;
	cout << F::_V[F::SEVEN] << endl;
	cout << fStr[F::SEVEN] << endl;
	cout << F::SEVEN << endl;
	return 0;
}

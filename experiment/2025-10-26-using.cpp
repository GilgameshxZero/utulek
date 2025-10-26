#include <rain.hpp>

using namespace std;

class Policy {
	public:
	using Value = int;
};

template <typename PolicyType>
class Class {
	public:
	// using Value = typename PolicyType::Value;
	// typedef typename PolicyType::Value Value;
	using typename PolicyType::Value;
};

int main() {
	Class<Policy>::Value i{3};
	cout << i;
	return 0;
}

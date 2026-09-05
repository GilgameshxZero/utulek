#include <rain.hpp>

using namespace Rain;
using namespace Math;

using namespace std;

int main() {
	Tensor<int, 2> t({{3, 3}, 0, 1, 2, 3, 4, 5, 6, 7, 8});
	cout << t << endl;

	return 0;
}

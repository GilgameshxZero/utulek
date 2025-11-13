#include <rain.hpp>

using namespace std;
using namespace Rain::Math;

int main() {
	Tensor<int, 2> A{{2, 2}, 1, 2, 3, 4};
	cout << A << endl << A * 3 << endl << A * (A * 3) << endl;
	return 0;
}

#include <rain.hpp>

using namespace Rain;
using namespace Math;
using namespace std;

int main() {
	{
		Matrix<int, 2> m;
		cout << m << '\n';
	}
	{
		Matrix<int, 2> m(5), n(1);
		cout << m << '\n' << n << '\n' << m + n << '\n';
		n += m;
		n += m;
		cout << n << '\n';
		n -= n;
		cout << n << '\n';
		n.fill(3);
		cout << n << '\n';
	}
	{
		Matrix<int, 2> x{1, 0, 1, 1};
		cout << x << '\n';
		x = x.T();
		cout << x << '\n';
		x += x.I();
		cout << x << '\n';
		x *= 3;
		cout << x << '\n';
		x *= x.I();
		cout << x << '\n';
		// Triples the first column.
		x *= Matrix<int, 2>{3, 0, 0, 1};
		cout << x << '\n';
	}
	return 0;
}

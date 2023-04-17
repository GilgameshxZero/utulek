// Not functional (yet).

#include "rain.hpp"

int main() {
	using LL = long long;
	using namespace std;

	LL A, B;
	cin >> A >> B;

	__asm {
		mov eax, A
		mov ecx, B
		shl eax, ecx
		mov A, eax
	}

	cout << A << '\n';
	return 0;
}

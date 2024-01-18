// MSVC: only works in x86 mode.
#include <iostream>

int main() {
	int A{3};

	__asm {
		mov eax, A
		shl eax, 0x2
		mov A, eax
	}

	std::cout << "A: " << A;
	return 0;
}

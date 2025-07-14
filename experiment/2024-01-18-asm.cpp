// MSVC: only works in x86 mode.
#include <rain/platform.hpp>

#include <iostream>

int A{3};

int main() {
#ifdef RAIN_PLATFORM_WINDOWS
	__asm {
		mov eax, A
		shl eax, 0x2
		mov A, eax
	}
#else
	__asm__(
		"mov eax, A\n"
		"shl eax, 0x2\n"
		"mov A, eax\n");
#endif

	std::cout << "A: " << A << '\n';
	return 0;
}

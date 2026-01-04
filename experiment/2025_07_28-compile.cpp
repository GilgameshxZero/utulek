#include <rain.hpp>

// Run with `make run BIN=2025-07-28-compile SRC="2025_07_28-compile.cpp
// 2025_07_28-compile_sub.cpp"`. Consider turning PCH off for experimentation.

// Specific header folder is not included because Intellisense would be hard.
#include "2025_07_28-compile.cpp.assets/sub.hpp"
#include "../build/version.hpp"

#include <iostream>

int main() {
	std::cout << "Build: " << UTULEK_VERSION_BUILD << ".\n";
	foo();
	return 0;
}

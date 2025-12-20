#include <iostream>

using namespace std;

int main() {
	// Should work on all platforms, except Windows CMD.
	cout << "\033[2J" << "\033[1;1H" << "Hello world!"
			 << "\033[5;5H" << "\x1b[32m" << "Line 5, col 5" << "\x1b[0m" << endl;
	return 0;
}

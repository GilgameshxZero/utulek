#include <rain.hpp>

using namespace Rain;

using namespace std;

int main() {
	Console::cout(
		{.fgColor = Console::Color::RED, .row = 3, .col = 4},
		"hi");
	return 0;
}

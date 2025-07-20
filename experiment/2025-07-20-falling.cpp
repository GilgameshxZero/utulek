#include <rain.hpp>

int main() {
	using namespace Rain;
	using namespace Algorithm::Geometry;
	using Color = Console::Color;
	using Code = Console::Code;

	Console::clear();
	Console::hideCursor();
	std::thread([&]() {
		auto size{Console::getSize()};
		std::uniform_int_distribution<long> randX(
			0, size.x - 1),
			randY(0, size.y - 1), randColor(0, 7);
		while (true) {
			PointL point{
				randX(Random::generator), randY(Random::generator)};
			Color color{
				static_cast<Color>(randColor(Random::generator))};
			Console::cout(
				{.bgColor = color, .row = point.y, .col = point.x},
				' ');
		}
	}).detach();
	Console::getch();
	Console::log("hi");
	return 0;
}

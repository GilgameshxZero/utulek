#include <rain.hpp>

int main() {
	using namespace Rain;
	using namespace Algorithm::Geometry;
	using Color = Console::Color;
	using Code = Console::Code;

	Console::clear();
	Console::hideCursor();
	auto size{Console::getSize()};
	for (long i{0}; i < size.x; i++) {
		Console::cout(
			{.bgColor = Color::BLUE, .row = 0, .col = i}, ' ');
		Console::cout(
			{.bgColor = Color::BLUE, .row = size.y - 1, .col = i},
			' ');
	}
	for (long i{0}; i < size.y; i++) {
		Console::cout(
			{.bgColor = Color::BLUE, .row = i, .col = 0}, ' ');
		Console::cout(
			{.bgColor = Color::BLUE, .row = i, .col = size.x - 1},
			' ');
	}

	PointL position{size / 2};
	Console::cout(
		{.fgColor = Color::BLACK,
		 .bgColor = Color::RED,
		 .row = position.y,
		 .col = position.x},
		'X');
	RectangleL bounds{{1, 1}, size - 2};
	Code c{Console::getch()};
	while (c != Code::CTRL_C) {
		Console::cout(
			{.row = position.y, .col = position.x}, ' ');
		switch (c) {
			case Code::ARROW_LEFT:
				position.x--;
				break;
			case Code::ARROW_UP:
				position.y--;
				break;
			case Code::ARROW_RIGHT:
				position.x++;
				break;
			case Code::ARROW_DOWN:
				position.y++;
				break;
			default:
				break;
		}
		position = bounds.clamp(position);
		Console::cout(
			{.fgColor = Color::BLACK,
			 .bgColor = Color::RED,
			 .row = position.y,
			 .col = position.x},
			'X');
		c = Console::getch();
	}

	Console::clear();
	Console::showCursor();
	return 0;
}

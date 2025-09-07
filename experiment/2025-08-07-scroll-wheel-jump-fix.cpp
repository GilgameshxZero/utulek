#include <rain.hpp>

using namespace Rain::Literal;
using namespace Rain::Windows;
using namespace std;

unsigned long timeDeltaThresh{32};
DWORD timePrev{0};
bool dirPrev{false};

LRESULT CALLBACK lowLevelMouseProc(int nCode, WPARAM wParam, LPARAM lParam) {
	if (nCode < 0 || wParam != WM_MOUSEWHEEL) {
		return CallNextHookEx(NULL, nCode, wParam, lParam);
	}

	MSLLHOOKSTRUCT *msLLHookStruct{reinterpret_cast<MSLLHOOKSTRUCT *>(lParam)};
	WORD data{HIWORD(msLLHookStruct->mouseData)};
	DWORD time{msLLHookStruct->time}, timeDelta{time - timePrev};
	timePrev = time;
	bool dir{data <= std::numeric_limits<WORD>::max() / 2},
		dirSame{dir == dirPrev};

	// Ignore if different direction and the switch was too fast. Allow multiple
	// rejections in a row.
	Rain::Console::log("Scroll: (", dir, ", ", timeDelta, ").");
	if (!dirSame && timeDelta <= timeDeltaThresh) {
		Rain::Console::log("Rejected!");
		return -1;
	}
	dirPrev = dir;
	return CallNextHookEx(NULL, nCode, wParam, lParam);
}

int main(int argc, const char *const *argv) {
	ShowWindow(validateSystemCall(GetConsoleWindow()), SW_HIDE);

	Rain::String::CommandLineParser parser;
	parser.addParser("time-delta-thresh", timeDeltaThresh);
	parser.parse(argc - 1, argv + 1);

	HINSTANCE hInstance{validateSystemCall(GetModuleHandle(NULL))};
	HHOOK llKbHook{validateSystemCall(
		SetWindowsHookEx(WH_MOUSE_LL, lowLevelMouseProc, hInstance, NULL))};

	runMessageLoop();
	validateSystemCall(UnhookWindowsHookEx(llKbHook));
	return 0;
}

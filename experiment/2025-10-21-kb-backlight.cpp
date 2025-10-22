#include <rain.hpp>

/*
Thoughts:

Doesn't seem like the keyboard backlight button is distinguishable from within
the OS. Maybe this is a BIOS capability? In which case, we don't have the
expertise to mess with that here.
*/

using namespace Rain::Literal;
using namespace Rain::Windows;
using namespace std;

unsigned long timeDeltaThresh{32};
DWORD timePrev{0};
bool dirPrev{false};

LRESULT CALLBACK lowLevelKeyboardProc(int nCode, WPARAM wParam, LPARAM lParam) {
	if (nCode < 0) {
		return CallNextHookEx(NULL, nCode, wParam, lParam);
	}
	KBDLLHOOKSTRUCT *kbdLLHookStruct{reinterpret_cast<KBDLLHOOKSTRUCT *>(lParam)};
	std::cout << kbdLLHookStruct->vkCode << ' ' << kbdLLHookStruct->scanCode
						<< std::endl;

	return CallNextHookEx(NULL, nCode, wParam, lParam);
}

int main(int argc, const char *const *argv) {
	ShowWindow(validateSystemCall(GetConsoleWindow()), SW_HIDE);

	HINSTANCE hInstance{validateSystemCall(GetModuleHandle(NULL))};
	HHOOK llKbHook{validateSystemCall(
		SetWindowsHookEx(WH_KEYBOARD_LL, lowLevelKeyboardProc, hInstance, NULL))};

	MessageBox(NULL, "hi", "hi", MB_OK);

	runMessageLoop();
	validateSystemCall(UnhookWindowsHookEx(llKbHook));
	return 0;
}

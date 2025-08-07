#include <rain.hpp>

using namespace Rain::Literal;
using namespace Rain::Windows;
using namespace std;

DWORD prevTime{0};

LRESULT CALLBACK lowLevelMouseProc(int nCode, WPARAM wParam, LPARAM lParam) {
	if (nCode < 0 || wParam != WM_MOUSEWHEEL) {
		return CallNextHookEx(NULL, nCode, wParam, lParam);
	}

	MSLLHOOKSTRUCT *msLLHookStruct{reinterpret_cast<MSLLHOOKSTRUCT *>(lParam)};
	WORD data{HIWORD(msLLHookStruct->mouseData)};
	DWORD time{msLLHookStruct->time};
	bool isDown{data == WHEEL_DELTA};
	Rain::Console::log(isDown, ' ', time - prevTime);
	prevTime = time;

	return CallNextHookEx(NULL, nCode, wParam, lParam);
}

int main() {
	ShowWindow(validateSystemCall(GetConsoleWindow()), SW_HIDE);

	HINSTANCE hInstance{validateSystemCall(GetModuleHandle(NULL))};
	HHOOK llKbHook{validateSystemCall(
		SetWindowsHookEx(WH_MOUSE_LL, lowLevelMouseProc, hInstance, NULL))};

	runMessageLoop();
	validateSystemCall(UnhookWindowsHookEx(llKbHook));
	return 0;
}

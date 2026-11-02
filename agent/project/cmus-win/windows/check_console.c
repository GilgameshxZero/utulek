/* Exercise the real application in an isolated Windows console.
 * GPL-2.0-or-later. Pass executable, WAV path, and output report path.
 * Uses a silent generated WAV; it does not touch the user's music/configuration.
 */
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdio.h>
#include <stdlib.h>
#include <wchar.h>

static HANDLE input, screen;
static FILE *report;
static int failed;
static void send_text(const wchar_t *text)
{
    for (; *text; text++) {
        INPUT_RECORD records[2] = {0};
        records[0].EventType = records[1].EventType = KEY_EVENT;
        records[0].Event.KeyEvent.bKeyDown = TRUE;
        records[0].Event.KeyEvent.wRepeatCount = records[1].Event.KeyEvent.wRepeatCount = 1;
        records[0].Event.KeyEvent.uChar.UnicodeChar = records[1].Event.KeyEvent.uChar.UnicodeChar = *text;
        if (*text == L'\r') records[0].Event.KeyEvent.wVirtualKeyCode = records[1].Event.KeyEvent.wVirtualKeyCode = VK_RETURN;
        records[1].Event.KeyEvent.bKeyDown = FALSE;
        DWORD written;
        WriteConsoleInputW(input, records, 2, &written);
    }
    Sleep(400);
}
static char *snapshot(const char *label)
{
    CloseHandle(screen);
    screen = CreateFileW(L"CONOUT$", GENERIC_READ|GENERIC_WRITE, FILE_SHARE_READ|FILE_SHARE_WRITE, NULL, OPEN_EXISTING, 0, NULL);
    CONSOLE_SCREEN_BUFFER_INFO info;
    GetConsoleScreenBufferInfo(screen, &info);
    int len = info.dwSize.X * info.dwSize.Y;
    wchar_t *wide = calloc(len + 1, sizeof(wchar_t));
    DWORD read;
    ReadConsoleOutputCharacterW(screen, wide, len, (COORD){0,0}, &read);
    int bytes = WideCharToMultiByte(CP_UTF8, 0, wide, read, NULL, 0, NULL, NULL);
    char *text = calloc(bytes + 1, 1);
    WideCharToMultiByte(CP_UTF8, 0, wide, read, text, bytes, NULL, NULL);
    fprintf(report, "\n--- %s ---\n", label);
    for (int i = 0; i < len; i += info.dwSize.X) {
        char line[4096];
        int n = WideCharToMultiByte(CP_UTF8, 0, wide+i, info.dwSize.X, line, sizeof(line), NULL, NULL);
        while (n && line[n-1] == ' ') n--;
        fwrite(line, 1, n, report); fputc('\n', report);
    }
    free(wide); fflush(report); return text;
}
static void expect(const char *label, const char *text, const char *needle)
{
    int ok = strstr(text, needle) != NULL;
    fprintf(report, "%s: %s\n", label, ok ? "PASS" : "FAIL");
    if (!ok) failed = 1;
    free((void *)text);
}
static int position(const char *text, char status)
{
    char pattern[] = {' ', status, ' ', 0};
    const char *found = text;
    while ((found = strstr(found, pattern))) {
        int minutes, seconds;
        if (sscanf(found + 3, "%d:%d", &minutes, &seconds) == 2)
            return minutes * 60 + seconds;
        found++;
    }
    return -1;
}
static void resize_and_check(int width, int height)
{
    CloseHandle(screen);
    screen = CreateFileW(L"CONOUT$", GENERIC_READ|GENERIC_WRITE,
        FILE_SHARE_READ|FILE_SHARE_WRITE, NULL, OPEN_EXISTING, 0, NULL);
    SMALL_RECT small = {0, 0, 19, 4};
    SMALL_RECT rect = {0, 0, width - 1, height - 1};
    int ok = SetConsoleWindowInfo(screen, TRUE, &small) &&
        SetConsoleScreenBufferSize(screen, (COORD){width, height}) &&
        SetConsoleWindowInfo(screen, TRUE, &rect);
    INPUT_RECORD record = { .EventType = WINDOW_BUFFER_SIZE_EVENT };
    record.Event.WindowBufferSizeEvent.dwSize = (COORD){width, height};
    DWORD written;
    WriteConsoleInputW(input, &record, 1, &written);
    Sleep(600);
    CONSOLE_SCREEN_BUFFER_INFO size;
    GetConsoleScreenBufferInfo(screen, &size);
    int divider = -1, rows = 0;
    for (int row = 1; row < height - 3; row++) {
        WCHAR line[512];
        DWORD read;
        ReadConsoleOutputCharacterW(screen, line, width,
            (COORD){0, row}, &read);
        int found = -1;
        for (int col = 0; col < (int)read; col++) {
            if (line[col] == 0x2502) {
                if (found != -1) ok = 0;
                found = col;
            }
        }
        if (found < 0 || (divider >= 0 && found != divider)) ok = 0;
        divider = found;
        rows++;
    }
    ok = ok && rows > 0 && size.dwSize.X == width &&
        size.srWindow.Bottom - size.srWindow.Top + 1 == height;
    fprintf(report, "resize %dx%d divider alignment: %s\n", width, height,
        ok ? "PASS" : "FAIL");
    if (!ok) failed = 1;
    fflush(report);
}
int wmain(int argc, wchar_t **argv)
{
    if (argc != 4) return 2;
    report = _wfopen(argv[3], L"wb"); if (!report) return 3;
    wchar_t config[32768];
    swprintf(config, 32768, L"%ls.config-%llu", argv[3], GetTickCount64());
    CreateDirectoryW(config, NULL);
    SetEnvironmentVariableW(L"CMUS_HOME", config);
    FreeConsole();
    if (!AllocConsole()) return 4;
    input = CreateFileW(L"CONIN$", GENERIC_READ|GENERIC_WRITE, FILE_SHARE_READ|FILE_SHARE_WRITE, NULL, OPEN_EXISTING, 0, NULL);
    screen = CreateFileW(L"CONOUT$", GENERIC_READ|GENERIC_WRITE, FILE_SHARE_READ|FILE_SHARE_WRITE, NULL, OPEN_EXISTING, 0, NULL);
    SetHandleInformation(input, HANDLE_FLAG_INHERIT, HANDLE_FLAG_INHERIT);
    SetHandleInformation(screen, HANDLE_FLAG_INHERIT, HANDLE_FLAG_INHERIT);
    SetConsoleScreenBufferSize(screen, (COORD){120,40});
    SMALL_RECT rect = {0,0,119,39}; SetConsoleWindowInfo(screen, TRUE, &rect);
    STARTUPINFOW start = { .cb = sizeof(start), .dwFlags = STARTF_USESTDHANDLES,
        .hStdInput = input, .hStdOutput = screen, .hStdError = screen };
    PROCESS_INFORMATION process;
    wchar_t command[32768]; swprintf(command, 32768, L"\"%ls\"", argv[1]);
    if (!CreateProcessW(argv[1], command, NULL, NULL, TRUE, 0, NULL, NULL, &start, &process)) {
        fprintf(report, "CreateProcess failed: %lu\n", GetLastError()); return 5;
    }
    Sleep(1200);
    /* The narrow layout shortens the title to just "Library". */
    expect("initial UI", snapshot("startup"), "Empty (use :add)");
    resize_and_check(80, 24);
    resize_and_check(60, 20);
    resize_and_check(40, 12);
    resize_and_check(80, 24);
    swprintf(command, 32768, L":add %ls\r", argv[2]);
    for (wchar_t *p = command; *p; p++) if (*p == L'\\') *p = L'/';
    send_text(command); Sleep(800);
    send_text(L"2");
    expect("library import", snapshot("library"), "1 tracks");
    send_text(L":player-play\r"); Sleep(1300);
    expect("playing", snapshot("playing"), " > 00:");
    send_text(L":player-pause\r");
    char *paused_screen = snapshot("paused");
    int paused_position = position(paused_screen, '|');
    expect("paused", paused_screen, " | 00:");
    send_text(L":seek +5\r");
    char *seek_screen = snapshot("seek while paused");
    int seek_position = position(seek_screen, '|');
    int seek_ok = paused_position >= 0 && seek_position - paused_position == 5;
    fprintf(report, "seek +5 seconds: %s\n", seek_ok ? "PASS" : "FAIL");
    if (!seek_ok) failed = 1;
    free(seek_screen);
    send_text(L":player-pause\r"); Sleep(600);
    expect("resumed", snapshot("resumed"), " > 00:");
    send_text(L":player-stop\r");
    expect("stopped", snapshot("stopped"), " . 00:");
    send_text(L":quit\r");
    DWORD result = WaitForSingleObject(process.hProcess, 5000), code;
    if (result != WAIT_OBJECT_0) { TerminateProcess(process.hProcess, 99); failed = 1; }
    GetExitCodeProcess(process.hProcess, &code);
    fprintf(report, "exit: %lu; checks: %s\n", code, failed ? "FAIL" : "PASS");
    CloseHandle(process.hThread); CloseHandle(process.hProcess);
    CloseHandle(input); CloseHandle(screen); fclose(report); FreeConsole();
    return failed || code != 0;
}

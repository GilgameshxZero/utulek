/* Real Windows console checks with an isolated config and disposable sources. */
#define wmain original_console_check_main
#include "check_console.c"
#undef wmain

static wchar_t base[32768], config_dir[32768];

static void write_fixture(const wchar_t *relative, const char *contents)
{
    wchar_t path[32768];
    swprintf(path, 32768, L"%ls/%ls", base, relative);
    FILE *f = _wfopen(path, L"wb");
    if (!f) { failed = 1; return; }
    fwrite(contents, 1, strlen(contents), f);
    fclose(f);
}

static void wav_fixture(const wchar_t *relative)
{
    wchar_t path[32768];
    swprintf(path, 32768, L"%ls/%ls", base, relative);
    FILE *f = _wfopen(path, L"wb");
    unsigned char header[] = {
        'R','I','F','F', 36+16,0,0,0, 'W','A','V','E', 'f','m','t',' ',
        16,0,0,0, 1,0, 2,0, 0x40,0x1f,0,0, 0,0x7d,0,0, 4,0,16,0,
        'd','a','t','a',16,0,0,0, 0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0
    };
    if (!f) { failed = 1; return; }
    fwrite(header, 1, sizeof(header), f);
    fclose(f);
}

static int saved_count(const wchar_t *name, const char *needle)
{
    wchar_t path[32768];
    swprintf(path, 32768, L"%ls/playlists/%ls", config_dir, name);
    FILE *f = _wfopen(path, L"rb");
    if (!f) return -1;
    char line[65536];
    int count = 0;
    while (fgets(line, sizeof(line), f)) {
        if (needle && !strstr(line, needle)) { count = -10000; break; }
        count++;
    }
    fclose(f);
    return count;
}

static void check_count(const char *label, const wchar_t *name, int expected, const char *needle)
{
    int actual = -2;
    for (int i = 0; i < 12; i++) {
        send_text(L":view 3\r:save\r");
        actual = saved_count(name, needle);
        if (actual == expected) break;
        Sleep(100);
    }
    fprintf(report, "%s: %s (%d expected %d)\n", label,
        actual == expected ? "PASS" : "FAIL", actual, expected);
    if (actual != expected) failed = 1;
    fflush(report);
}

static PROCESS_INFORMATION launch(const wchar_t *exe)
{
    /* snapshot() reopens CONOUT$; the new handle must be inheritable too. */
    SetHandleInformation(input, HANDLE_FLAG_INHERIT, HANDLE_FLAG_INHERIT);
    SetHandleInformation(screen, HANDLE_FLAG_INHERIT, HANDLE_FLAG_INHERIT);
    STARTUPINFOW start = { .cb = sizeof(start), .dwFlags = STARTF_USESTDHANDLES,
        .hStdInput = input, .hStdOutput = screen, .hStdError = screen };
    PROCESS_INFORMATION process = {0};
    wchar_t command[32768];
    swprintf(command, 32768, L"\"%ls\"", exe);
    if (!CreateProcessW(exe, command, NULL, NULL, TRUE, 0, NULL, NULL, &start, &process)) {
        fprintf(report, "CreateProcess failed: %lu\n", GetLastError());
        failed = 1;
    }
    Sleep(1000);
    DWORD exit_code;
    if (process.hProcess && GetExitCodeProcess(process.hProcess, &exit_code) &&
            exit_code != STILL_ACTIVE) {
        fprintf(report, "player exited during startup: %lu\n", exit_code);
        free(snapshot("failed startup"));
        CloseHandle(process.hThread);
        CloseHandle(process.hProcess);
        process.hProcess = NULL;
        failed = 1;
    }
    return process;
}

static void quit_player(PROCESS_INFORMATION process)
{
    send_text(L":quit\r");
    if (WaitForSingleObject(process.hProcess, 5000) != WAIT_OBJECT_0) {
        TerminateProcess(process.hProcess, 99);
        failed = 1;
    }
    DWORD code;
    GetExitCodeProcess(process.hProcess, &code);
    if (code) failed = 1;
    fprintf(report, "clean exit: %s\n", code ? "FAIL" : "PASS");
    CloseHandle(process.hThread);
    CloseHandle(process.hProcess);
}

int wmain(int argc, wchar_t **argv)
{
    if (argc != 4) return 2;
    report = _wfopen(argv[3], L"wb");
    if (!report) return 3;
    swprintf(base, 32768, L"%ls", argv[2]);
    for (wchar_t *p = base; *p; p++) if (*p == L'\\') *p = L'/';
    CreateDirectoryW(base, NULL);
    wchar_t path[32768];
    for (const wchar_t **p = (const wchar_t *[]){L"sources", L"sources/sub", L"config", NULL}; *p; p++) {
        swprintf(path, 32768, L"%ls/%ls", base, *p);
        CreateDirectoryW(path, NULL);
    }
    swprintf(config_dir, 32768, L"%ls/config", base);
    write_fixture(L"config/rc", "set softvol=true\n");
    wav_fixture(L"sources/one.wav");
    wav_fixture(L"sources/two.wav");
    write_fixture(L"sources/a.m3u", "\xef\xbb\xbf#EXTM3U\r\none.wav\r\ntwo.wav\r\n");
    write_fixture(L"sources/sub/a.m3u", "../two.wav\n");
    write_fixture(L"sources/b.pls", "[playlist]\nFile2=two.wav\nFile1=one.wav\n");
    char utf8_base[32768], uri[100000] = "file:///";
    WideCharToMultiByte(CP_UTF8, 0, base, -1, utf8_base, sizeof(utf8_base), NULL, NULL);
    char *encoded = uri + strlen(uri);
    for (unsigned char *p = (unsigned char *)utf8_base; *p; p++) {
        if (*p == ' ' || *p >= 128) { sprintf(encoded, "%%%02X", *p); encoded += 3; }
        else *encoded++ = *p;
    }
    strcpy(encoded, "/sources/one.wav\n");
    write_fixture(L"sources/u.m3u8", uri);
    write_fixture(L"sources/c.pl", "one.wav\none.wav\n");
    write_fixture(L"individual.m3u8", "sources/one.wav\n");
    SetEnvironmentVariableW(L"CMUS_HOME", config_dir);
    FreeConsole();
    if (!AllocConsole()) return 4;
    input = CreateFileW(L"CONIN$", GENERIC_READ|GENERIC_WRITE, FILE_SHARE_READ|FILE_SHARE_WRITE,
        NULL, OPEN_EXISTING, 0, NULL);
    screen = CreateFileW(L"CONOUT$", GENERIC_READ|GENERIC_WRITE, FILE_SHARE_READ|FILE_SHARE_WRITE,
        NULL, OPEN_EXISTING, 0, NULL);
    SetHandleInformation(input, HANDLE_FLAG_INHERIT, HANDLE_FLAG_INHERIT);
    SetHandleInformation(screen, HANDLE_FLAG_INHERIT, HANDLE_FLAG_INHERIT);
    PROCESS_INFORMATION process = launch(argv[1]);
    if (!process.hProcess) return 5;
    wchar_t command[32768];
    swprintf(command, 32768, L":cd %ls/sources\r:view 5\r:browser-up\r", base);
    send_text(command);
    send_text(L"a");
    check_count("M3U BOM CRLF", L"a.m3u", 2, ".wav");
    check_count("nested name collision", L"a.m3u.1", 1, "two.wav");
    check_count("PLS", L"b.pls", 2, ".wav");
    check_count("M3U8 percent-encoded local file URI", L"u.m3u8", 1, "one.wav");
    check_count("PL duplicates", L"c.pl", 2, "one.wav");
    swprintf(command, 32768, L":cd %ls\r:view 5\r:win-bottom\r", base);
    send_text(command);
    send_text(L"a");
    check_count("individual import", L"individual.m3u8", 1, "one.wav");
    write_fixture(L"sources/a.m3u", "two.wav\n");
    write_fixture(L"sources/b.pls", "");
    swprintf(path, 32768, L"%ls/sources/sub/a.m3u", base); DeleteFileW(path);
    write_fixture(L"sources/new.M3U8", "two.wav\n");
    send_text(L":update\r:update\r:update\r");
    check_count("update replacement", L"a.m3u", 1, "two.wav");
    check_count("empty file clears", L"b.pls", 0, NULL);
    check_count("missing file removes entry", L"a.m3u.1", -1, NULL);
    check_count("new file discovered", L"new.M3U8", 1, "two.wav");
    expect("playlist UI", snapshot("updated playlists"), "new.M3U8");
    quit_player(process);
    write_fixture(L"sources/a.m3u", "one.wav\n");
    write_fixture(L"sources/restart.pls", "[playlist]\nFile1=one.wav\n");
    swprintf(path, 32768, L"%ls/individual.m3u8", base); DeleteFileW(path);
    process = launch(argv[1]);
    if (!process.hProcess) return 5;
    free(snapshot("restart startup"));
    send_text(L":update\r");
    free(snapshot("restart update"));
    check_count("association survives restart", L"a.m3u", 1, "one.wav");
    check_count("directory survives restart", L"restart.pls", 1, "one.wav");
    check_count("missing individual after restart", L"individual.m3u8", -1, NULL);
    quit_player(process);
    fprintf(report, "playlist source checks: %s\n", failed ? "FAIL" : "PASS");
    CloseHandle(input); CloseHandle(screen); fclose(report); FreeConsole();
    return failed;
}

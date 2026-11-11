/* Real native-console/remote-client regression checks. GPL-2.0-or-later.
 * All configuration, fixtures and output belong to the supplied scratch root. */
#define wmain old_console_main
#include "check_console.c"
#undef wmain
#include <winsock2.h>
#include <ws2tcpip.h>

static wchar_t root[32768], remote_exe[32768], endpoint[512];
static wchar_t test_password[64];
static unsigned checks;

static void verify(const char *label, int ok)
{
    fprintf(report, "%s: %s\n", label, ok ? "PASS" : "FAIL");
    fflush(report); checks++;
    if (!ok) failed = 1;
}

static char *client(const wchar_t *arguments, const char *stdin_text, DWORD *code)
{
    wchar_t output_path[32768], stdin_path[32768], command[32768];
    swprintf(output_path, 32768, L"%ls/client_output.txt", root);
    swprintf(stdin_path, 32768, L"%ls/client_stdin.txt", root);
    FILE *in = _wfopen(stdin_path, L"wb");
    if (!in) { *code = 999; return _strdup(""); }
    if (stdin_text) fwrite(stdin_text, 1, strlen(stdin_text), in);
    fclose(in);
    SECURITY_ATTRIBUTES sa = {sizeof(sa), NULL, TRUE};
    HANDLE out = CreateFileW(output_path, GENERIC_WRITE, FILE_SHARE_READ, &sa, CREATE_ALWAYS, 0, NULL);
    HANDLE source = CreateFileW(stdin_path, GENERIC_READ, FILE_SHARE_READ, &sa, OPEN_EXISTING, 0, NULL);
    STARTUPINFOW start = {.cb = sizeof(start), .dwFlags = STARTF_USESTDHANDLES,
        .hStdInput = source, .hStdOutput = out, .hStdError = out};
    PROCESS_INFORMATION process = {0};
    swprintf(command, 32768, L"\"%ls\" %ls", remote_exe, arguments);
    if (!CreateProcessW(remote_exe, command, NULL, NULL, TRUE, CREATE_NO_WINDOW, NULL, NULL, &start, &process)) {
        *code = 998;
    } else {
        if (WaitForSingleObject(process.hProcess, 7000) != WAIT_OBJECT_0) {
            TerminateProcess(process.hProcess, 997); WaitForSingleObject(process.hProcess, 2000);
        }
        GetExitCodeProcess(process.hProcess, code);
        CloseHandle(process.hThread); CloseHandle(process.hProcess);
    }
    CloseHandle(out); CloseHandle(source);
    FILE *f = _wfopen(output_path, L"rb");
    if (!f) return _strdup("");
    fseek(f, 0, SEEK_END); long size = ftell(f); rewind(f);
    char *text = calloc((size_t)size + 1, 1);
    fread(text, 1, (size_t)size, f); fclose(f);
    return text;
}

static char *run(const wchar_t *arguments)
{
    DWORD code;
    char *text = client(arguments, NULL, &code);
    if (code) { fprintf(report, "client exit %lu: %s\n", code, text); failed = 1; }
    return text;
}

static char *run_password(const wchar_t *arguments)
{
    wchar_t command[32768];
    swprintf(command, 32768, L"--passwd %ls %ls", test_password, arguments);
    return run(command);
}

static PROCESS_INFORMATION player(const wchar_t *exe, const wchar_t *arguments)
{
    SetHandleInformation(input, HANDLE_FLAG_INHERIT, HANDLE_FLAG_INHERIT);
    SetHandleInformation(screen, HANDLE_FLAG_INHERIT, HANDLE_FLAG_INHERIT);
    STARTUPINFOW start = {.cb = sizeof(start), .dwFlags = STARTF_USESTDHANDLES,
        .hStdInput = input, .hStdOutput = screen, .hStdError = screen};
    PROCESS_INFORMATION process = {0};
    wchar_t command[32768];
    swprintf(command, 32768, L"\"%ls\" %ls", exe, arguments);
    if (!CreateProcessW(exe, command, NULL, NULL, TRUE, 0, NULL, NULL, &start, &process)) failed = 1;
    Sleep(800);
    return process;
}

static void stop(PROCESS_INFORMATION process, const wchar_t *command)
{
    free(run(command));
    int ended = WaitForSingleObject(process.hProcess, 5000) == WAIT_OBJECT_0;
    if (!ended) { TerminateProcess(process.hProcess, 996); WaitForSingleObject(process.hProcess, 2000); }
    DWORD code = 996;
    GetExitCodeProcess(process.hProcess, &code);
    verify("player clean exit", ended && code == 0);
    CloseHandle(process.hThread); CloseHandle(process.hProcess);
}

static void rc(const wchar_t *configuration, const char *contents)
{
    wchar_t path[32768];
    CreateDirectoryW(configuration, NULL);
    SetEnvironmentVariableW(L"CMUS_HOME", configuration);
    swprintf(path, 32768, L"%ls/rc", configuration);
    FILE *f = _wfopen(path, L"wb");
    if (!f) { failed = 1; return; }
    fwrite(contents, 1, strlen(contents), f); fclose(f);
}

static unsigned tcp_port(void)
{
    WSADATA data; WSAStartup(MAKEWORD(2,2), &data);
    SOCKET s = socket(AF_INET, SOCK_STREAM, 0);
    struct sockaddr_in address = {.sin_family = AF_INET};
    address.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
    bind(s, (struct sockaddr *)&address, sizeof(address));
    int len = sizeof(address); getsockname(s, (struct sockaddr *)&address, &len);
    closesocket(s);
    return ntohs(address.sin_port);
}

#ifndef CMUS_REMOTE_CHECK_MAIN
#define CMUS_REMOTE_CHECK_MAIN wmain
#endif
int CMUS_REMOTE_CHECK_MAIN(int argc, wchar_t **argv)
{
    if (argc != 5) return 2;
    swprintf(remote_exe, 32768, L"%ls", argv[2]);
    swprintf(root, 32768, L"%ls", argv[3]);
    for (wchar_t *p = root; *p; p++) if (*p == L'\\') *p = L'/';
    report = _wfopen(argv[4], L"wb");
    if (!report) return 3;
    CreateDirectoryW(root, NULL);
    wchar_t config[32768], address[32768];
    swprintf(config, 32768, L"%ls/config \u00fc", root);
    rc(config, "set softvol=true\nset resume=true\n");
    swprintf(address, 32768, L"%ls/socket \u97f3", config);
    SetEnvironmentVariableW(L"CMUS_SOCKET", address);
    FreeConsole(); if (!AllocConsole()) return 4;
    input = CreateFileW(L"CONIN$", GENERIC_READ|GENERIC_WRITE, FILE_SHARE_READ|FILE_SHARE_WRITE, NULL, OPEN_EXISTING, 0, NULL);
    screen = CreateFileW(L"CONOUT$", GENERIC_READ|GENERIC_WRITE, FILE_SHARE_READ|FILE_SHARE_WRITE, NULL, OPEN_EXISTING, 0, NULL);
    PROCESS_INFORMATION process = player(argv[1], L"");
    char *text = run(L"-Q");
    verify("default local endpoint and UTF8 CMUS_SOCKET", strstr(text, "status stopped\n") != NULL); free(text);
    send_text(L":set repeat=false\r");
    free(run(L"-R")); text = run(L"-Q");
    verify("keyboard and cooked repeat share behavior", strstr(text, "set repeat true\n") != NULL); free(text);
    free(run(L"-v 37%")); text = run(L"-Q");
    verify("cooked software volume", strstr(text, "set vol_left 37\n") != NULL); free(text);
    text = run(L"-C \"no-such-command\"");
    verify("command errors reach remote client", strstr(text, "Error:") != NULL); free(text);
    wchar_t long_command[6000];
    wcscpy(long_command, L"-C \"echo ");
    size_t prefix = wcslen(long_command);
    for (int i = 0; i < 4096; i++) long_command[prefix+i] = L'x';
    wcscpy(long_command+prefix+4096, L"\"");
    text = run(long_command);
    verify("raw command exceeds old 512 byte limit", strlen(text) >= 4096); free(text);
    char stdin_text[6000] = "set repeat=false\r\necho ";
    size_t stdin_prefix = strlen(stdin_text);
    memset(stdin_text+stdin_prefix, 'y', 4096); stdin_text[stdin_prefix+4096] = 0;
    DWORD code;
    text = client(L"", stdin_text, &code);
    verify("long CRLF stdin and unterminated final line", code == 0 && strlen(text) >= 4096); free(text);
    text = run(L"-C \"format_print hello world\"");
    verify("local format_print", strstr(text, "hello\nworld\n") != NULL); free(text);

    wchar_t wav[32768]; swprintf(wav, 32768, L"%ls/\u97f3 spaced.wav", root);
    FILE *f = _wfopen(wav, L"wb");
    unsigned bytes = 8000 * 4 * 60, riff = bytes + 36;
    unsigned char header[44] = {'R','I','F','F',0,0,0,0,'W','A','V','E','f','m','t',' ',16,0,0,0,1,0,2,0,0x40,0x1f,0,0,0,0x7d,0,0,4,0,16,0,'d','a','t','a'};
    memcpy(header+4, &riff, 4); memcpy(header+40, &bytes, 4);
    fwrite(header, 1, 44, f); unsigned char silence[4096] = {0};
    for (unsigned n = 0; n < bytes; n += sizeof(silence)) fwrite(silence, 1, bytes-n < sizeof(silence) ? bytes-n : sizeof(silence), f);
    fclose(f);
    wchar_t command[32768]; swprintf(command, 32768, L"-l \"%ls\"", wav); free(run(command));
    Sleep(300); text = run(L"-C \"save -l -\"");
    verify("UTF8 cooked file add and stdout library export", strstr(text, "\xe9\x9f\xb3 spaced.wav") != NULL); free(text);
    swprintf(command, 32768, L"-q \"%ls\"", wav); free(run(command)); Sleep(200);
    text = run(L"-C \"save -q -\""); verify("cooked queue add and export", strstr(text, "spaced.wav") != NULL); free(text);
    free(run(L"-q -c")); text = run(L"-C \"save -q -\""); verify("cooked queue clear", !strstr(text, "spaced.wav")); free(text);
    swprintf(command, 32768, L"-f \"%ls\"", wav); free(run(command)); Sleep(400);
    text = run(L"-Q"); verify("cooked play file", strstr(text, "status playing\n") != NULL); free(text);
    free(run(L"-u")); text = run(L"-Q");
    int paused_position = 0;
    char *paused_at = strstr(text, "position ");
    if (paused_at) sscanf(paused_at, "position %d", &paused_position);
    verify("cooked pause", strstr(text, "status paused\n") != NULL); free(text);
    free(run(L"-k +10"));
    int seek_position = -1;
    for (int i = 0; i < 20; i++) {
        text = run(L"-Q"); char *position = strstr(text, "position ");
        if (position) sscanf(position, "position %d", &seek_position);
        free(text);
        if (seek_position >= paused_position + 9 && seek_position <= paused_position + 11) break;
        Sleep(25);
    }
    fprintf(report, "relative seek position: %d -> %d\n", paused_position, seek_position);
    verify("cooked seek", seek_position >= paused_position + 9 && seek_position <= paused_position + 11);
    free(run(L"-u")); text = run(L"-Q"); verify("cooked resume", strstr(text, "status playing\n") != NULL); free(text);
    free(run(L"-s")); text = run(L"-Q"); verify("cooked stop", strstr(text, "status stopped\n") != NULL); free(text);
    PROCESS_INFORMATION duplicate = player(argv[1], L"");
    int duplicate_exited = WaitForSingleObject(duplicate.hProcess, 2000) == WAIT_OBJECT_0;
    if (!duplicate_exited) { TerminateProcess(duplicate.hProcess, 995); WaitForSingleObject(duplicate.hProcess, 2000); }
    GetExitCodeProcess(duplicate.hProcess, &code);
    verify("duplicate default server rejected", duplicate_exited && code != 0);
    CloseHandle(duplicate.hThread); CloseHandle(duplicate.hProcess);
    stop(process, L"-C quit");
    process = player(argv[1], L"");
    text = run(L"-C \"save -l -\""); verify("restart reconnect and library persistence", strstr(text, "spaced.wav") != NULL); free(text);
    stop(process, L"-C quit");
    text = client(L"-C", NULL, &code); verify("quiet missing server probe", code == 1 && !*text); free(text);

    swprintf(endpoint, 512, L"\\\\.\\pipe\\cmus-protocol-test-%lu", GetCurrentProcessId());
    swprintf(command, 32768, L"--listen \"%ls\"", endpoint);
    SetEnvironmentVariableW(L"CMUS_SOCKET", endpoint);
    process = player(argv[1], command);
    HANDLE pipe = CreateFileW(endpoint, GENERIC_READ|GENERIC_WRITE, 0, NULL, OPEN_EXISTING, 0, NULL);
    if (pipe != INVALID_HANDLE_VALUE) {
        DWORD n; WriteFile(pipe, "set repe", 8, &n, NULL); Sleep(150);
        WriteFile(pipe, "at=true\n", 8, &n, NULL);
        char reply[8]; ReadFile(pipe, reply, sizeof(reply), &n, NULL);
        CloseHandle(pipe);
    }
    text = run(L"-Q"); verify("fragmented native pipe command", pipe != INVALID_HANDLE_VALUE && strstr(text, "set repeat true\n")); free(text);
    wchar_t playlist[32768]; swprintf(playlist, 32768, L"%ls/large_queue.m3u8", root);
    char utf8_wav[65536]; WideCharToMultiByte(CP_UTF8, 0, wav, -1, utf8_wav, sizeof(utf8_wav), NULL, NULL);
    f = _wfopen(playlist, L"wb");
    for (int i = 0; i < 10000; i++) fprintf(f, "%s\n", utf8_wav);
    fclose(f);
    swprintf(command, 32768, L"-q \"%ls\"", playlist); free(run(command));
    int queue_lines = 0;
    for (int i = 0; i < 30; i++) {
        text = run(L"-C \"save -q -\""); queue_lines = 0;
        for (char *p = text; *p; p++) if (*p == '\n') queue_lines++;
        free(text);
        if (queue_lines == 10000) break;
        Sleep(50);
    }
    verify("complete large pipe stdout export", queue_lines == 10000);
    pipe = CreateFileW(endpoint, GENERIC_READ|GENERIC_WRITE, 0, NULL, OPEN_EXISTING, 0, NULL);
    if (pipe != INVALID_HANDLE_VALUE) {
        DWORD written; WriteFile(pipe, "save -q -\n", 10, &written, NULL); Sleep(100);
    }
    ULONGLONG responsive_start = GetTickCount64();
    send_text(L":set repeat=false\r"); text = run(L"-Q");
    verify("slow pipe reader does not stall keyboard or other clients", pipe != INVALID_HANDLE_VALUE &&
        GetTickCount64() - responsive_start < 2000 && strstr(text, "set repeat false\n"));
    free(text); if (pipe != INVALID_HANDLE_VALUE) CloseHandle(pipe);
    text = run(L"-Q"); verify("disconnected slow pipe reader preserves server", strstr(text, "status stopped\n") != NULL); free(text);
    stop(process, L"-C quit");

    swprintf(config, 32768, L"%ls/tcp_config", root);
    LARGE_INTEGER counter;
    QueryPerformanceCounter(&counter);
    swprintf(test_password, 64, L"test_%08lx%08lx", (unsigned long)GetTickCount(),
            (unsigned long)counter.LowPart);
    char rc_contents[160];
    snprintf(rc_contents, sizeof(rc_contents), "set softvol=true\nset passwd=%ls\n",
            test_password);
    rc(config, rc_contents);
    swprintf(endpoint, 512, L"127.0.0.1:%u", tcp_port());
    SetEnvironmentVariableW(L"CMUS_SOCKET", endpoint);
    swprintf(command, 32768, L"--listen %ls", endpoint);
    process = player(argv[1], command);
    text = client(L"--passwd wrong -Q", NULL, &code);
    verify("TCP incorrect password rejected", code == 1 && strstr(text, "authentication failed")); free(text);
    text = run_password(L"-Q"); verify("TCP authenticated status", strstr(text, "status stopped\n") != NULL); free(text);
    text = run_password(L"-C \"run cmus_missing_test_program\"");
    verify("TCP retains upstream silent unsafe-command restriction", !*text); free(text);
    swprintf(command, 32768, L"--passwd %ls -l \"%ls\"", test_password, wav);
    free(run(command)); Sleep(200);
    text = run_password(L"-C \"save -l -\"");
    verify("TCP allows upstream stdout export exception", strstr(text, "spaced.wav") != NULL); free(text);
    swprintf(command, 32768, L"--passwd %ls -C quit", test_password); stop(process, command);
    fprintf(report, "remote checks: %u; result: %s\n", checks, failed ? "FAIL" : "PASS");
    CloseHandle(input); CloseHandle(screen); FreeConsole(); fclose(report);
    return failed;
}

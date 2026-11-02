/* Real native-console library import/navigation/persistence at 50,000 files. */
#define CMUS_PLAYLIST_CHECK_MAIN original_playlist_check_main
#include "check_playlist_sources.c"

static int library_count(void)
{
    wchar_t path[32768];
    swprintf(path, 32768, L"%ls/library.pl", base);
    FILE *f = _wfopen(path, L"rb");
    if (!f) return -1;
    int count = 0, ch;
    while ((ch = fgetc(f)) != EOF) if (ch == '\n') count++;
    fclose(f);
    return count;
}

static int expect_library(int expected)
{
    wchar_t command[32768];
    swprintf(command, 32768, L":save -l %ls/library.pl\r", base);
    int actual = -1;
    ULONGLONG start = GetTickCount64();
    do {
        wchar_t snapshot_path[32768];
        swprintf(snapshot_path, 32768, L"%ls/library.pl", base);
        DeleteFileW(snapshot_path);
        send_text(command);
        actual = library_count();
        if (actual == expected) break;
        Sleep(3000);
    } while (GetTickCount64()-start < 120000);
    fprintf(report, "library count: %s (%d expected %d), %llu ms\n",
        actual == expected ? "PASS" : "FAIL", actual, expected, GetTickCount64()-start);
    if (actual != expected) failed = 1;
    fflush(report);
    return actual == expected;
}

int wmain(int argc, wchar_t **argv)
{
    if (argc != 4) return 2;
    report = _wfopen(argv[3], L"wb");
    if (!report) return 3;
    swprintf(base, 32768, L"%ls", argv[2]);
    for (wchar_t *p = base; *p; p++) if (*p == L'\\') *p = L'/';
    CreateDirectoryW(base, NULL);
    wchar_t path[32768], command[32768];
    swprintf(path, 32768, L"%ls/music", base); CreateDirectoryW(path, NULL);
    swprintf(config_dir, 32768, L"%ls/config", base); CreateDirectoryW(config_dir, NULL);
    write_fixture(L"config/rc", "set softvol=true\n");
    ULONGLONG start = GetTickCount64();
    for (int i = 0; i < 50000; i++) {
        wchar_t name[100];
        swprintf(name, 100, L"music/track_%05d.wav", i);
        wav_fixture(name);
    }
    fprintf(report, "50000 distinct WAV fixtures: %llu ms\n", GetTickCount64()-start);
    SetEnvironmentVariableW(L"CMUS_HOME", config_dir);
    FreeConsole();
    if (!AllocConsole()) return 4;
    input = CreateFileW(L"CONIN$", GENERIC_READ|GENERIC_WRITE, FILE_SHARE_READ|FILE_SHARE_WRITE,
        NULL, OPEN_EXISTING, 0, NULL);
    screen = CreateFileW(L"CONOUT$", GENERIC_READ|GENERIC_WRITE, FILE_SHARE_READ|FILE_SHARE_WRITE,
        NULL, OPEN_EXISTING, 0, NULL);
    SetConsoleScreenBufferSize(screen, (COORD){120,40});
    SMALL_RECT rect = {0,0,119,39}; SetConsoleWindowInfo(screen, TRUE, &rect);
    PROCESS_INFORMATION process = launch(argv[1]);
    if (!process.hProcess) return 5;
    start = GetTickCount64();
    swprintf(command, 32768, L":add -l %ls/music\r", base); send_text(command);
    Sleep(5000);
    if (!expect_library(50000)) {
        free(snapshot("incomplete directory import"));
        goto finish;
    }
    fprintf(report, "directory import/export: %llu ms\n", GetTickCount64()-start);
    send_text(L"2/track_49999\r");
    swprintf(command, 32768, L":win-add-q\r:save -q %ls/queue.pl\r", base);
    send_text(command);
    swprintf(path, 32768, L"%ls/queue.pl", base);
    FILE *queue = _wfopen(path, L"rb");
    char line[65536] = "";
    int search_ok = queue && fgets(line, sizeof(line), queue) && strstr(line, "track_49999.wav");
    if (queue) fclose(queue);
    fprintf(report, "search selection added to queue: %s\n", search_ok ? "PASS" : "FAIL");
    if (!search_ok) failed = 1;
    send_text(L":win-top\r:win-bottom\r");
    expect("populated library", snapshot("navigation"), "50000 tracks");
    send_text(L"1");
    resize_and_check(80, 24);
    resize_and_check(60, 20);
    resize_and_check(80, 24);
    quit_player(process);
    start = GetTickCount64();
    process = launch(argv[1]);
    if (!process.hProcess) return 5;
    if (!expect_library(50000)) goto finish;
    fprintf(report, "restart/export: %llu ms\n", GetTickCount64()-start);
    swprintf(path, 32768, L"%ls/music/track_00000.wav", base);
    if (!DeleteFileW(path)) failed = 1;
    send_text(L":update\r");
    expect_library(49999);
finish:
    quit_player(process);
    fprintf(report, "large library checks: %s\n", failed ? "FAIL" : "PASS");
    CloseHandle(input); CloseHandle(screen); fclose(report); FreeConsole();
    return failed;
}

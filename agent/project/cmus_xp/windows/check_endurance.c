/* Sustained playback, format transitions and failure recovery checks.
 * GPL-2.0-or-later. */
#define CMUS_REMOTE_CHECK_MAIN original_remote_main
#include "check_remote.c"
#undef CMUS_REMOTE_CHECK_MAIN
#include <psapi.h>

static PROCESS_INFORMATION active_process;

static const wchar_t *fixtures[] = {
    L"stereo16.wav", L"mono32.wav", L"surround24.wav", L"surround71.flac",
    L"stereo.opus", L"stereo16.wav",
};

static unsigned long long memory_used(PROCESS_INFORMATION process)
{
    PROCESS_MEMORY_COUNTERS_EX counters;
    if (!GetProcessMemoryInfo(process.hProcess, (PROCESS_MEMORY_COUNTERS *)&counters,
                sizeof(counters)))
        return 0;
    return counters.PrivateUsage;
}

static int wait_status(const char *needle)
{
    for (int i = 0; i < 100; i++) {
        char *text = run(L"-Q");
        int found = strstr(text, needle) != NULL;
        free(text);
        if (found)
            return 1;
        Sleep(30);
    }
    return 0;
}

int wmain(int argc, wchar_t **argv)
{
    if (argc != 5)
        return 2;
    swprintf(remote_exe, 32768, L"%ls", argv[2]);
    swprintf(root, 32768, L"%ls", argv[3]);
    for (wchar_t *p = root; *p; p++) if (*p == L'\\') *p = L'/';
    report = _wfopen(argv[4], L"wb");
    if (!report)
        return 3;

    wchar_t config[32768], address[32768], command[32768];
    swprintf(config, 32768, L"%ls/config", root);
    rc(config, "set softvol=true\nset softvol_state=0 0\nset resume=false\nset repeat=false\n");
    swprintf(address, 32768, L"%ls/socket", config);
    SetEnvironmentVariableW(L"CMUS_SOCKET", address);
    FreeConsole();
    if (!AllocConsole())
        return 4;
    input = CreateFileW(L"CONIN$", GENERIC_READ | GENERIC_WRITE,
            FILE_SHARE_READ | FILE_SHARE_WRITE, NULL, OPEN_EXISTING, 0, NULL);
    screen = CreateFileW(L"CONOUT$", GENERIC_READ | GENERIC_WRITE,
            FILE_SHARE_READ | FILE_SHARE_WRITE, NULL, OPEN_EXISTING, 0, NULL);

    active_process = player(argv[1], L"");
    Sleep(300);
    unsigned long long baseline = memory_used(active_process);

    for (int cycle = 0; cycle < 60; cycle++) {
        const wchar_t *fixture = fixtures[cycle % (sizeof(fixtures) / sizeof(*fixtures))];
        swprintf(command, 32768, L"-f \"%ls/%ls\"", root, fixture);
        free(run(command));
        if (!wait_status("status playing\n")) {
            verify("endurance playback starts", 0);
            goto done;
        }
        free(run(L"-u"));
        Sleep(20);
        free(run(L"-k +1"));
        free(run(L"-u"));
        free(run(L"-C \"toggle repeat\""));
        free(run(L"-s"));
        if ((cycle % 10) == 0) {
            unsigned long long used = memory_used(active_process);
            fprintf(report, "cycle %d memory %llu baseline %llu\n", cycle,
                    (unsigned long long)used, baseline);
            fflush(report);
        }
    }
    verify("60 mixed-format playback cycles", !failed);
    unsigned long long final_memory = memory_used(active_process);
    verify("memory growth bounded", final_memory > baseline && final_memory - baseline < 48ULL * 1024 * 1024);

    /* Truncate a file while it is being read. The player must remain responsive
     * and recoverable rather than deadlocking or crashing. */
    swprintf(command, 32768, L"-f \"%ls/long.wav\"", root);
    free(run(command));
    if (wait_status("status playing\n")) {
        wchar_t path[32768];
        swprintf(path, 32768, L"%ls/long.wav", root);
        HANDLE truncate = CreateFileW(path, GENERIC_WRITE, FILE_SHARE_READ | FILE_SHARE_WRITE,
                NULL, OPEN_EXISTING, 0, NULL);
        verify("truncation fixture open", truncate != INVALID_HANDLE_VALUE);
        if (truncate != INVALID_HANDLE_VALUE) {
            SetEndOfFile(truncate);
            CloseHandle(truncate);
        }
        Sleep(800);
        char *text = run(L"-Q");
        verify("player responsive after truncated input", strstr(text, "status ") != NULL);
        free(text);
        free(run(L"-s"));
    } else {
        verify("long fixture playback", 0);
    }

    for (int i = 0; i < 40; i++)
        free(run(L"-C \"toggle repeat\""));
    char *text = run(L"-Q");
    verify("rapid command storm leaves player responsive", strstr(text, "status ") != NULL);
    free(text);

done:
    stop(active_process, L"-C quit");
    fprintf(report, "endurance checks: %u; result: %s\n", checks, failed ? "FAIL" : "PASS");
    CloseHandle(input);
    CloseHandle(screen);
    FreeConsole();
    fclose(report);
    return failed;
}

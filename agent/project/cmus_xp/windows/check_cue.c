/* Actual native console, import, transport and restart CUE regression checks. */
#define CMUS_REMOTE_CHECK_MAIN original_remote_main
#include "check_remote.c"
#undef CMUS_REMOTE_CHECK_MAIN

static char *wait_for(const char *needle)
{
    for (int i = 0; i < 60; i++) {
        char *text = run(L"-Q");
        if (strstr(text, needle)) return text;
        free(text); Sleep(50);
    }
    return run(L"-Q");
}

static int library_count(void)
{
    char *text = run(L"-C \"save -l -\"");
    int count = 0;
    for (char *p = text; *p; p++) if (*p == '\n') count++;
    free(text); return count;
}

static void extended_path(const wchar_t *path, wchar_t *out)
{
    swprintf(out, 32768, L"\\\\?\\%ls", path);
    for (wchar_t *p = out; *p; p++) if (*p == L'/') *p = L'\\';
}

static void check_long_cue(void)
{
    wchar_t directory[32768], extended[32768], source[32768], destination[32768], command[32768];
    swprintf(directory, 32768, L"%ls/long", root);
    extended_path(directory, extended);
    int created = CreateDirectoryW(extended, NULL) || GetLastError() == ERROR_ALREADY_EXISTS;
    for (int level = 0; level < 40 && created; level++) {
        size_t length = wcslen(directory); directory[length++] = L'/';
        for (int i = 0; i < 40; i++) directory[length++] = L'\u97f3';
        directory[length] = 0;
        extended_path(directory, extended);
        created = CreateDirectoryW(extended, NULL) || GetLastError() == ERROR_ALREADY_EXISTS;
    }
    const wchar_t *files[] = {L"audio \u00fc.wav", L"album.CUE", NULL};
    for (int i = 0; files[i] && created; i++) {
        swprintf(source, 32768, L"%ls/disc/%ls", root, files[i]);
        swprintf(destination, 32768, L"%ls/%ls", directory, files[i]);
        extended_path(destination, extended);
        created = CopyFileW(source, extended, TRUE);
    }
    verify("private CUE fixture exceeds 4096 UTF8 bytes", created &&
        WideCharToMultiByte(CP_UTF8, 0, directory, -1, NULL, 0, NULL, NULL) > 4096);
    if (!created) return;
    free(run(L"-C \"clear -l\""));
    swprintf(command, 32768, L"-C \"add -l %ls/album.CUE\"", directory); free(run(command));
    for (int i = 0; i < 40 && library_count() != 2; i++) Sleep(100);
    verify("CUE URL/import preserves paths beyond 4096 UTF8 bytes", library_count() == 2);
    swprintf(command, 32768, L"-f \"cue://%ls/album.CUE/2\"", directory); free(run(command));
    char *text = wait_for("tag title Second\n");
    verify("long Unicode CUE path playback", strstr(text, "tag title Second\n") && strstr(text, "status playing\n"));
    free(text); free(run(L"-s"));
    int removed = 1;
    for (int i = 0; files[i]; i++) {
        swprintf(destination, 32768, L"%ls/%ls", directory, files[i]);
        extended_path(destination, extended);
        if (!DeleteFileW(extended)) removed = 0;
    }
    for (int level = 0; level <= 40; level++) {
        extended_path(directory, extended);
        if (!RemoveDirectoryW(extended)) removed = 0;
        wchar_t *slash = wcsrchr(directory, L'/');
        if (slash) *slash = 0;
    }
    verify("long-path fixture cleanup", removed);
}

int wmain(int argc, wchar_t **argv)
{
    if (argc != 5) return 2;
    swprintf(remote_exe, 32768, L"%ls", argv[2]);
    swprintf(root, 32768, L"%ls", argv[3]);
    for (wchar_t *p = root; *p; p++) if (*p == L'\\') *p = L'/';
    report = _wfopen(argv[4], L"wb"); if (!report) return 3;
    wchar_t config[32768], address[32768], command[32768];
    swprintf(config, 32768, L"%ls/config", root);
    rc(config, "set softvol=true\nset softvol_state=0 0\nset resume=false\n");
    swprintf(address, 32768, L"%ls/socket", config); SetEnvironmentVariableW(L"CMUS_SOCKET", address);
    FreeConsole(); if (!AllocConsole()) return 4;
    input = CreateFileW(L"CONIN$", GENERIC_READ|GENERIC_WRITE, FILE_SHARE_READ|FILE_SHARE_WRITE, NULL, OPEN_EXISTING, 0, NULL);
    screen = CreateFileW(L"CONOUT$", GENERIC_READ|GENERIC_WRITE, FILE_SHARE_READ|FILE_SHARE_WRITE, NULL, OPEN_EXISTING, 0, NULL);
    PROCESS_INFORMATION process = player(argv[1], L"");
    swprintf(command, 32768, L":add %ls/disc/album.CUE\r", root); send_text(command);
    for (int i = 0; i < 30 && library_count() != 2; i++) Sleep(100);
    verify("typed uppercase CUE import creates two tracks", library_count() == 2);
    free(run(L"-C \"clear -l\""));
    swprintf(command, 32768, L"-C \"cd %ls/browser\" \"view 5\" \"win-top\" \"win-down\"", root);
    free(run(command)); send_text(L"a");
    for (int i = 0; i < 30 && library_count() != 2; i++) Sleep(100);
    verify("browser a imports individual CUE", library_count() == 2);
    free(run(L"-C \"clear -l\""));
    swprintf(command, 32768, L"-C \"add -l %ls/disc\"", root); free(run(command));
    for (int i = 0; i < 30 && library_count() != 2; i++) Sleep(100);
    verify("directory CUE import suppresses backing audio duplicate", library_count() == 2);
    const wchar_t *targets[] = {L"p", L"q", NULL};
    for (int i = 0; targets[i]; i++) {
        swprintf(command, 32768, L"-C \"add -%ls %ls/disc/album.CUE\"", targets[i], root); free(run(command));
        int count = 0;
        for (int attempt = 0; attempt < 40; attempt++) {
            swprintf(command, 32768, L"-C \"save -%ls -\"", targets[i]);
            char *entries = run(command); count = 0;
            for (char *p = entries; *p; p++) if (*p == '\n') count++;
            free(entries);
            if (count == 2) break;
            Sleep(100);
        }
        verify("CUE playlist/queue import expands tracks", count == 2);
    }
    free(run(L"-C \"clear -q\""));
    swprintf(command, 32768, L"-f \"cue://%ls/disc/album.CUE/2\"", root); free(run(command));
    char *text = wait_for("tag title Second\n");
    verify("CUE track metadata and native playback", strstr(text, "tag title Second\n") && strstr(text, "status playing\n"));
    verify("last CUE track duration", strstr(text, "duration 20\n") != NULL); free(text);
    free(run(L"-u")); text = wait_for("status paused\n");
    verify("CUE pause", strstr(text, "status paused\n") != NULL); free(text);
    free(run(L"-k +4")); text = run(L"-Q");
    int position = -1; char *p = strstr(text, "position "); if (p) sscanf(p, "position %d", &position);
    verify("CUE seek is relative to track start", position >= 4 && position < 8); free(text);
    free(run(L"-u")); text = wait_for("status playing\n");
    verify("CUE resume", strstr(text, "status playing\n") != NULL); free(text);
    free(run(L"-s"));
    swprintf(command, 32768, L"-f \"cue://%ls/disc/album.CUE/1\"", root); free(run(command));
    text = wait_for("tag title First\n");
    verify("first CUE track metadata and boundary duration", strstr(text, "tag title First\n") && strstr(text, "duration 10\n")); free(text);
    free(run(L"-C \"view 2\" \"win-top\"")); send_text(L"\r");
    text = wait_for("tag title First\n"); free(text);
    free(run(L"-k 9"));
    text = wait_for("tag title Second\n");
    verify("Enter playback advances across CUE track boundary", strstr(text, "tag title Second\n") && strstr(text, "status playing\n")); free(text);
    free(run(L"-s"));
    swprintf(command, 32768, L"-f \"cue://%ls/compressed/album.cue/2\"", root); free(run(command));
    text = wait_for("tag title Second\n");
    verify("FLAC-backed CUE playback", strstr(text, "tag title Second\n") && strstr(text, "status playing\n")); free(text);
    free(run(L"-s"));
    wchar_t absolute_cue[32768], absolute_audio[32768];
    swprintf(absolute_cue, 32768, L"%ls/absolute.cue", root);
    swprintf(absolute_audio, 32768, L"%ls/disc/audio \u00fc.wav", root);
    int bytes = WideCharToMultiByte(CP_UTF8, 0, absolute_audio, -1, NULL, 0, NULL, NULL);
    char *utf8 = calloc(bytes, 1);
    WideCharToMultiByte(CP_UTF8, 0, absolute_audio, -1, utf8, bytes, NULL, NULL);
    FILE *cue = _wfopen(absolute_cue, L"wb");
    if (cue) {
        fprintf(cue, "FILE \"%s\" WAVE\nTRACK 01 AUDIO\nTITLE \"Absolute\"\nINDEX 01 00:00:00\n", utf8);
        fclose(cue);
    }
    free(utf8);
    swprintf(command, 32768, L"-f \"cue://%ls/1\"", absolute_cue); free(run(command));
    text = wait_for("tag title Absolute\n");
    verify("CUE FILE accepts absolute Windows drive path", cue && strstr(text, "tag title Absolute\n") && strstr(text, "status playing\n")); free(text);
    free(run(L"-s"));
    stop(process, L"-C quit");
    process = player(argv[1], L"");
    for (int i = 0; i < 30 && library_count() != 2; i++) Sleep(100);
    verify("CUE library survives restart", library_count() == 2);
    swprintf(command, 32768, L":player-play cue://%ls/disc/album.CUE/1\r", root); send_text(command);
    text = wait_for("tag title First\n");
    verify("typed CUE playback after restart", strstr(text, "tag title First\n") && strstr(text, "status playing\n")); free(text);
    free(run(L"-s"));
    check_long_cue();
    stop(process, L"-C quit");
    fprintf(report, "CUE checks: %u; result: %s\n", checks, failed ? "FAIL" : "PASS");
    CloseHandle(input); CloseHandle(screen); FreeConsole(); fclose(report);
    return failed;
}

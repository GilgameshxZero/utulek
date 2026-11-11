/* Real player/UI/remote streaming checks against controlled HTTP fixtures.
 * Reuse the native console and CLI process helpers. GPL-2.0-or-later. */
#define CMUS_REMOTE_CHECK_MAIN original_remote_main
#include "check_remote.c"
#undef CMUS_REMOTE_CHECK_MAIN

static int status_position(char *text)
{
    int position = -1;
    char *p = strstr(text, "position ");
    if (p) sscanf(p, "position %d", &position);
    return position;
}

static int wait_state(const char *state, int minimum_position, char **last)
{
    for (int i = 0; i < 60; i++) {
        char *text = run(L"-Q");
        if (strstr(text, state) && status_position(text) >= minimum_position) {
            *last = text; return 1;
        }
        free(text); Sleep(50);
    }
    *last = run(L"-Q"); return 0;
}

static void start_url(const wchar_t *base, const wchar_t *path)
{
    wchar_t command[32768];
    swprintf(command, 32768, L"-f \"%ls%ls\"", base, path);
    free(run(command));
}

int wmain(int argc, wchar_t **argv)
{
    if (argc != 7) return 2;
    swprintf(remote_exe, 32768, L"%ls", argv[2]);
    swprintf(root, 32768, L"%ls", argv[3]);
    for (wchar_t *p = root; *p; p++) if (*p == L'\\') *p = L'/';
    report = _wfopen(argv[4], L"wb"); if (!report) return 3;
    CreateDirectoryW(root, NULL);
    wchar_t config[32768], address[32768];
    swprintf(config, 32768, L"%ls/config", root);
    rc(config, "set softvol=true\nset resume=false\n");
    swprintf(address, 32768, L"%ls/socket", config); SetEnvironmentVariableW(L"CMUS_SOCKET", address);
    FreeConsole(); if (!AllocConsole()) return 4;
    input = CreateFileW(L"CONIN$", GENERIC_READ|GENERIC_WRITE, FILE_SHARE_READ|FILE_SHARE_WRITE, NULL, OPEN_EXISTING, 0, NULL);
    screen = CreateFileW(L"CONOUT$", GENERIC_READ|GENERIC_WRITE, FILE_SHARE_READ|FILE_SHARE_WRITE, NULL, OPEN_EXISTING, 0, NULL);
    PROCESS_INFORMATION process = player(argv[1], L"");
    const wchar_t *paths[] = {L"/audio.mp3", L"/audio.opus", L"/audio.flac", L"/redirect", L"/chunked.mp3", L"/list.m3u8", L"/list.pls", L"/icy.mp3", L"/legacy_icy.mp3", L"/mono44.mp3", L"/surround51.flac", L"/surround71.flac", L"/audio.aac", L"/audio.ogg", NULL};
    const char *labels[] = {"HTTP MP3", "HTTP Opus", "HTTP FLAC", "relative HTTP redirect", "chunked audio", "M3U8 BOM relative fallback", "PLS indexed fallback", "ICY metadata", "legacy ICY status", "mono 44.1kHz stream", "native 5.1 stream", "native 7.1 stream", "HTTP AAC", "HTTP Vorbis"};
    for (int i = 0; paths[i]; i++) {
        ULONGLONG start = GetTickCount64(); start_url(argv[5], paths[i]);
        verify("play request returns without network blocking", GetTickCount64() - start < 1500);
        char *text;
        int playing = wait_state("status playing\n", 1, &text);
        verify(labels[i], playing);
        if (i >= 9 && i <= 11) {
            const char *expected[] = {"output_channels 1\n", "output_channels 6\n", "output_channels 8\n"};
            verify("stream native channel count reaches output", strstr(text, expected[i - 9]) != NULL);
            verify("stream native rate reaches output", strstr(text, i == 9 ? "output_sample_rate 44100\n" : "output_sample_rate 48000\n") != NULL);
        }
        if (i == 0) verify("stream embedded title", strstr(text, "tag title Network tone\n") != NULL);
        if (i == 7) {
            verify("stream ICY title surfaced in status", strstr(text, "stream Fixture artist - Fixture title\n") != NULL);
            verify("station genre surfaced in tags", strstr(text, "tag genre Test genre\n") != NULL);
        }
        if (!playing) fprintf(report, "state: %s\n", text);
        free(text);
        start = GetTickCount64(); free(run(L"-s"));
        text = run(L"-Q");
        verify("stream stop is responsive", GetTickCount64() - start < 1500 && strstr(text, "status stopped\n"));
        free(text);
    }
    const wchar_t *switches[] = {L"/surround51.flac", L"/mono44.mp3", L"/surround71.flac", L"/audio.mp3", NULL};
    const char *switch_channels[] = {"output_channels 6\n", "output_channels 1\n", "output_channels 8\n", "output_channels 2\n"};
    for (int i = 0; switches[i]; i++) {
        start_url(argv[5], switches[i]); char *state;
        int playing = wait_state("status playing\n", 1, &state);
        verify("active radio switch uses newly negotiated native format", playing && strstr(state, switch_channels[i]));
        free(state);
    }
    free(run(L"-s"));
    for (int attempt = 0; attempt < 10; attempt++) {
        start_url(argv[5], attempt % 2 ? L"/surround51.flac" : L"/mono44.mp3");
        free(run(L"-C \"set dsp.waveout.device=default\" \"player-stop\""));
        char *state = run(L"-Q");
        verify("format opening/device option/cancel stress", strstr(state, "status stopped\n") != NULL);
        free(state);
    }
    const wchar_t *stalls[] = {L"/stall_headers", L"/stall_body", NULL};
    for (int i = 0; stalls[i]; i++) {
        start_url(argv[5], stalls[i]); Sleep(150);
        ULONGLONG start = GetTickCount64(); char *text = run(L"-C \"echo responsive\"");
        verify("UI/remote responsive during pending network request", GetTickCount64() - start < 1500 && strstr(text, "responsive")); free(text);
        start = GetTickCount64(); free(run(L"-s"));
        text = run(L"-Q");
        verify("pending network cancellation is bounded", GetTickCount64() - start < 1500 && strstr(text, "status stopped\n")); free(text);
    }
    const wchar_t *invalid_lists[] = {L"/huge.m3u", L"/nested.m3u", L"/hls.m3u8", NULL};
    for (int i = 0; invalid_lists[i]; i++) {
        start_url(argv[5], invalid_lists[i]);
        char *state; int failed_cleanly = wait_state("status stopped\n", -1, &state);
        verify("unsupported/oversized/recursive playlist fails cleanly", failed_cleanly); free(state);
    }
    start_url(argv[5], L"/missing");
    char *text; int stopped = wait_state("status stopped\n", -1, &text);
    verify("HTTP 404 fails cleanly", stopped); free(text);
    start_url(argv[5], L"/redirect_loop");
    stopped = wait_state("status stopped\n", -1, &text);
    verify("redirect loop terminates", stopped); free(text);
    start_url(argv[6], L"/audio.mp3");
    stopped = wait_state("status stopped\n", -1, &text);
    verify("untrusted TLS certificate rejected", stopped); free(text);
    stop(process, L"-C quit");
    wchar_t certificates[32768];
    swprintf(certificates, 32768, L"%ls/../test_cert.pem", root);
    SetEnvironmentVariableW(L"SSL_CERT_FILE", certificates);
    process = player(argv[1], L"");
    start_url(argv[6], L"/audio.mp3");
    int secure_playing = wait_state("status playing\n", 1, &text);
    verify("verified HTTPS with explicit private CA", secure_playing); free(text);
    free(run(L"-s"));
    wchar_t typed[32768];
    swprintf(typed, 32768, L":player-play %ls/audio.mp3\r", argv[5]);
    send_text(typed);
    int typed_playing = wait_state("status playing\n", 1, &text);
    verify("typed playback command shares streaming behavior", typed_playing); free(text);
    free(run(L"-s"));
    start_url(argv[5], L"/audio.mp3");
    int playing = wait_state("status playing\n", 1, &text); free(text);
    verify("valid stream works after failed requests", playing);
    free(run(L"-u")); text = run(L"-Q");
    verify("radio pause follows upstream stop behavior", strstr(text, "status stopped\n") != NULL); free(text);
    start_url(argv[5], L"/stall_headers"); Sleep(100);
    ULONGLONG quit_start = GetTickCount64(); stop(process, L"-C quit");
    verify("quit cancels pending network request", GetTickCount64() - quit_start < 2000);
    fprintf(report, "streaming checks: %u; result: %s\n", checks, failed ? "FAIL" : "PASS");
    CloseHandle(input); CloseHandle(screen); FreeConsole(); fclose(report);
    return failed;
}

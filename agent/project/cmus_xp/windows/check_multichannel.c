/* Real native output, format transitions and isolated device-selection checks. */
#define CMUS_REMOTE_CHECK_MAIN original_remote_main
#include "check_remote.c"
#undef CMUS_REMOTE_CHECK_MAIN
#include <mmsystem.h>
#include <mmddk.h>

static char *wait_for(const char *needle)
{
    for (int i = 0; i < 80; i++) {
        char *text = run(L"-Q");
        if (strstr(text, needle)) return text;
        free(text); Sleep(50);
    }
    return run(L"-Q");
}

static char *wait_format(int channels, int bits)
{
    char channel_text[64], bits_text[64];
    snprintf(channel_text, sizeof(channel_text), "output_channels %d\n", channels);
    snprintf(bits_text, sizeof(bits_text), "output_bits %d\n", bits);
    for (int i = 0; i < 80; i++) {
        char *text = run(L"-Q"); int position = -1;
        char *p = strstr(text, "position "); if (p) sscanf(p, "position %d", &position);
        if (strstr(text, "status playing\n") && strstr(text, channel_text) && strstr(text, bits_text) && position >= 1) return text;
        free(text); Sleep(50);
    }
    return run(L"-Q");
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
    const wchar_t *files[] = {L"stereo16.wav", L"mono32.wav", L"surround8.wav", L"surround16.wav", L"surround24.wav", L"surround32.wav", L"surround71.flac", L"stereo16.wav", NULL};
    const int channels[] = {2, 1, 6, 6, 6, 6, 8, 2};
    const int bits[] = {16, 32, 8, 16, 24, 32, 16, 16};
    for (int i = 0; files[i]; i++) {
        fprintf(report, "fixture: %ls\n", files[i]);
        swprintf(command, 32768, L"-f \"%ls/%ls\"", root, files[i]); free(run(command));
        char expected[80]; snprintf(expected, sizeof(expected), "output_channels %d\n", channels[i]);
        char *text = wait_format(channels[i], bits[i]);
        verify("native local channel count reaches WaveOut", strstr(text, expected) && strstr(text, "status playing\n"));
        snprintf(expected, sizeof(expected), "output_bits %d\n", bits[i]);
        verify("native local PCM depth reaches WaveOut", strstr(text, expected) != NULL); free(text);
        if (failed) goto done;
        free(run(L"-u")); text = wait_for("status paused\n");
        verify("native-format pause", strstr(text, "status paused\n") != NULL); free(text);
        free(run(L"-k +4")); free(run(L"-u")); text = wait_for("status playing\n");
        verify("native-format seek/resume", strstr(text, "status playing\n") != NULL); free(text);
        free(run(L"-s"));
    }
    free(run(L"-C \"set dsp.waveout.device=#0\""));
    swprintf(command, 32768, L"-f \"%ls/stereo16.wav\"", root); free(run(command));
    char *text = wait_for("status playing\n");
    verify("explicit device index playback", strstr(text, "status playing\n") != NULL); free(text);
    text = run(L"-C \"set dsp.waveout.device=default\"");
    verify("device change while playing reports busy", strstr(text, "Error:") != NULL); free(text);
    free(run(L"-s"));
    DWORD size = 0;
    int queried = waveOutMessage((HWAVEOUT)(UINT_PTR)0, DRV_QUERYFUNCTIONINSTANCEIDSIZE, (DWORD_PTR)&size, 0) == 0 && size && size < 65536;
    wchar_t *endpoint = queried ? calloc(1, size + sizeof(wchar_t)) : NULL;
    queried = endpoint && waveOutMessage((HWAVEOUT)(UINT_PTR)0, DRV_QUERYFUNCTIONINSTANCEID, (DWORD_PTR)endpoint, size) == 0;
    verify("native stable endpoint identifier is available", queried);
    if (queried) {
        swprintf(command, 32768, L"-C \"set dsp.waveout.device=id:%ls\"", endpoint); free(run(command));
        swprintf(command, 32768, L"-f \"%ls/surround16.wav\"", root); free(run(command));
        text = wait_for("output_channels 6\n");
        verify("stable endpoint selection plays native surround", strstr(text, "status playing\n") && strstr(text, "output_channels 6\n")); free(text);
        free(run(L"-s"));
    }
    free(endpoint);
    free(run(L"-C \"set dsp.waveout.device=missing-test-device\""));
    swprintf(command, 32768, L"-f \"%ls/stereo16.wav\"", root); free(run(command));
    text = wait_for("status stopped\n");
    verify("missing selected device fails cleanly", strstr(text, "status stopped\n") != NULL); free(text);
    stop(process, L"-C quit");
    process = player(argv[1], L"");
    text = run(L"-C \"set dsp.waveout.device\"");
    verify("unavailable device setting survives restart without startup prompt", strstr(text, "missing-test-device") != NULL); free(text);
    free(run(L"-C \"set dsp.waveout.device=default\""));
    swprintf(command, 32768, L"-f \"%ls/stereo16.wav\"", root); free(run(command));
    text = wait_for("status playing\n");
    verify("default device recovery after missing selection/restart", strstr(text, "status playing\n") != NULL); free(text);
done:
    free(run(L"-s")); stop(process, L"-C quit");
    fprintf(report, "Native output checks: %u; result: %s\n", checks, failed ? "FAIL" : "PASS");
    CloseHandle(input); CloseHandle(screen); FreeConsole(); fclose(report);
    return failed;
}

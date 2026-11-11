/* Resolve a fresh WinMM device ID on every open, retaining stable identifiers.
 * Enumeration reads the audio configuration without changing defaults. */
#include <windows.h>
#include <mmsystem.h>
#include <mmddk.h>
#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <strings.h>
#include "compat.h"
#include "audio_devices.h"

static int print_utf8(const char *line)
{
    HANDLE output = GetStdHandle(STD_OUTPUT_HANDLE);
    DWORD mode, written;
    if (GetConsoleMode(output, &mode)) {
        wchar_t *wide = win_wide(line);
        if (!wide) return -1;
        size_t length = wcslen(wide);
        if (length && wide[length - 1] == L'\n') {
            wchar_t *expanded = realloc(wide, (length + 2) * sizeof(*wide));
            if (!expanded) { free(wide); return -1; }
            wide = expanded; wide[length - 1] = L'\r'; wide[length++] = L'\n'; wide[length] = 0;
        }
        int ok = WriteConsoleW(output, wide, (DWORD)length, &written, NULL) && written == length;
        free(wide); return ok ? 0 : -1;
    }
    return fputs(line, stdout) < 0 ? -1 : 0;
}

static wchar_t *endpoint_id(unsigned id)
{
    DWORD size = 0;
    if (waveOutMessage((HWAVEOUT)(UINT_PTR)id, DRV_QUERYFUNCTIONINSTANCEIDSIZE,
            (DWORD_PTR)&size, 0) || size < sizeof(wchar_t) || size > 65536) return NULL;
    wchar_t *value = calloc(1, size + sizeof(wchar_t));
    if (!value) return NULL;
    if (waveOutMessage((HWAVEOUT)(UINT_PTR)id, DRV_QUERYFUNCTIONINSTANCEID,
            (DWORD_PTR)value, size) || !*value) { free(value); return NULL; }
    return value;
}

int win_waveout_device_id(const char *selection, unsigned int *id)
{
    if (!selection || !*selection || !strcasecmp(selection, "default")) {
        *id = WAVE_MAPPER; return 0;
    }
    unsigned count = waveOutGetNumDevs();
    if (selection[0] == '#') {
        char *end; errno = 0;
        unsigned long number = strtoul(selection + 1, &end, 10);
        if (errno || end == selection + 1 || *end || number >= count) { errno = ENODEV; return -1; }
        *id = number; return 0;
    }
    int stable = !strncmp(selection, "id:", 3);
    wchar_t *requested = win_wide(selection + (stable ? 3 : 0));
    if (!requested) return -1;
    int found = 0;
    for (unsigned index = 0; index < count; index++) {
        WAVEOUTCAPSW caps;
        wchar_t *endpoint = stable ? endpoint_id(index) : NULL;
        const wchar_t *name = endpoint;
        if (!stable && !waveOutGetDevCapsW(index, &caps, sizeof(caps))) name = caps.szPname;
        if (name && CompareStringOrdinal(name, -1, requested, -1, TRUE) == CSTR_EQUAL) {
            *id = index; found++;
        }
        free(endpoint);
    }
    free(requested);
    if (found == 1) return 0;
    errno = found ? EINVAL : ENODEV;
    return -1;
}

int win_list_output_devices(void)
{
    if (print_utf8("default\tWindows default playback device\n")) return 1;
    unsigned count = waveOutGetNumDevs();
    for (unsigned index = 0; index < count; index++) {
        WAVEOUTCAPSW caps;
        if (waveOutGetDevCapsW(index, &caps, sizeof(caps))) continue;
        char *name = win_utf8(caps.szPname);
        wchar_t *endpoint = endpoint_id(index);
        char *stable = endpoint ? win_utf8(endpoint) : NULL;
        size_t size = (name ? strlen(name) : 0) + (stable ? strlen(stable) : 0) + 64;
        char *line = malloc(size);
        int rc = -1;
        if (line) {
            snprintf(line, size, "#%u\t%s\t%s%s\n", index, name ? name : "", stable ? "id:" : "", stable ? stable : "");
            rc = print_utf8(line);
        }
        free(line);
        free(name); free(endpoint); free(stable);
        if (rc) return 1;
    }
    return 0;
}

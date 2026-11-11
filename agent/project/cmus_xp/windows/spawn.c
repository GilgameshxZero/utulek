/* Native UTF-8 external commands. GPL-2.0-or-later. */
#include "../spawn.h"
#include "compat.h"
#include <errno.h>
#include <wchar.h>

#define COMMAND_CAPACITY 32767



static int append(wchar_t *line, size_t *used, wchar_t c)
{
    if (*used >= COMMAND_CAPACITY - 1) { errno = E2BIG; return -1; }
    line[(*used)++] = c;
    line[*used] = 0;
    return 0;
}

/* CRT argv escaping: backslashes are doubled only before quotes or the
 * closing quote. Shell metacharacters are ordinary executable arguments. */
static int argument(wchar_t *line, size_t *used, const wchar_t *arg)
{
    if (*used && append(line, used, L' ')) return -1;
    if (append(line, used, L'"')) return -1;
    for (;;) {
        size_t slashes = 0;
        while (*arg == L'\\') { slashes++; arg++; }
        size_t count = (*arg == L'"' || !*arg) ? slashes * 2 : slashes;
        while (count--) if (append(line, used, L'\\')) return -1;
        if (!*arg) break;
        if (*arg == L'"' && append(line, used, L'\\')) return -1;
        if (append(line, used, *arg++)) return -1;
    }
    return append(line, used, L'"');
}

static int launch(wchar_t *line, const wchar_t *application, int *status, int wait)
{
    STARTUPINFOW startup = { .cb = sizeof(startup) };
    PROCESS_INFORMATION process;
    /* No inherited player handles or console: a hook cannot steal input or
     * corrupt the display. Files/pipes requested by cmd are opened by cmd. */
    if (!CreateProcessW(application, line, NULL, NULL, FALSE, CREATE_NO_WINDOW,
                NULL, NULL, &startup, &process)) {
        DWORD error = GetLastError();
        errno = error == ERROR_FILE_NOT_FOUND || error == ERROR_PATH_NOT_FOUND ? ENOENT :
            error == ERROR_ACCESS_DENIED ? EACCES :
            error == ERROR_BAD_EXE_FORMAT ? ENOEXEC : EIO;
        return -1;
    }
    CloseHandle(process.hThread);
    int result = 0;
    if (wait) {
        DWORD code;
        if (WaitForSingleObject(process.hProcess, INFINITE) != WAIT_OBJECT_0 ||
                !GetExitCodeProcess(process.hProcess, &code)) {
            errno = EIO; result = -1;
        } else if (status) *status = (code & 255) << 8;
    }
    CloseHandle(process.hProcess);
    return result;
}

static int is_batch(const wchar_t *application)
{
    const wchar_t *dot = wcsrchr(application, L'.');
    return dot && (!_wcsicmp(dot, L".bat") || !_wcsicmp(dot, L".cmd"));
}

static int launch_batch(char *argv[], int *status, int wait)
{
    wchar_t *shell = win_wide(getenv("COMSPEC") ? getenv("COMSPEC") : "cmd.exe");
    wchar_t *line = calloc(COMMAND_CAPACITY, sizeof(*line));
    size_t used = 0;
    int result = -1;
    if (!shell || !line) { errno = ENOMEM; goto done; }
    if (argument(line, &used, shell)) goto done;
    const wchar_t *prefix = L" /d /s /c \"";
    while (*prefix) if (append(line, &used, *prefix++)) goto done;
    for (size_t i = 0; argv[i]; i++) {
        wchar_t *wide = win_wide(argv[i]);
        if (!wide) goto done;
        int error = argument(line, &used, wide);
        free(wide);
        if (error) goto done;
    }
    if (append(line, &used, L'"')) goto done;
    result = launch(line, shell, status, wait);
done:
    free(shell);
    free(line);
    return result;
}

int spawn(char *argv[], int *status, int do_wait)
{
    if (!argv || !argv[0] || !argv[0][0]) { errno = EINVAL; return -1; }
    wchar_t *first = win_wide(argv[0]);
    if (!first) return -1;
    int batch = is_batch(first);
    free(first);
    if (batch)
        return launch_batch(argv, status, do_wait);
    wchar_t *line = calloc(COMMAND_CAPACITY, sizeof(*line));
    if (!line) { errno = ENOMEM; return -1; }
    size_t used = 0;
    int result = -1;
    for (size_t i = 0; argv[i]; i++) {
        wchar_t *wide = win_wide(argv[i]);
        if (!wide) goto done;
        int error = argument(line, &used, wide);
        free(wide);
        if (error) goto done;
    }
    result = launch(line, NULL, status, do_wait);
done:
    free(line);
    return result;
}

int spawn_shell(const char *command)
{
    wchar_t *shell = win_wide(getenv("COMSPEC") ? getenv("COMSPEC") : "cmd.exe");
    wchar_t *wide = win_wide(command);
    wchar_t *line = calloc(COMMAND_CAPACITY, sizeof(*line));
    size_t used = 0;
    int result = -1;
    if (!shell || !wide || !line) { errno = ENOMEM; goto done; }
    if (argument(line, &used, shell)) goto done;
    const wchar_t *prefix = L" /d /s /c \"";
    while (*prefix) if (append(line, &used, *prefix++)) goto done;
    for (const wchar_t *p = wide; *p; p++) if (append(line, &used, *p)) goto done;
    if (append(line, &used, L'"')) goto done;
    result = launch(line, shell, NULL, 0);
done:
    free(shell); free(wide); free(line);
    return result;
}

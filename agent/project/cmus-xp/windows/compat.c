/* Native Windows services for the upstream cmus engine. GPL-2.0-or-later. */
#include "compat.h"
#include <sys/mman.h>
#include <sys/select.h>
#include <sys/ioctl.h>
#include <stdarg.h>
#include <errno.h>
#include <wchar.h>
#include <curses.h>

static void win_set_errno(DWORD err)
{
    switch (err) {
    case ERROR_FILE_NOT_FOUND: case ERROR_PATH_NOT_FOUND: errno = ENOENT; break;
    case ERROR_ACCESS_DENIED: case ERROR_SHARING_VIOLATION: errno = EACCES; break;
    case ERROR_ALREADY_EXISTS: case ERROR_FILE_EXISTS: errno = EEXIST; break;
    case ERROR_NOT_ENOUGH_MEMORY: errno = ENOMEM; break;
    case ERROR_FILENAME_EXCED_RANGE: errno = ENAMETOOLONG; break;
    case ERROR_INVALID_NAME: errno = EINVAL; break;
    case ERROR_DIRECTORY: errno = ENOTDIR; break;
    case ERROR_NOT_READY: case ERROR_DEVICE_NOT_CONNECTED: errno = ENODEV; break;
    default: errno = EIO; break;
    }
}

wchar_t *win_wide(const char *s)
{
    int n = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, s, -1, NULL, 0);
    if (!n) { errno = EILSEQ; return NULL; }
    wchar_t *out = malloc(n * sizeof(*out));
    if (!out) { errno = ENOMEM; return NULL; }
    MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, s, -1, out, n);
    return out;
}
char *win_utf8(const wchar_t *s)
{
    int n = WideCharToMultiByte(CP_UTF8, 0, s, -1, NULL, 0, NULL, NULL);
    if (!n) { errno = EILSEQ; return NULL; }
    char *out = malloc(n);
    if (!out) { errno = ENOMEM; return NULL; }
    WideCharToMultiByte(CP_UTF8, 0, s, -1, out, n, NULL, NULL);
    return out;
}
char *win_normalize(char *s)
{
    if (s) for (char *p = s; *p; p++) if (*p == 92) *p = '/';
    return s;
}
char **win_environment;

char *win_getenv(const char *name)
{
    if (!win_environment) return getenv(name);
    size_t size = strlen(name);
    for (char **p = win_environment; *p; p++)
        if (!_strnicmp(*p, name, size) && (*p)[size] == '=')
            return *p + size + 1;
    return NULL;
}

void win_init(void)
{
    /* Binary CRT descriptors are essential for audio, cache, and pipe data. */
    _fmode = _O_BINARY;
    wchar_t buf[32768];
    if (!GetEnvironmentVariableW(L"HOME", buf, 32768) &&
            GetEnvironmentVariableW(L"USERPROFILE", buf, 32768))
        SetEnvironmentVariableW(L"HOME", buf);
    /* CRT getenv/environ use the legacy ANSI code page. Keep an immutable
     * UTF-8 snapshot for the shared engine and playlist env expansion. */
    wchar_t *block = GetEnvironmentStringsW();
    if (!block) return;
    size_t count = 0;
    for (wchar_t *p = block; *p; p += wcslen(p) + 1) count++;
    win_environment = calloc(count + 1, sizeof(*win_environment));
    if (win_environment) {
        size_t i = 0;
        for (wchar_t *p = block; *p; p += wcslen(p) + 1) {
            char *entry = win_utf8(p);
            if (entry) win_environment[i++] = entry;
        }
    }
    FreeEnvironmentStringsW(block);
}
FILE *win_fopen(const char *p, const char *mode)
{
    wchar_t *w = win_path(p), *m = win_wide(mode);
    if (!w || !m) { free(w); free(m); return NULL; }
    FILE *f = _wfopen(w, m); free(w); free(m); return f;
}
int win_open(const char *p, int flags, ...)
{
    int mode = 0;
    if (flags & O_CREAT) { va_list args; va_start(args, flags); mode = va_arg(args, int); va_end(args); }
    wchar_t *w = !_stricmp(p, "NUL") ? win_wide(p) : win_path(p); if (!w) return -1;
    int fd = _wopen(w, flags | _O_BINARY, mode); free(w); return fd;
}
int win_stat(const char *p, struct stat *st)
{
    wchar_t *w = win_path(p); if (!w) return -1;
    int rc = wstat(w, st); free(w); return rc;
}
int win_access(const char *p, int mode)
{
    wchar_t *w = win_path(p); if (!w) return -1;
    int rc = _waccess(w, mode); free(w); return rc;
}
int win_resource_access(const char *p, int mode)
{
    const char *data; size_t size;
    if (win_resource(p, &data, &size)) return 0;
    return win_access(p, mode);
}
int win_unlink(const char *p)
{
    wchar_t *w = win_path(p); if (!w) return -1;
    int rc = _wunlink(w); free(w); return rc;
}
int win_rename(const char *from, const char *to)
{
    wchar_t *a = win_path(from), *b = win_path(to);
    if (!a || !b) { free(a); free(b); return -1; }
    /* POSIX rename replaces autosave/cache files; CRT rename does not. */
    BOOL ok = MoveFileExW(a, b, MOVEFILE_REPLACE_EXISTING | MOVEFILE_WRITE_THROUGH);
    DWORD err = GetLastError(); free(a); free(b);
    if (!ok) { win_set_errno(err); return -1; }
    return 0;
}
int win_chdir(const char *p)
{
    wchar_t *w = win_path(p); if (!w) return -1;
    int rc = _wchdir(w); free(w); return rc;
}
char *win_getcwd(char *buf, int size)
{
    wchar_t *w = _wgetcwd(NULL, 0); if (!w) return NULL;
    char *raw = win_utf8(w); free(w);
    if (!raw) return NULL;
    char *s = win_absolute_path(raw, NULL); free(raw);
    if (!s) return NULL;
    if (!buf) return s;
    if (strlen(s) >= (size_t)size) { free(s); errno = ERANGE; return NULL; }
    strcpy(buf, s); free(s); return buf;
}
int win_mkdir(const char *p)
{
    wchar_t *w = win_path(p); if (!w) return -1;
    int rc = _wmkdir(w); free(w); return rc;
}
/* Win32 enumeration avoids the CRT's MAX_PATH-limited directory buffer. */
struct win_directory {
    HANDLE find;
    WIN32_FIND_DATAW data;
    int first;
    struct dirent entry;
};
DIR *win_opendir(const char *p)
{
    wchar_t *w = win_path(p); if (!w) return NULL;
    DWORD attributes = GetFileAttributesW(w);
    if (attributes == INVALID_FILE_ATTRIBUTES || !(attributes & FILE_ATTRIBUTE_DIRECTORY)) {
        DWORD err = GetLastError(); free(w);
        if (attributes != INVALID_FILE_ATTRIBUTES) errno = ENOTDIR;
        else win_set_errno(err);
        return NULL;
    }
    size_t len = wcslen(w);
    wchar_t *pattern = realloc(w, (len + 3) * sizeof(*w));
    if (!pattern) { free(w); errno = ENOMEM; return NULL; }
    if (len && pattern[len-1] != L'\\') pattern[len++] = L'\\';
    pattern[len++] = L'*'; pattern[len] = 0;
    DIR *dir = calloc(1, sizeof(*dir));
    if (!dir) { free(pattern); errno = ENOMEM; return NULL; }
    dir->find = FindFirstFileW(pattern, &dir->data);
    DWORD find_error = GetLastError(); free(pattern);
    dir->first = dir->find != INVALID_HANDLE_VALUE;
    if (!dir->first && find_error != ERROR_FILE_NOT_FOUND) {
        win_set_errno(find_error); free(dir); return NULL;
    }
    return dir;
}
struct dirent *win_readdir(DIR *dir)
{
    if (dir->find == INVALID_HANDLE_VALUE) return NULL;
    if (!dir->first && !FindNextFileW(dir->find, &dir->data)) return NULL;
    dir->first = 0;
    char *name = win_utf8(dir->data.cFileName); if (!name) return NULL;
    size_t len = strlen(name);
    if (len >= sizeof(dir->entry.d_name)) { free(name); errno = ENAMETOOLONG; return NULL; }
    strcpy(dir->entry.d_name, name); dir->entry.d_namlen = len;
    /* Avoid recursively following junctions/symlinks back into the library. */
    dir->entry.d_is_link = dir->data.dwReserved0 == 0xa0000003UL ||
        dir->data.dwReserved0 == 0xa000000cUL;
    free(name); return &dir->entry;
}
int win_closedir(DIR *dir)
{
    int rc = dir->find == INVALID_HANDLE_VALUE || FindClose(dir->find) ? 0 : -1;
    free(dir); return rc;
}
void *mmap(void *address, size_t size, int prot, int flags, int fd, off_t offset)
{
    (void)address; (void)prot; (void)flags;
    HANDLE file = (HANDLE)_get_osfhandle(fd);
    HANDLE map = CreateFileMappingW(file, NULL, PAGE_READONLY, 0, 0, NULL);
    if (!map) { win_set_errno(GetLastError()); return MAP_FAILED; }
    void *view = MapViewOfFile(map, FILE_MAP_READ, (uint64_t)offset >> 32, (DWORD)offset, size);
    DWORD err = GetLastError(); CloseHandle(map);
    if (!view) { win_set_errno(err); return MAP_FAILED; }
    return view;
}
int munmap(void *view, size_t size)
{
    (void)size;
    return UnmapViewOfFile(view) ? 0 : -1;
}
static int descriptor_ready(int fd)
{
    HANDLE h = (HANDLE)_get_osfhandle(fd);
    if (h == INVALID_HANDLE_VALUE) return 0;
    if (fd == 0) return WaitForSingleObject(h, 0) == WAIT_OBJECT_0;
    if (GetFileType(h) == FILE_TYPE_PIPE) {
        DWORD available = 0;
        if (!PeekNamedPipe(h, NULL, 0, NULL, &available, NULL))
            return GetLastError() == ERROR_BROKEN_PIPE;
        return available != 0;
    }
    return GetFileType(h) == FILE_TYPE_DISK;
}
int select(int n, fd_set *readfds, fd_set *writefds, fd_set *exceptfds, struct timeval *tv)
{
    if (writefds || exceptfds || n > 2048) { errno = ENOTSUP; return -1; }
    fd_set requested = *readfds;
    ULONGLONG start = GetTickCount64();
    ULONGLONG timeout = tv ? tv->tv_sec * 1000ULL + tv->tv_usec / 1000 : UINT64_MAX;
    for (;;) {
        int ready = 0; FD_ZERO(readfds);
        for (int fd = 0; fd < n; fd++)
            if (FD_ISSET(fd, &requested) && descriptor_ready(fd)) { FD_SET(fd, readfds); ready++; }
        if (ready || GetTickCount64() - start >= timeout) return ready;
        Sleep(10);
    }
}
int ioctl(int fd, int request, struct winsize *ws)
{
    (void)fd; (void)request;
    CONSOLE_SCREEN_BUFFER_INFO info;
    if (!GetConsoleScreenBufferInfo(GetStdHandle(STD_OUTPUT_HANDLE), &info)) return -1;
    ws->ws_col = info.srWindow.Right - info.srWindow.Left + 1;
    ws->ws_row = info.srWindow.Bottom - info.srWindow.Top + 1;
    return 0;
}
struct passwd *getpwuid(unsigned int id)
{
    (void)id;
    static struct passwd user;
    user.pw_name = win_getenv("USERNAME"); user.pw_dir = win_getenv("HOME");
    return user.pw_dir ? &user : NULL;
}
struct passwd *getpwnam(const char *name)
{
    struct passwd *p = getpwuid(0);
    return p && p->pw_name && !_stricmp(name, p->pw_name) ? p : NULL;
}
char *tgetstr(const char *id, char **area) { (void)id; (void)area; return NULL; }
char *tgoto(const char *cap, int col, int row) { (void)col; (void)row; return (char *)cap; }
char *win_data_dir(void) { return ":cmus-data:"; }
char *win_config_dir(void)
{
    wchar_t path[32768];
    if (!GetEnvironmentVariableW(L"APPDATA", path, 32768)) return NULL;
    char *base = win_normalize(win_utf8(path));
    if (!base) return NULL;
    char *dir = malloc(strlen(base) + 10);
    if (dir) sprintf(dir, "%s/cmus-win", base);
    free(base); return dir;
}

/* Native Windows platform services. SPDX-License-Identifier: GPL-2.0-or-later */
#ifdef _WIN32
#include "compat.h"
#include <errno.h>
#include <stdarg.h>
#include <grp.h>
#include <sys/utsname.h>
#include <termios.h>
#include <tlhelp32.h>
extern int _nc_wcwidth(wchar_t);
int wcwidth(wchar_t character) { return _nc_wcwidth(character); }
void xp_platform_init(void)
{
  if (!getenv("HOME")) { const char *home = getenv("USERPROFILE"); if (home) _putenv_s("HOME", home); }
  const char *configured_home = getenv("HOME");
  if (configured_home)
  {
    char *normalized = _strdup(configured_home);
    if (normalized)
    {
      for (char *p = normalized; *p; p++) if (*p == '\\') *p = '/';
      _putenv_s("HOME", normalized);
      free(normalized);
    }
  }
  if (!getenv("TMPDIR")) { const char *temp = getenv("TEMP"); if (temp) _putenv_s("TMPDIR", temp); }
  if (!getenv("NEOMUTT_XP_SHELL"))
  {
    wchar_t shell[32768];
    DWORD n = SearchPathW(NULL, L"bash.exe", NULL, 32768, shell, NULL);
    if (n && n < 32768)
    {
      char utf8[65536];
      if (WideCharToMultiByte(CP_UTF8, 0, shell, -1, utf8, sizeof(utf8), NULL, NULL))
        _putenv_s("NEOMUTT_XP_SHELL", utf8);
    }
  }
  SetConsoleCP(CP_UTF8); SetConsoleOutputCP(CP_UTF8);
  SetErrorMode(SEM_FAILCRITICALERRORS | SEM_NOGPFAULTERRORBOX);
}
const char *xp_runtime_path(const char *relative)
{
  static char result[65536];
  wchar_t path[32768];
  DWORD size = GetModuleFileNameW(NULL, path, 32768);
  if (!size || size >= 32768) return NULL;
  wchar_t *last = wcsrchr(path, L'\\');
  if (!last) return NULL;
  int available = (int)(32768 - (last + 1 - path));
  if (!MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, relative, -1, last + 1, available)) return NULL;
  if (GetFileAttributesW(path) == INVALID_FILE_ATTRIBUTES) return NULL;
  return WideCharToMultiByte(CP_UTF8, 0, path, -1, result, sizeof(result), NULL, NULL) ? result : NULL;
}

const char *xp_crypto_directory(void)
{
  const char *override = getenv("NEOMUTT_XP_CRYPTO_DIR");
  return override && *override ? override : xp_runtime_path("neomutt/gnupg/bin");
}

static struct passwd User;
static char Home[32768], Username[512], Shell[32768];
static void utf8_env(const wchar_t *key, char *out, int size)
{
  wchar_t value[32768];
  DWORD n = GetEnvironmentVariableW(key, value, 32768);
  if (!n || n >= 32768 || !WideCharToMultiByte(CP_UTF8, 0, value, -1, out, size, NULL, NULL)) out[0] = 0;
}
struct passwd *getpwuid(unsigned uid)
{
  (void) uid;
  utf8_env(L"HOME", Home, sizeof(Home));
  if (!*Home) utf8_env(L"USERPROFILE", Home, sizeof(Home));
  utf8_env(L"USERNAME", Username, sizeof(Username));
  utf8_env(L"COMSPEC", Shell, sizeof(Shell));
  for (char *p = Shell; *p; p++) if (*p == '\\') *p = '/';
  for (char *p = Home; *p; p++) if (*p == '\\') *p = '/';
  User = (struct passwd) { .pw_name = Username, .pw_gecos = Username, .pw_dir = Home, .pw_shell = Shell };
  return &User;
}
struct passwd *getpwnam(const char *name)
{
  struct passwd *user = getpwuid(0);
  return !_stricmp(name, user->pw_name) ? user : NULL;
}
unsigned getuid(void) { return 0; }
unsigned getgid(void) { return 0; }
unsigned geteuid(void) { return 0; }
unsigned getegid(void) { return 0; }
struct group *getgrgid(unsigned gid)
{
  static struct group group = { "current-user", 0 }; (void) gid; return &group;
}
int chown(const char *path, uid_t uid, gid_t gid)
{ (void) path; (void) uid; (void) gid; errno = ENOTSUP; return -1; }
int fchown(int fd, uid_t uid, gid_t gid)
{ (void) fd; (void) uid; (void) gid; errno = ENOTSUP; return -1; }
locale_t xp_newlocale(int mask, const char *name, locale_t base)
{ (void) mask; (void) base; return _create_locale(LC_ALL, name); }
locale_t xp_duplocale(locale_t locale)
{ (void) locale; return _create_locale(LC_ALL, ""); }
struct tm *xp_localtime_r(const time_t *time, struct tm *out) { return localtime_s(out, time) ? NULL : out; }
struct tm *xp_gmtime_r(const time_t *time, struct tm *out) { return gmtime_s(out, time) ? NULL : out; }
int fcntl(int fd, int command, ...)
{
  va_list args; va_start(args, command);
  int rc = -1;
  if (command == F_SETLK)
  {
    struct flock *lock = va_arg(args, struct flock *);
    if (lock->l_whence != SEEK_SET || lock->l_start || lock->l_len) errno = ENOTSUP;
    else rc = xp_lock(fd, lock->l_type == F_WRLCK, lock->l_type == F_UNLCK);
  }
  else if (command == F_SETFD)
  {
    int flags = va_arg(args, int);
    rc = SetHandleInformation((HANDLE) _get_osfhandle(fd), HANDLE_FLAG_INHERIT,
                              flags & FD_CLOEXEC ? 0 : HANDLE_FLAG_INHERIT) ? 0 : -1;
  }
  else errno = ENOTSUP;
  va_end(args); return rc;
}
char *strsep(char **text, const char *delimiters)
{
  char *start = *text;
  if (!start) return NULL;
  char *end = start + strcspn(start, delimiters);
  if (*end) { *end = 0; *text = end + 1; } else *text = NULL;
  return start;
}
char *strcasestr(const char *text, const char *needle)
{
  size_t length = strlen(needle);
  for (; *text; text++) if (!_strnicmp(text, needle, length)) return (char *) text;
  return length ? NULL : (char *) text;
}
int getdomainname(char *buffer, size_t size)
{
  DWORD n = (DWORD) size;
  return GetComputerNameExA(ComputerNameDnsDomain, buffer, &n) ? 0 : -1;
}
int uname(struct utsname *name)
{
  memset(name, 0, sizeof(*name));
  strcpy(name->sysname, "Windows"); strcpy(name->release, "native");
  strcpy(name->version, "NeoMutt XP"); strcpy(name->machine, "x86_64");
  DWORD n = sizeof(name->nodename);
  return GetComputerNameA(name->nodename, &n) ? 0 : -1;
}
int getpagesize(void) { SYSTEM_INFO info; GetSystemInfo(&info); return (int) info.dwPageSize; }
char *ttyname(int fd) { return _isatty(fd) ? "CONIN$" : NULL; }
int setenv(const char *name, const char *value, int replace)
{ return !replace && getenv(name) ? 0 : _putenv_s(name, value); }
int unsetenv(const char *name) { return _putenv_s(name, ""); }
int xp_console_read_key(int fd)
{
  HANDLE input = (HANDLE) _get_osfhandle(fd);
  INPUT_RECORD event;
  DWORD count;
  while (ReadConsoleInputW(input, &event, 1, &count) && count)
  {
    if (event.EventType == KEY_EVENT && event.Event.KeyEvent.bKeyDown &&
        event.Event.KeyEvent.uChar.UnicodeChar)
      return event.Event.KeyEvent.uChar.UnicodeChar;
  }
  return EOF;
}
void xp_console_flush_input(int fd)
{ FlushConsoleInputBuffer((HANDLE) _get_osfhandle(fd)); }
int tcgetattr(int fd, struct termios *term)
{
  DWORD mode;
  if (!GetConsoleMode((HANDLE) _get_osfhandle(fd), &mode)) { errno = ENOTTY; return -1; }
  memset(term, 0, sizeof(*term)); term->console_mode = mode;
  if (mode & ENABLE_LINE_INPUT) term->c_lflag |= ICANON;
  if (mode & ENABLE_ECHO_INPUT) term->c_lflag |= ECHO;
  return 0;
}
int tcsetattr(int fd, int action, const struct termios *term)
{
  (void) action;
  DWORD mode = term->console_mode & ~(ENABLE_LINE_INPUT | ENABLE_ECHO_INPUT);
  if (term->c_lflag & ICANON) mode |= ENABLE_LINE_INPUT;
  if (term->c_lflag & ECHO) mode |= ENABLE_ECHO_INPUT;
  if (!SetConsoleMode((HANDLE) _get_osfhandle(fd), mode)) { errno = ENOTTY; return -1; } return 0;
}
int ioctl(int fd, unsigned long request, void *value)
{
  if (request != TIOCGWINSZ) { errno = EINVAL; return -1; }
  (void) fd;
  CONSOLE_SCREEN_BUFFER_INFO info;
  HANDLE handle = CreateFileW(L"CONOUT$", GENERIC_READ,
      FILE_SHARE_READ | FILE_SHARE_WRITE, NULL, OPEN_EXISTING, 0, NULL);
  if (handle == INVALID_HANDLE_VALUE) { errno = ENOTTY; return -1; }
  BOOL ok = GetConsoleScreenBufferInfo(handle, &info);
  CloseHandle(handle);
  if (!ok) { errno = ENOTTY; return -1; }
  struct winsize *size = value;
  size->ws_row = info.srWindow.Bottom - info.srWindow.Top + 1;
  size->ws_col = info.srWindow.Right - info.srWindow.Left + 1;
  size->ws_xpixel = size->ws_ypixel = 0; return 0;
}
static struct sigaction Actions[32];
static sigset_t Blocked;
int sigemptyset(sigset_t *set) { *set = 0; return 0; }
int sigaddset(sigset_t *set, int signal) { if (signal < 0 || signal >= 32) return -1; *set |= 1UL << signal; return 0; }
int sigprocmask(int how, const sigset_t *set, sigset_t *old)
{
  if (old) *old = Blocked;
  if (set) { if (how == SIG_BLOCK) Blocked |= *set; else if (how == SIG_UNBLOCK) Blocked &= ~*set; else Blocked = *set; }
  return 0;
}
int sigaction(int number, const struct sigaction *action, struct sigaction *old)
{
  if (number < 0 || number >= 32) { errno = EINVAL; return -1; }
  if (old) *old = Actions[number];
  if (action)
  {
    Actions[number] = *action;
    if (number == SIGINT || number == SIGTERM || number == SIGSEGV || number == SIGABRT || number == SIGFPE || number == SIGILL)
      signal(number, action->sa_handler);
  }
  return 0;
}
int kill(pid_t pid, int number)
{
  if (!pid) { errno = ENOTSUP; return -1; }
  HANDLE process = OpenProcess(number ? PROCESS_TERMINATE : PROCESS_QUERY_LIMITED_INFORMATION, FALSE, (DWORD) pid);
  if (!process) { errno = ESRCH; return -1; }
  BOOL rc = number ? TerminateProcess(process, 128 + number) : TRUE;
  CloseHandle(process); if (!rc) { errno = EACCES; return -1; } return 0;
}
#endif

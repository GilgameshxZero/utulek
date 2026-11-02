/* Native Windows compatibility boundary. GPL-2.0-or-later. */
#ifndef CMUS_WIN_COMPAT_H
#define CMUS_WIN_COMPAT_H
#define WIN32_LEAN_AND_MEAN
#define NOMINMAX
#include <windows.h>
#undef MOUSE_MOVED
#define PDC_NCMOUSE
#define NCURSES_MOUSE_VERSION 2
#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <io.h>
#include <direct.h>
#include <unistd.h>
#include <signal.h>
#include <time.h>
#include <dirent.h>
#include <pwd.h>
#ifndef S_ISLNK
#define S_ISLNK(m) 0
#endif
#define lstat stat
#define mkdir(path, mode) _mkdir(path)
#define fsync _commit
#define SIGHUP 1
#define SIGPIPE 13
#define SIGWINCH 28
struct sigaction { void (*sa_handler)(int); int sa_mask; int sa_flags; };
static inline int sigemptyset(int *s) { *s=0; return 0; }
static inline int sigaction(int s, const struct sigaction *a, void *old) {
    (void)old;
    if (s == SIGINT || s == SIGTERM) signal(s, a->sa_handler);
    return 0;
}
wchar_t *win_wide(const char *);
char *win_utf8(const wchar_t *);
wchar_t *win_path(const char *);
char *win_absolute_path(const char *, const char *);
int win_check_runtime(void);
char *win_normalize(char *);
void win_init(void);
void win_curses_pair_defaults(int pair, int foreground, int background);
FILE *win_fopen(const char *, const char *);
int win_open(const char *, int, ...);
int win_stat(const char *, struct stat *);
int win_access(const char *, int);
int win_unlink(const char *);
int win_rename(const char *, const char *);
int win_chdir(const char *);
char *win_getcwd(char *, int);
int win_mkdir(const char *);
DIR *win_opendir(const char *);
struct dirent *win_readdir(DIR *);
int win_closedir(DIR *);
int win_resource(const char *, const char **, size_t *);
int win_resource_access(const char *, int);
char *win_data_dir(void);
char *win_config_dir(void);
char *tgetstr(const char *, char **);
char *tgoto(const char *, int, int);
#ifndef CMUS_WIN_IMPLEMENTATION
#define fopen win_fopen
#define open win_open
#define stat(path, buf) win_stat(path, buf)
#define access win_resource_access
#define unlink win_unlink
#define rename win_rename
#define chdir win_chdir
#define getcwd win_getcwd
#undef mkdir
#define mkdir(path, mode) win_mkdir(path)
#define opendir win_opendir
#define readdir win_readdir
#define closedir win_closedir
#endif
#endif

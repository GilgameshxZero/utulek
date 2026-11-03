/* Native Windows platform declarations. SPDX-License-Identifier: GPL-2.0-or-later */
#ifndef NEOMUTT_XP_COMPAT_H
#define NEOMUTT_XP_COMPAT_H
#ifdef _WIN32
#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#include <winsock2.h>
#include <ws2tcpip.h>
#include <windows.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <unistd.h>
#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <string.h>
#include <locale.h>
#include <io.h>
#include <process.h>
#include <fcntl.h>
#include <direct.h>
#include <time.h>
#include <wchar.h>
#include <pwd.h>
#include <sys/ioctl.h>
#include <signal.h>
typedef unsigned int uid_t;
typedef unsigned int gid_t;
typedef unsigned int nlink_t;
typedef _locale_t locale_t;
#ifndef S_IRUSR
#define S_IRUSR _S_IREAD
#define S_IWUSR _S_IWRITE
#define S_IXUSR _S_IEXEC
#define S_IRWXU (S_IRUSR | S_IWUSR | S_IXUSR)
#define S_IRGRP 0
#define S_IWGRP 0
#define S_IXGRP 0
#define S_IRWXG 0
#define S_IROTH 0
#define S_IWOTH 0
#define S_IXOTH 0
#define S_IRWXO 0
#endif
#ifndef S_ISLNK
#define S_ISLNK(mode) (((mode) & _S_IFMT) == 0120000)
#endif
#ifndef O_NOFOLLOW
#define O_NOFOLLOW 0x10000000
#endif
#ifndef O_CLOEXEC
#define O_CLOEXEC _O_NOINHERIT
#endif
#define LC_ALL_MASK 0
#define LC_TIME_MASK 0
#define LC_GLOBAL_LOCALE ((locale_t) -1)
locale_t xp_newlocale(int mask, const char *name, locale_t base);
#define newlocale xp_newlocale
#define freelocale _free_locale
#define strftime_l _strftime_l
locale_t xp_duplocale(locale_t);
#define duplocale xp_duplocale
struct tm *xp_localtime_r(const time_t *, struct tm *);
struct tm *xp_gmtime_r(const time_t *, struct tm *);
#define localtime_r xp_localtime_r
#define gmtime_r xp_gmtime_r
#define SA_NOCLDSTOP 1
struct flock { int l_type, l_whence; int64_t l_start, l_len; };
#define F_SETLK 1
#define F_SETFD 2
#define FD_CLOEXEC 1
#define F_WRLCK 1
#define F_RDLCK 2
#define F_UNLCK 3
int fcntl(int, int, ...);
int chown(const char *, uid_t, gid_t);
int fchown(int, uid_t, gid_t);
char *strsep(char **, const char *);
char *strcasestr(const char *, const char *);
int getdomainname(char *, size_t);
int getpagesize(void);
int wcwidth(wchar_t);
void xp_platform_init(void);
int wcscasecmp(const wchar_t *, const wchar_t *);
int kill(pid_t, int);
#define S_ISUID 04000
#define S_ISGID 02000
#define S_ISVTX 01000
#define timegm _mkgmtime
#define execvp _execvp
char *ttyname(int);
int setenv(const char *, const char *, int);
int unsetenv(const char *);
typedef unsigned long sigset_t;
struct sigaction { void (*sa_handler)(int); sigset_t sa_mask; int sa_flags; };
#define SIG_BLOCK 0
#define SIG_UNBLOCK 1
#define SIG_SETMASK 2
const char *xp_runtime_path(const char *relative);
const char *xp_crypto_directory(void);
int xp_console_read_key(int fd);
void xp_console_flush_input(int fd);
int sigemptyset(sigset_t *);
int sigaddset(sigset_t *, int);
int sigaction(int, const struct sigaction *, struct sigaction *);
int sigprocmask(int, const sigset_t *, sigset_t *);
#ifndef SSIZE_MAX
#define SSIZE_MAX INTPTR_MAX
#endif
#ifndef SIGWINCH
#define SIGWINCH 28
#define SIGTSTP 20
#define SIGCONT 18
#define SIGQUIT 3
#define SIGHUP 1
#define SIGPIPE 13
#define SIGCHLD 17
#define SIGALRM 14
#define SIGSTOP 19
#include "files.h"
#endif
#endif
#endif

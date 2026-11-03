/* UTF-8 paths and 64-bit file identity on Windows. GPL-2.0-or-later */
#ifndef NEOMUTT_XP_FILES_H
#define NEOMUTT_XP_FILES_H
#ifdef _WIN32
#include <stdio.h>
#include <stdint.h>
#include <time.h>
#include <sys/types.h>
#include <sys/stat.h>
#ifdef stat
#undef stat
#endif
#ifdef fstat
#undef fstat
#endif
struct xp_stat {
  uint64_t st_dev, st_ino, st_rdev, st_nlink;
  unsigned st_mode, st_uid, st_gid;
  int64_t st_size;
  time_t st_atime, st_mtime, st_ctime;
};
int xp_stat(const char *, struct xp_stat *);
int xp_lstat(const char *, struct xp_stat *);
int xp_fstat(int, struct xp_stat *);
int xp_mkstemp(char *);
int xp_open(const char *, int, ...);
FILE *xp_fopen(const char *, const char *);
int xp_unlink(const char *);
int xp_rename(const char *, const char *);
int xp_rmdir(const char *);
int xp_access(const char *, int);
int xp_chmod(const char *, int);
int xp_mkdir(const char *, int);
char *xp_realpath(const char *, char *);
char *xp_getcwd(char *, int);
int xp_chdir(const char *);
int xp_utime(const char *, const void *);
int xp_fsync(int);
int xp_fchmod(int, mode_t);
int xp_link(const char *, const char *);
int xp_symlink(const char *, const char *);
int xp_lock(int, int, int);
#ifndef XP_NO_FILE_MACROS
#define stat xp_stat
#define fstat xp_fstat
#define lstat xp_lstat
#define mkstemp xp_mkstemp
#define open xp_open
#define fopen xp_fopen
#define unlink xp_unlink
#define rename xp_rename
#define rmdir xp_rmdir
#define access xp_access
#define chmod xp_chmod
#define mkdir(path,mode) xp_mkdir(path,mode)
#define realpath xp_realpath
#define getcwd xp_getcwd
#define chdir xp_chdir
#define fsync xp_fsync
#define fchmod xp_fchmod
#define link xp_link
#define symlink xp_symlink
#endif
#endif
#endif

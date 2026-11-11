#ifndef NEOMUTT_XP_PWD_H
#define NEOMUTT_XP_PWD_H
#include <sys/types.h>
struct passwd { char *pw_name; char *pw_passwd; unsigned pw_uid, pw_gid; char *pw_gecos; char *pw_dir; char *pw_shell; };
struct passwd *getpwuid(unsigned uid);
struct passwd *getpwnam(const char *name);
unsigned getuid(void);
unsigned getgid(void);
unsigned geteuid(void);
unsigned getegid(void);
#endif

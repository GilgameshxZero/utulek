#ifndef NEOMUTT_XP_UTSNAME_H
#define NEOMUTT_XP_UTSNAME_H
struct utsname { char sysname[128], nodename[256], release[128], version[128], machine[128]; };
int uname(struct utsname *name);
#endif

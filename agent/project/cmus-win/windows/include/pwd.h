#ifndef CMUS_WIN_PWD_H
#define CMUS_WIN_PWD_H
struct passwd { char *pw_name; char *pw_dir; };
struct passwd *getpwuid(unsigned int);
struct passwd *getpwnam(const char *);
static inline unsigned int getuid(void) { return 0; }
#endif

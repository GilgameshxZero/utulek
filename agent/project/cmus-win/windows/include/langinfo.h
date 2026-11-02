#ifndef CMUS_WIN_LANGINFO_H
#define CMUS_WIN_LANGINFO_H
#define CODESET 1
static inline char *nl_langinfo(int item) { (void)item; return "UTF-8"; }
#endif

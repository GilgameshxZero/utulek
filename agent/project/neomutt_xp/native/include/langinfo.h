#ifndef NEOMUTT_XP_LANGINFO_H
#define NEOMUTT_XP_LANGINFO_H
#define CODESET 1
#define YESEXPR 2
#define NOEXPR 3
#define D_T_FMT 4
static inline char *nl_langinfo(int item) { return item == D_T_FMT ? "%a %b %d %H:%M:%S %Y" : item == YESEXPR ? "^[yY]" : item == NOEXPR ? "^[nN]" : "UTF-8"; }
#endif

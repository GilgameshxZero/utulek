/* UTF-8 Windows directory enumeration. GPL-2.0-or-later */
#ifndef NEOMUTT_XP_DIRENT_H
#define NEOMUTT_XP_DIRENT_H
#include <stdint.h>
struct dirent { uint64_t d_ino; unsigned char d_type; unsigned short d_namlen; char d_name[1024]; };
typedef struct xp_directory DIR;
#define DT_UNKNOWN 0
#define DT_DIR 4
#define DT_REG 8
#define DT_LNK 10
DIR *opendir(const char *);
struct dirent *readdir(DIR *);
int closedir(DIR *);
void rewinddir(DIR *);
#endif

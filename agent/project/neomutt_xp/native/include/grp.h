#ifndef NEOMUTT_XP_GRP_H
#define NEOMUTT_XP_GRP_H
struct group { char *gr_name; unsigned gr_gid; };
struct group *getgrgid(unsigned);
#endif

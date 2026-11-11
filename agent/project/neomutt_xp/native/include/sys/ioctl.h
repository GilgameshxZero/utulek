#ifndef NEOMUTT_XP_IOCTL_H
#define NEOMUTT_XP_IOCTL_H
struct winsize { unsigned short ws_row, ws_col, ws_xpixel, ws_ypixel; };
#define TIOCGWINSZ 1
int ioctl(int, unsigned long, void *);
#endif

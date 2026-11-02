#ifndef CMUS_WIN_IOCTL_H
#define CMUS_WIN_IOCTL_H
struct winsize { unsigned short ws_row, ws_col; };
#define TIOCGWINSZ 1
int ioctl(int, int, struct winsize *);
#endif

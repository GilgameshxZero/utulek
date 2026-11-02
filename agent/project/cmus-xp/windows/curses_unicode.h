/* Unicode cells for the statically built PDCurses boundary. GPL-2.0-or-later. */
#ifndef CMUS_CURSES_UNICODE_H
#define CMUS_CURSES_UNICODE_H
#include <curses.h>
#include <windows.h>
#define CMUS_WIDE_CONTINUATION 0x1fffff
int win_curses_waddnstr(WINDOW *win, const char *str, int n);
const WCHAR *win_curses_cluster(unsigned int code, int *length);
#endif

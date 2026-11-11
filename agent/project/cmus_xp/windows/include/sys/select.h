#ifndef CMUS_WIN_SELECT_H
#define CMUS_WIN_SELECT_H
#include <sys/time.h>
#include <string.h>
/* CRT descriptors, deliberately separate from Winsock socket handles. */
typedef struct { unsigned char bits[2048]; } fd_set;
#define FD_ZERO(s) memset((s), 0, sizeof(*(s)))
#define FD_SET(fd,s) do { if ((fd) >= 0 && (fd) < 2048) (s)->bits[(fd)] = 1; } while (0)
#define FD_ISSET(fd,s) ((fd) >= 0 && (fd) < 2048 && (s)->bits[(fd)])
/* CRT descriptors are unrelated to Winsock SOCKETs. Keep this emulation's
 * symbol private so network libraries retain the native Winsock select. */
#define select cmus_crt_select
int cmus_crt_select(int, fd_set *, fd_set *, fd_set *, struct timeval *);
#endif

/* Native remote transports; command semantics remain in server.c/main.c.
 * GPL-2.0-or-later. Handles are transport IDs, never CRT descriptors. */
#ifndef CMUS_WINDOWS_REMOTE_IO_H
#define CMUS_WINDOWS_REMOTE_IO_H
#include <sys/types.h>
#include <stddef.h>
int win_remote_local(const char *address);
int win_remote_listen(const char *address);
int win_remote_connect(const char *address);
int win_remote_accept(int listener);
int win_remote_ready(int connection, int writing);
ssize_t win_remote_read(int connection, void *buffer, size_t size);
ssize_t win_remote_write(int connection, const void *buffer, size_t size);
void win_remote_close(int connection);
#endif

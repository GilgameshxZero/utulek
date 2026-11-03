/* Native sockets. SPDX-License-Identifier: GPL-2.0-or-later */
#ifndef NEOMUTT_XP_SOCKET_H
#define NEOMUTT_XP_SOCKET_H
#include <stdint.h>
#include <stddef.h>
#include <signal.h>
intptr_t xp_socket_open(const char *, unsigned short, int, int, volatile sig_atomic_t *);
int xp_socket_read(intptr_t, char *, size_t, int, volatile sig_atomic_t *);
int xp_socket_write(intptr_t, const char *, size_t, int, volatile sig_atomic_t *);
int xp_socket_poll(intptr_t, int, int, volatile sig_atomic_t *);
int xp_socket_close(intptr_t);
#endif

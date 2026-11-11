/* SPDX-License-Identifier: GPL-2.0-or-later */
#ifndef NEOMUTT_XP_TLS_IO_H
#define NEOMUTT_XP_TLS_IO_H
#include <stdint.h>
#include <signal.h>
#include <openssl/ssl.h>
int xp_tls_set_socket(SSL *, intptr_t, int, volatile sig_atomic_t *);
int xp_tls_system_certs(SSL_CTX *);
#endif

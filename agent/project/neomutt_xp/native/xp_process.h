/* Native child process support. SPDX-License-Identifier: GPL-2.0-or-later */
#ifndef NEOMUTT_XP_PROCESS_H
#define NEOMUTT_XP_PROCESS_H
#include <stdio.h>
#include <sys/types.h>
char *xp_quote_cmd_argument(const char *, int);
pid_t xp_filter_create(const char *command, FILE **input, FILE **output,
                       FILE **error, int fdin, int fdout, int fderr, char **environment);
int xp_waitpid(pid_t pid, int *status, int flags);
pid_t xp_spawn_arguments(const char *, const char *const *, int, int, int, char **);
int xp_waitpid_timeout(pid_t, int *, int);
int xp_detach_process(pid_t);
#endif

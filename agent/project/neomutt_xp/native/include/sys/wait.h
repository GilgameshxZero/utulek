#ifndef NEOMUTT_XP_WAIT_H
#define NEOMUTT_XP_WAIT_H
#include "xp_process.h"
#define WNOHANG 1
#define WIFEXITED(status) (((status) & 127) == 0)
#define WEXITSTATUS(status) (((status) >> 8) & 255)
#define WIFSIGNALED(status) (((status) & 127) != 0)
#define WTERMSIG(status) ((status) & 127)
#define waitpid xp_waitpid
#endif

#ifndef CMUS_WIN_WAIT_H
#define CMUS_WIN_WAIT_H
#define WIFEXITED(s) 1
#define WEXITSTATUS(s) (((s) >> 8) & 255)
#define WIFSIGNALED(s) 0
#define WTERMSIG(s) 0
#endif

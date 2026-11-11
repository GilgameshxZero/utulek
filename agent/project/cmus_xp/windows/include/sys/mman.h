#ifndef CMUS_WIN_MMAN_H
#define CMUS_WIN_MMAN_H
#include <sys/types.h>
#include <stddef.h>
#define PROT_READ 1
#define MAP_PRIVATE 2
#define MAP_FAILED ((void *)-1)
void *mmap(void *, size_t, int, int, int, off_t);
int munmap(void *, size_t);
#endif

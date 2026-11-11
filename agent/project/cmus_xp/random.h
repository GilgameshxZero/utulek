/* Portable non-cryptographic randomness for playback order. */
#ifndef CMUS_RANDOM_H
#define CMUS_RANDOM_H
#include <stdint.h>
#include <stdlib.h>

static inline uint64_t cmus_random_u64(void)
{
    uint64_t value = 0;
    /* ISO C only promises 15 random bits; Windows uses that minimum. */
    for (int i = 0; i < 5; i++)
        value = (value << 15) | ((unsigned int)rand() & 0x7fff);
    return value;
}

static inline uint64_t cmus_random_bounded(uint64_t bound)
{
    if (bound <= 1) return 0;
    uint64_t threshold = -bound % bound, value;
    do { value = cmus_random_u64(); } while (value < threshold);
    return value % bound;
}
#endif

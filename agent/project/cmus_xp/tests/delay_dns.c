/* Test-only process-local resolver delay. No DNS/system configuration changes. */
#define _GNU_SOURCE
#include <netdb.h>
#include <dlfcn.h>
#include <fcntl.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>

int getaddrinfo(const char *node, const char *service, const struct addrinfo *hints,
        struct addrinfo **result)
{
    if (node && !strncmp(node, "cmus-delay-", 11)) {
        const char *marker = getenv("CMUS_DNS_MARKER");
        int fd = marker ? open(marker, O_WRONLY | O_CREAT | O_APPEND, 0600) : -1;
        if (fd >= 0) { (void)write(fd, "resolver entered\n", 17); close(fd); }
        struct timespec remaining = {.tv_sec = 8};
        while (nanosleep(&remaining, &remaining)) {}
        *result = NULL;
        return EAI_NONAME;
    }
    typedef int (*function)(const char *, const char *, const struct addrinfo *, struct addrinfo **);
    function original = (function)dlsym(RTLD_NEXT, "getaddrinfo");
    return original(node, service, hints, result);
}

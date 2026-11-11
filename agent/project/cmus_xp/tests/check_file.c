/* Exercise real write loops and atomic replacement with injected I/O faults. */
#include <unistd.h>
#include <errno.h>
#include <assert.h>
#include <stdio.h>
#include <string.h>
#include <sys/stat.h>
#include <dirent.h>
static int fault, calls;
static ssize_t fault_write(int fd, const void *buf, size_t count)
{
    calls++;
    if (fault == 1) return 0;
    if (fault == 2 && calls > 1) { errno = ENOSPC; return -1; }
    if (fault == 3 && calls == 1) { errno = EINTR; return -1; }
    if (count > 3) count = 3;
    return write(fd, buf, count);
}
#define write fault_write
#ifndef CMUS_FILE_SOURCE
#define CMUS_FILE_SOURCE "../file.c"
#endif
#include CMUS_FILE_SOURCE
#undef write

static int save(int fd, void *data) { return write_all(fd, data, strlen(data)) < 0 ? -1 : 0; }
int main(int argc, char **argv)
{
    assert(argc == 2);
    char path[4096], link[4096], data[64] = {0};
    snprintf(path, sizeof(path), "%s/state", argv[1]);
    int fd = open(path, O_WRONLY|O_CREAT|O_TRUNC, 0640);
    assert(fd >= 0);
    fault = 1; calls = 0;
    assert(write_all(fd, "abc", 3) < 0 && errno == EIO);
    close(fd);
#ifndef CHECK_WRITE_ZERO_ONLY
    fault = 3; calls = 0;
    assert(file_write_atomic(path, save, "original contents") == 0);
    struct stat before, after;
    assert(!stat(path, &before));
    fault = 2; calls = 0;
    assert(file_write_atomic(path, save, "replacement must not survive") < 0 && errno == ENOSPC);
    fd = open(path, O_RDONLY); assert(fd >= 0);
    assert(read_all(fd, data, sizeof(data)) == 17 && !strcmp(data,"original contents")); close(fd);
    fault = 0; calls = 0;
    snprintf(link, sizeof(link), "%s/link", argv[1]);
    assert(!symlink(path, link));
    assert(!file_write_atomic(link, save, "updated"));
    assert(!lstat(link, &after) && S_ISLNK(after.st_mode));
    assert(!stat(path, &after) && (after.st_mode & 0777) == (before.st_mode & 0777));
    DIR *dir = opendir(argv[1]); struct dirent *entry; assert(dir);
    while ((entry = readdir(dir))) assert(!strstr(entry->d_name, ".tmp."));
    closedir(dir);
    unlink(link); unlink(path);
    puts("short/EINTR/zero writes, ENOSPC preservation, symlink target, permissions and temp cleanup: PASS");
#endif
    return 0;
}

/* Real-file flush/close/sync failure preservation, including Windows UTF-8 paths. */
#include <stdio.h>
#include <errno.h>
#include <assert.h>
#include <string.h>
#include <unistd.h>
static int fault;
static int fault_flush(FILE *stream)
{
    if (fault == 1) { errno = ENOSPC; return EOF; }
    return fflush(stream);
}
static int fault_close(FILE *stream)
{
    int rc = fclose(stream);
    if (fault == 2) { errno = ENOSPC; return EOF; }
    return rc;
}
static int fault_sync(int fd)
{
    if (fault == 3) { errno = EIO; return -1; }
    return fsync(fd);
}
#define fflush fault_flush
#define fclose fault_close
#undef fsync
#define fsync fault_sync
#include "../file.c"
#undef fflush
#undef fclose
#undef fsync

static int save(FILE *stream, void *data)
{
    return fputs(data, stream) == EOF ? -1 : 0;
}
static int reject(FILE *stream, void *data)
{
    fputs(data, stream);
    errno = EIO;
    return -1;
}
static const char *ready_path;
static int hold_write(FILE *stream, void *data)
{
    fputs(data, stream); assert(!fflush(stream));
    FILE *ready = fopen(ready_path, "wb"); assert(ready);
    assert(fclose(ready) == 0);
#ifdef CMUS_NATIVE_WINDOWS
    Sleep(60000);
#else
    sleep(60);
#endif
    return 0;
}
static void verify(const char *path, const char *expected)
{
    char buffer[128] = {0};
    FILE *file = fopen(path, "rb"); assert(file);
    size_t size = fread(buffer, 1, sizeof(buffer), file); assert(size == strlen(expected));
    assert(!memcmp(buffer, expected, size)); fclose(file);
}
static int run(const char *directory, int hold)
{
    char *path = malloc(strlen(directory) + 30);
    sprintf(path, "%s/atomic_音_state", directory);
    assert(!file_write_atomic_stdio(path, save, "original contents"));
    if (hold) {
        char *marker = malloc(strlen(path) + 7);
        sprintf(marker, "%s.ready", path); ready_path = marker;
        return file_write_atomic_stdio(path, hold_write, "interrupted replacement");
    }
    for (fault = 1; fault <= 3; fault++) {
        assert(file_write_atomic_stdio(path, save, "failed replacement") == -1);
        verify(path, "original contents");
    }
    fault = 0;
    assert(file_write_atomic_stdio(path, reject, "callback rejected") == -1);
    verify(path, "original contents");
    assert(!file_write_atomic_stdio(path, save, "successful replacement"));
    verify(path, "successful replacement");
    unlink(path); free(path);
    puts("stdio flush/close/sync/callback failure preservation and successful replacement: PASS");
    return 0;
}
#ifdef CMUS_NATIVE_WINDOWS
int wmain(int argc, wchar_t **argv)
{
    assert(argc == 2 || (argc == 3 && !wcscmp(argv[2], L"--hold")));
    char *directory = win_utf8(argv[1]); assert(directory);
    int rc = run(directory, argc == 3); free(directory); return rc;
}
#else
int main(int argc, char **argv)
{
    assert(argc == 2 || (argc == 3 && !strcmp(argv[2], "--hold")));
    return run(argv[1], argc == 3);
}
#endif

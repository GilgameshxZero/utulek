/* Read-only Windows platform checks: no config, reports, or fixtures written. */
#include "compat.h"
#include "../path.h"
#include <sys/mman.h>
#include <errno.h>
#include <wchar.h>

static int checks, failures;
#define CHECK(condition) do { checks++; if (!(condition)) { failures++; fprintf(stderr, "FAIL line %d: %s\n", __LINE__, #condition); } } while (0)
int win_check_runtime(void)
{
    char *path = win_absolute_path("../音/track.opus", "C:/Music/Album");
    CHECK(path && !strcmp(path, "C:/Music/音/track.opus")); free(path);
    path = win_absolute_path("../../../../", "C:/Music/Album");
    CHECK(path && !strcmp(path, "C:/")); free(path);
    path = win_absolute_path("../song.opus", "//server/share/album");
    CHECK(path && !strcmp(path, "//server/share/song.opus")); free(path);
    path = win_absolute_path("\\Music\\track.opus", "D:/Album");
    CHECK(path && !strcmp(path, "D:/Music/track.opus")); free(path);
    char long_name[5000] = "C:/";
    for (int i = 0; i < 12; i++) strcat(long_name, "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz/");
    strcat(long_name, "音.opus");
    wchar_t *extended = win_path(long_name);
    CHECK(extended && wcslen(extended) > 260 && !wcsncmp(extended, L"\\\\?\\C:\\", 7));
    free(extended);
    wchar_t unicode[256]; for (int i = 0; i < 255; i++) unicode[i] = 0x4e00; unicode[255] = 0;
    path = win_utf8(unicode); CHECK(path && strlen(path) == 765); free(path);
    CHECK(!get_extension(""));
    wchar_t windows[32768];
    CHECK(GetWindowsDirectoryW(windows, 32768) > 0);
    char *base = win_normalize(win_utf8(windows));
    CHECK(base != NULL);
    if (base) {
        path = malloc(strlen(base) + 40);
        sprintf(path, "%s/System32/kernel32.dll", base);
        struct stat st; CHECK(win_stat(path, &st) == 0 && st.st_size > 0);
        int fd = win_open(path, O_RDONLY); CHECK(fd >= 0);
        if (fd >= 0) {
            void *view = mmap(NULL, st.st_size, PROT_READ, MAP_PRIVATE, fd, 0);
            CHECK(view != MAP_FAILED && !memcmp(view, "MZ", 2));
            if (view != MAP_FAILED) munmap(view, st.st_size);
            close(fd);
        }
        sprintf(path, "%s/System32", base);
        DIR *dir = win_opendir(path); CHECK(dir != NULL);
        if (dir) {
            int count = 0; struct dirent *entry;
            while ((entry = win_readdir(dir))) {
                CHECK(strlen(entry->d_name) == entry->d_namlen); count++;
            }
            CHECK(count > 10); win_closedir(dir);
        }
        free(path); free(base);
    }
    printf("Windows read-only runtime checks: %d passed, %d failed\n", checks-failures, failures);
    return failures != 0;
}

/* Windows path canonicalization and extended-length filesystem paths.
 * GPL-2.0-or-later. No filesystem changes are made by these helpers.
 */
#include "compat.h"
#include <errno.h>
#include <wchar.h>
#include <ctype.h>

char *win_absolute_path(const char *src, const char *cwd)
{
    char *joined = NULL;
    int drive = src[0] && src[1] == ':';
    int unc = (src[0] == '/' || src[0] == '\\') &&
              (src[1] == '/' || src[1] == '\\');
    int rooted = src[0] == '/' || src[0] == '\\';
    if (cwd && !drive && !unc) {
        size_t base = strlen(cwd);
        if (rooted) {
            if (cwd[0] && cwd[1] == ':') base = 2;
            else if (cwd[0] == '/' && cwd[1] == '/') {
                const char *server = strchr(cwd + 2, '/');
                const char *share = server ? strchr(server + 1, '/') : NULL;
                if (share) base = share - cwd;
            }
        }
        joined = malloc(base + strlen(src) + 2);
        if (!joined) { errno = ENOMEM; return NULL; }
        memcpy(joined, cwd, base);
        joined[base] = 0;
        if (!rooted && base && joined[base-1] != '/') strcat(joined, "/");
        strcat(joined, src);
        src = joined;
    }
    wchar_t *wide = win_wide(src); free(joined);
    if (!wide) return NULL;
    for (wchar_t *p = wide; *p; p++) if (*p == L'/') *p = L'\\';
    DWORD needed = GetFullPathNameW(wide, 0, NULL, NULL);
    if (!needed || needed > 32768) { free(wide); errno = ENAMETOOLONG; return NULL; }
    wchar_t *absolute = malloc(needed * sizeof(*absolute));
    if (!absolute) { free(wide); errno = ENOMEM; return NULL; }
    DWORD length = GetFullPathNameW(wide, needed, absolute, NULL);
    free(wide);
    if (!length || length >= needed) { free(absolute); errno = ENAMETOOLONG; return NULL; }
    wchar_t *visible = absolute;
    if (!wcsncmp(absolute, L"\\\\?\\UNC\\", 8)) {
        visible = absolute + 6; visible[0] = L'\\';
    } else if (!wcsncmp(absolute, L"\\\\?\\", 4)) visible = absolute + 4;
    char *result = win_normalize(win_utf8(visible));
    free(absolute);
    return result;
}

wchar_t *win_path(const char *src)
{
    char *absolute = win_absolute_path(src, NULL);
    if (!absolute) return NULL;
    wchar_t *plain = win_wide(absolute); free(absolute);
    if (!plain) return NULL;
    for (wchar_t *p = plain; *p; p++) if (*p == L'/') *p = L'\\';
    size_t len = wcslen(plain);
    int unc = plain[0] == L'\\' && plain[1] == L'\\';
    wchar_t *result = malloc((len + 9) * sizeof(*result));
    if (!result) { free(plain); errno = ENOMEM; return NULL; }
    if (unc) { wcscpy(result, L"\\\\?\\UNC\\"); wcscat(result, plain + 2); }
    else { wcscpy(result, L"\\\\?\\"); wcscat(result, plain); }
    free(plain);
    return result;
}

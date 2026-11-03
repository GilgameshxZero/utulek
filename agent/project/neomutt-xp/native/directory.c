/* UTF-8 Windows directory enumeration. SPDX-License-Identifier: GPL-2.0-or-later */
#ifdef _WIN32
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <errno.h>
#include <stdlib.h>
#include <string.h>
#include <wchar.h>
#include <dirent.h>
struct xp_directory { HANDLE handle; wchar_t *pattern; WIN32_FIND_DATAW data; int first; struct dirent entry; };
DIR *opendir(const char *path)
{
  int n = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, path, -1, NULL, 0);
  if (!n) { errno = EILSEQ; return NULL; }
  DIR *dir = calloc(1, sizeof(*dir));
  if (!dir) return NULL;
  dir->pattern = calloc((size_t) n + 3, sizeof(wchar_t));
  if (!dir->pattern) { free(dir); return NULL; }
  MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, path, -1, dir->pattern, n);
  wcscat(dir->pattern, L"/*");
  dir->handle = FindFirstFileW(dir->pattern, &dir->data);
  if (dir->handle == INVALID_HANDLE_VALUE)
  { free(dir->pattern); free(dir); errno = ENOENT; return NULL; }
  dir->first = 1;
  return dir;
}
struct dirent *readdir(DIR *dir)
{
  if (dir->first) dir->first = 0;
  else if (!FindNextFileW(dir->handle, &dir->data))
  { errno = GetLastError() == ERROR_NO_MORE_FILES ? 0 : EIO; return NULL; }
  int n = WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS, dir->data.cFileName,
                             -1, dir->entry.d_name, sizeof(dir->entry.d_name), NULL, NULL);
  if (!n) { errno = EILSEQ; return NULL; }
  dir->entry.d_namlen = (unsigned short) (n - 1);
  dir->entry.d_type = dir->data.dwFileAttributes & FILE_ATTRIBUTE_REPARSE_POINT ? DT_LNK :
                     dir->data.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY ? DT_DIR : DT_REG;
  return &dir->entry;
}
int closedir(DIR *dir)
{
  int rc = FindClose(dir->handle) ? 0 : -1;
  free(dir->pattern); free(dir); return rc;
}
void rewinddir(DIR *dir)
{
  FindClose(dir->handle);
  dir->handle = FindFirstFileW(dir->pattern, &dir->data);
  dir->first = dir->handle != INVALID_HANDLE_VALUE;
}
#endif

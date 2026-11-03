/* Windows file primitives with UTF-8 paths, stable identity and no-clobber links.
 * SPDX-License-Identifier: GPL-2.0-or-later */
#ifdef _WIN32
#define WIN32_LEAN_AND_MEAN
#define XP_NO_FILE_MACROS
#include <windows.h>
#include <sddl.h>
#include <bcrypt.h>
#include <errno.h>
#include <stdarg.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <wchar.h>
#include <io.h>
#include <fcntl.h>
#include <direct.h>
#include "files.h"
#ifndef O_NOFOLLOW
#define O_NOFOLLOW 0x10000000
#endif

/* Elevated tokens can default to Administrators as owner. Owner Rights then
 * excludes the same user's unelevated programs (including Syncthing). */
static INIT_ONCE SecurityOnce = INIT_ONCE_STATIC_INIT;
static PSECURITY_DESCRIPTOR FileSecurity, DirectorySecurity;
static BOOL CALLBACK initialize_security(PINIT_ONCE once, PVOID arg, PVOID *context)
{
  (void) once; (void) arg; (void) context;
  HANDLE token = NULL;
  DWORD length = 0;
  TOKEN_USER *user = NULL;
  LPWSTR sid = NULL;
  PSECURITY_DESCRIPTOR file = NULL, directory = NULL;
  BOOL ok = FALSE;
  if (!OpenProcessToken(GetCurrentProcess(), TOKEN_QUERY, &token)) goto done;
  GetTokenInformation(token, TokenUser, NULL, 0, &length);
  user = malloc(length);
  if (!user || !GetTokenInformation(token, TokenUser, user, length, &length) ||
      !ConvertSidToStringSidW(user->User.Sid, &sid)) goto done;
  wchar_t sddl[512];
  swprintf(sddl, 512, L"O:%lsD:P(A;;FA;;;%ls)(A;;FA;;;SY)", sid, sid);
  if (!ConvertStringSecurityDescriptorToSecurityDescriptorW(sddl, SDDL_REVISION_1, &file, NULL)) goto done;
  swprintf(sddl, 512, L"O:%lsD:P(A;OICI;FA;;;%ls)(A;OICI;FA;;;SY)", sid, sid);
  if (!ConvertStringSecurityDescriptorToSecurityDescriptorW(sddl, SDDL_REVISION_1, &directory, NULL)) goto done;
  FileSecurity = file; DirectorySecurity = directory; ok = TRUE;
done:
  if (!ok) { if (file) LocalFree(file); if (directory) LocalFree(directory); }
  if (sid) LocalFree(sid);
  free(user);
  if (token) CloseHandle(token);
  return ok;
}

static void file_error(void)
{
  DWORD e = GetLastError();
  switch (e) {
    case ERROR_FILE_NOT_FOUND: case ERROR_PATH_NOT_FOUND: errno = ENOENT; break;
    case ERROR_ALREADY_EXISTS: case ERROR_FILE_EXISTS: errno = EEXIST; break;
    case ERROR_SHARING_VIOLATION: case ERROR_ACCESS_DENIED: errno = EACCES; break;
    case ERROR_LOCK_VIOLATION: errno = EAGAIN; break;
    case ERROR_DISK_FULL: errno = ENOSPC; break;
    case ERROR_DIR_NOT_EMPTY: errno = ENOTEMPTY; break;
    case ERROR_NOT_SAME_DEVICE: errno = EXDEV; break;
    case ERROR_INVALID_NAME: case ERROR_INVALID_PARAMETER: errno = EINVAL; break;
    default: errno = EIO;
  }
}

static wchar_t *wide_path(const char *path)
{
  if (!path) { errno = EINVAL; return NULL; }
  if (!strcmp(path, "/dev/null")) path = "NUL";
  if (!strcmp(path, "/dev/tty")) path = "CONIN$";
  int n = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, path, -1, NULL, 0);
  if (!n) { errno = EILSEQ; return NULL; }
  wchar_t *out = calloc((size_t) n, sizeof(*out));
  if (out) MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, path, -1, out, n);
  return out;
}

static time_t file_time(FILETIME time)
{
  ULARGE_INTEGER ticks = { .LowPart = time.dwLowDateTime, .HighPart = time.dwHighDateTime };
  return (time_t) ((int64_t) ticks.QuadPart / 10000000 - 11644473600LL);
}

static int handle_stat(HANDLE handle, struct xp_stat *out, int nofollow)
{
  DWORD kind = GetFileType(handle);
  if (kind == FILE_TYPE_CHAR || kind == FILE_TYPE_PIPE)
  {
    memset(out, 0, sizeof(*out));
    out->st_mode = (kind == FILE_TYPE_CHAR ? _S_IFCHR : _S_IFIFO) | 0600;
    out->st_nlink = 1;
    return 0;
  }
  BY_HANDLE_FILE_INFORMATION info;
  if (!GetFileInformationByHandle(handle, &info)) { file_error(); return -1; }
  memset(out, 0, sizeof(*out));
  out->st_dev = info.dwVolumeSerialNumber;
  out->st_ino = ((uint64_t) info.nFileIndexHigh << 32) | info.nFileIndexLow;
  out->st_nlink = info.nNumberOfLinks;
  out->st_size = (int64_t) (((uint64_t) info.nFileSizeHigh << 32) | info.nFileSizeLow);
  out->st_mode = info.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY ? _S_IFDIR | 0700 : _S_IFREG | 0600;
  if (info.dwFileAttributes & FILE_ATTRIBUTE_READONLY) out->st_mode &= ~0200;
  if (nofollow && (info.dwFileAttributes & FILE_ATTRIBUTE_REPARSE_POINT)) out->st_mode = 0120000 | 0700;
  out->st_atime = file_time(info.ftLastAccessTime);
  out->st_mtime = file_time(info.ftLastWriteTime);
  out->st_ctime = file_time(info.ftCreationTime);
  return 0;
}

static int path_stat(const char *path, struct xp_stat *out, int nofollow)
{
  wchar_t *name = wide_path(path);
  if (!name) return -1;
  HANDLE handle = CreateFileW(name, FILE_READ_ATTRIBUTES,
      FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE, NULL, OPEN_EXISTING,
      FILE_FLAG_BACKUP_SEMANTICS | (nofollow ? FILE_FLAG_OPEN_REPARSE_POINT : 0), NULL);
  free(name);
  if (handle == INVALID_HANDLE_VALUE) { file_error(); return -1; }
  int rc = handle_stat(handle, out, nofollow);
  CloseHandle(handle);
  return rc;
}
int xp_stat(const char *path, struct xp_stat *out) { return path_stat(path, out, 0); }
int xp_lstat(const char *path, struct xp_stat *out) { return path_stat(path, out, 1); }
int xp_fstat(int fd, struct xp_stat *out) { return handle_stat((HANDLE) _get_osfhandle(fd), out, 0); }

int xp_open(const char *path, int flags, ...)
{
  int mode = 0600;
  if (flags & _O_CREAT) { va_list args; va_start(args, flags); mode = va_arg(args, int); va_end(args); }
  wchar_t *name = wide_path(path);
  if (!name) return -1;
  DWORD access = flags & _O_RDWR ? GENERIC_READ | GENERIC_WRITE : flags & _O_WRONLY ? GENERIC_WRITE : GENERIC_READ;
  DWORD creation = flags & _O_CREAT ? (flags & _O_EXCL ? CREATE_NEW : flags & _O_TRUNC ? CREATE_ALWAYS : OPEN_ALWAYS)
                                    : flags & _O_TRUNC ? TRUNCATE_EXISTING : OPEN_EXISTING;
  PSECURITY_DESCRIPTOR descriptor = NULL;
  SECURITY_ATTRIBUTES security = { sizeof(security), NULL, FALSE };
  /* Newly created message/config files must not inherit a broad directory ACL. */
  if (flags & _O_CREAT)
  {
    if (!InitOnceExecuteOnce(&SecurityOnce, initialize_security, NULL, NULL))
    { free(name); file_error(); return -1; }
    descriptor = FileSecurity;
    security.lpSecurityDescriptor = descriptor;
  }
  HANDLE handle = CreateFileW(name, access, FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE,
      descriptor ? &security : NULL, creation,
      FILE_ATTRIBUTE_NORMAL | (flags & O_NOFOLLOW ? FILE_FLAG_OPEN_REPARSE_POINT : 0), NULL);
  free(name);
  if (handle == INVALID_HANDLE_VALUE) { file_error(); return -1; }
  if (flags & O_NOFOLLOW)
  {
    FILE_ATTRIBUTE_TAG_INFO tag;
    if (!GetFileInformationByHandleEx(handle, FileAttributeTagInfo, &tag, sizeof(tag)) ||
        tag.FileAttributes & FILE_ATTRIBUTE_REPARSE_POINT)
    { CloseHandle(handle); errno = ELOOP; return -1; }
  }
  int fd = _open_osfhandle((intptr_t) handle, (flags & (_O_RDWR | _O_WRONLY | _O_APPEND)) | _O_BINARY | _O_NOINHERIT);
  if (fd < 0) CloseHandle(handle);
  (void) mode; /* Windows ACLs, rather than POSIX permission bits, enforce privacy. */
  return fd;
}

int xp_mkstemp(char *name)
{
  size_t length = name ? strlen(name) : 0;
  if (length < 6 || strcmp(name + length - 6, "XXXXXX"))
  { errno = EINVAL; return -1; }
  static const char alphabet[] = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789";
  for (int attempt = 0; attempt < 100; attempt++)
  {
    uint64_t random;
    if (BCryptGenRandom(NULL, (PUCHAR) &random, sizeof(random), BCRYPT_USE_SYSTEM_PREFERRED_RNG) != 0)
    { errno = EIO; return -1; }
    for (int i = 0; i < 6; i++)
    { name[length - 6 + i] = alphabet[random % 62]; random /= 62; }
    /* Unlike CRT mkstemp, xp_open permits deleting the open temporary file. */
    int fd = xp_open(name, _O_RDWR | _O_CREAT | _O_EXCL, 0600);
    if (fd >= 0 || errno != EEXIST) return fd;
  }
  errno = EEXIST;
  return -1;
}

FILE *xp_fopen(const char *path, const char *mode)
{
  int flags = strchr(mode, '+') ? _O_RDWR : mode[0] == 'r' ? _O_RDONLY : _O_WRONLY;
  if (mode[0] == 'w') flags |= _O_CREAT | _O_TRUNC;
  else if (mode[0] == 'a') flags |= _O_CREAT | _O_APPEND;
  if (strchr(mode, 'x')) flags |= _O_EXCL;
  int fd = xp_open(path, flags, 0600);
  if (fd < 0) return NULL;
  char binary[8] = { mode[0], 'b', 0 };
  if (strchr(mode, '+')) strcat(binary, "+");
  FILE *file = _fdopen(fd, binary);
  if (!file) _close(fd);
  return file;
}

#define PATH_OPERATION(function, operation) \
int function(const char *path) { wchar_t *p = wide_path(path); if (!p) return -1; \
  int rc = (operation); free(p); if (!rc) { file_error(); return -1; } return 0; }
PATH_OPERATION(xp_unlink, DeleteFileW(p))
PATH_OPERATION(xp_rmdir, RemoveDirectoryW(p))
PATH_OPERATION(xp_chdir, SetCurrentDirectoryW(p))

int xp_mkdir(const char *path, int mode)
{
  wchar_t *p = wide_path(path); if (!p) return -1;
  if (!InitOnceExecuteOnce(&SecurityOnce, initialize_security, NULL, NULL))
  { free(p); file_error(); return -1; }
  SECURITY_ATTRIBUTES security = { sizeof(security), DirectorySecurity, FALSE };
  int rc = CreateDirectoryW(p, &security);
  free(p); (void) mode;
  if (!rc) { file_error(); return -1; } return 0;
}

int xp_rename(const char *source, const char *target)
{
  wchar_t *a = wide_path(source), *b = wide_path(target);
  if (!a || !b) { free(a); free(b); return -1; }
  int rc = MoveFileExW(a, b, MOVEFILE_REPLACE_EXISTING | MOVEFILE_WRITE_THROUGH);
  free(a); free(b); if (!rc) { file_error(); return -1; } return 0;
}
int xp_link(const char *source, const char *target)
{
  wchar_t *a = wide_path(source), *b = wide_path(target);
  if (!a || !b) { free(a); free(b); return -1; }
  int rc = CreateHardLinkW(b, a, NULL);
  free(a); free(b); if (!rc) { file_error(); return -1; } return 0;
}
int xp_symlink(const char *source, const char *target)
{
  wchar_t *a = wide_path(source), *b = wide_path(target);
  if (!a || !b) { free(a); free(b); return -1; }
  DWORD attributes = GetFileAttributesW(a);
  int rc = CreateSymbolicLinkW(b, a, SYMBOLIC_LINK_FLAG_ALLOW_UNPRIVILEGED_CREATE |
      (attributes != INVALID_FILE_ATTRIBUTES && (attributes & FILE_ATTRIBUTE_DIRECTORY) ? SYMBOLIC_LINK_FLAG_DIRECTORY : 0));
  free(a); free(b); if (!rc) { file_error(); return -1; } return 0;
}
int xp_access(const char *path, int mode)
{
  wchar_t *p = wide_path(path); if (!p) return -1;
  int rc = _waccess(p, mode & 6); free(p); return rc;
}
int xp_chmod(const char *path, int mode)
{
  wchar_t *p = wide_path(path); if (!p) return -1;
  int rc = _wchmod(p, mode & (_S_IREAD | _S_IWRITE)); free(p); return rc;
}
int xp_fchmod(int fd, mode_t mode)
{
  FILE_BASIC_INFO info;
  HANDLE handle = (HANDLE) _get_osfhandle(fd);
  if (!GetFileInformationByHandleEx(handle, FileBasicInfo, &info, sizeof(info))) { file_error(); return -1; }
  if (mode & _S_IWRITE) info.FileAttributes &= ~FILE_ATTRIBUTE_READONLY;
  else info.FileAttributes |= FILE_ATTRIBUTE_READONLY;
  if (!SetFileInformationByHandle(handle, FileBasicInfo, &info, sizeof(info))) { file_error(); return -1; }
  return 0;
}
int xp_fsync(int fd)
{
  if (!FlushFileBuffers((HANDLE) _get_osfhandle(fd))) { file_error(); return -1; } return 0;
}
int xp_lock(int fd, int exclusive, int unlock)
{
  OVERLAPPED offset = { 0 };
  HANDLE handle = (HANDLE) _get_osfhandle(fd);
  BOOL rc = unlock ? UnlockFileEx(handle, 0, MAXDWORD, MAXDWORD, &offset) :
      LockFileEx(handle, LOCKFILE_FAIL_IMMEDIATELY | (exclusive ? LOCKFILE_EXCLUSIVE_LOCK : 0),
                 0, MAXDWORD, MAXDWORD, &offset);
  if (!rc) { file_error(); return -1; } return 0;
}
char *xp_getcwd(char *buffer, int size)
{
  wchar_t *path = _wgetcwd(NULL, 0);
  if (!path) return NULL;
  int n = WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS, path, -1, NULL, 0, NULL, NULL);
  if (!n || (buffer && n > size)) { free(path); errno = ERANGE; return NULL; }
  char *out = buffer ? buffer : malloc((size_t) n);
  if (out) {
    WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS, path, -1, out, n, NULL, NULL);
    for (char *p = out; *p; p++) if (*p == '\\') *p = '/';
  }
  free(path); return out;
}
char *xp_realpath(const char *path, char *buffer)
{
  struct xp_stat info;
  if (xp_stat(path, &info) != 0) return NULL;
  wchar_t *p = wide_path(path); if (!p) return NULL;
  DWORD n = GetFullPathNameW(p, 0, NULL, NULL);
  wchar_t *full = calloc((size_t) n + 1, sizeof(*full));
  if (!full) { free(p); return NULL; }
  if (!GetFullPathNameW(p, n + 1, full, NULL)) { free(p); free(full); file_error(); return NULL; }
  free(p);
  int bytes = WideCharToMultiByte(CP_UTF8, 0, full, -1, NULL, 0, NULL, NULL);
  if (buffer && bytes > 260) { free(full); errno = ENAMETOOLONG; return NULL; }
  char *out = buffer ? buffer : malloc((size_t) bytes);
  if (out) { WideCharToMultiByte(CP_UTF8, 0, full, -1, out, bytes, NULL, NULL);
    for (char *at = out; *at; at++) if (*at == '\\') *at = '/'; }
  free(full); return out;
}
#endif

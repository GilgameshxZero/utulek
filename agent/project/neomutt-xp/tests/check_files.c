/* Native file safety regression. SPDX-License-Identifier: GPL-2.0-or-later */
#ifdef _WIN32
#include <windows.h>
#include <winioctl.h>
#include <aclapi.h>
#include <sddl.h>
#include <assert.h>
#include <errno.h>
#include <stdio.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <io.h>
#include <fcntl.h>
#include <dirent.h>
#include "files.h"
int main(void)
{
  SetErrorMode(SEM_FAILCRITICALERRORS | SEM_NOGPFAULTERRORBOX);
  char original[32768], temporary[32768];
  assert(xp_getcwd(original, sizeof(original)));
  assert(GetTempPathA(sizeof(temporary), temporary));
  assert(xp_chdir(temporary) == 0);
  char root[128]; snprintf(root, sizeof(root), "neomutt-xp-files-%lu", (unsigned long) GetCurrentProcessId());
  assert(xp_mkdir(root, 0700) == 0);
  assert(xp_chdir(root) == 0);
  char anonymous[] = "anonymous-XXXXXX";
  int temporary_fd = xp_mkstemp(anonymous); assert(temporary_fd >= 0);
  assert(xp_unlink(anonymous) == 0);
  const char anonymous_data[] = "anonymous\r\nfixture\n";
  FILE *anonymous_file = _fdopen(temporary_fd, "w+"); assert(anonymous_file);
  assert(fwrite(anonymous_data, 1, sizeof(anonymous_data), anonymous_file) == sizeof(anonymous_data));
  assert(fflush(anonymous_file) == 0); rewind(anonymous_file);
  char anonymous_copy[sizeof(anonymous_data)];
  assert(fread(anonymous_copy, 1, sizeof(anonymous_copy), anonymous_file) == sizeof(anonymous_copy));
  assert(memcmp(anonymous_data, anonymous_copy, sizeof(anonymous_data)) == 0);
  assert(fclose(anonymous_file) == 0);
  assert(xp_stat(anonymous, &(struct xp_stat){0}) == -1 && errno == ENOENT);
  struct xp_stat device;
  assert(xp_stat("/dev/null", &device) == 0 && (device.st_mode & _S_IFMT) == _S_IFCHR);
  int nullfd = xp_open("/dev/null", _O_RDWR); assert(nullfd >= 0);
  assert(xp_fstat(nullfd, &device) == 0 && (device.st_mode & _S_IFMT) == _S_IFCHR);
  _close(nullfd);
  int pipes[2]; assert(_pipe(pipes, 256, _O_BINARY) == 0);
  assert(xp_fstat(pipes[0], &device) == 0 && (device.st_mode & _S_IFMT) == _S_IFIFO);
  _close(pipes[0]); _close(pipes[1]);
  const char *first = "Grüße 日本語.eml", *second = "linked.eml";
  int fd = xp_open(first, _O_RDWR | _O_CREAT | _O_EXCL, 0600);
  assert(fd >= 0);
  const char data[] = "Subject: fixture\n\nline\n";
  assert(_write(fd, data, sizeof(data) - 1) == sizeof(data) - 1);
  assert(xp_fsync(fd) == 0);
  struct xp_stat a, b;
  assert(xp_fstat(fd, &a) == 0 && a.st_size == sizeof(data) - 1);
  assert(xp_stat(first, &b) == 0 && a.st_ino == b.st_ino && a.st_dev == b.st_dev);
  assert(xp_open(first, _O_WRONLY | _O_CREAT | _O_EXCL, 0600) == -1 && errno == EEXIST);
  assert(xp_link(first, second) == 0);
  assert(xp_stat(second, &b) == 0 && a.st_ino == b.st_ino);
  assert(xp_link(first, second) == -1 && errno == EEXIST);
  int other = xp_open(first, _O_RDWR);
  assert(other >= 0);
  assert(xp_lock(fd, 1, 0) == 0);
  assert(xp_lock(other, 1, 0) == -1 && errno == EAGAIN);
  assert(xp_lock(fd, 1, 1) == 0);
  assert(xp_lock(other, 1, 0) == 0);
  assert(xp_lock(other, 1, 1) == 0);
  _close(other); _close(fd);
  DIR *dir = opendir("."); assert(dir);
  int seen = 0; struct dirent *entry;
  while ((entry = readdir(dir))) if (!strcmp(entry->d_name, first)) seen++;
  assert(closedir(dir) == 0 && seen == 1);
  char renamed[] = "renamed 日本語.eml";
  assert(xp_rename(first, renamed) == 0);
  FILE *file = xp_fopen(renamed, "r"); assert(file);
  char content[128] = {0};
  assert(fread(content, 1, sizeof(content), file) == sizeof(data) - 1);
  assert(memcmp(content, data, sizeof(data) - 1) == 0);
  fclose(file);
  fd = xp_open("large.sparse", _O_RDWR | _O_CREAT | _O_EXCL, 0600); assert(fd >= 0);
  HANDLE handle = (HANDLE) _get_osfhandle(fd);
  DWORD ignored;
  assert(DeviceIoControl(handle, FSCTL_SET_SPARSE, NULL, 0, NULL, 0, &ignored, NULL));
  LARGE_INTEGER size; size.QuadPart = 5LL * 1024 * 1024 * 1024;
  assert(SetFilePointerEx(handle, size, NULL, FILE_BEGIN) && SetEndOfFile(handle));
  assert(xp_fstat(fd, &a) == 0 && a.st_size == size.QuadPart);
  _close(fd);
  PSECURITY_DESCRIPTOR descriptor = NULL; PACL acl = NULL;
  assert(GetNamedSecurityInfoW(L"large.sparse", SE_FILE_OBJECT, DACL_SECURITY_INFORMATION,
                              NULL, NULL, &acl, NULL, &descriptor) == ERROR_SUCCESS);
  assert(acl && acl->AceCount == 2);
  HANDLE token; DWORD token_size = 0;
  assert(OpenProcessToken(GetCurrentProcess(), TOKEN_QUERY, &token));
  GetTokenInformation(token, TokenUser, NULL, 0, &token_size);
  TOKEN_USER *user = malloc(token_size);
  assert(user && GetTokenInformation(token, TokenUser, user, token_size, &token_size));
  int explicit_user = 0, system = 0;
  for (DWORD i = 0; i < acl->AceCount; i++) {
    ACCESS_ALLOWED_ACE *ace; assert(GetAce(acl, i, (void **) &ace));
    assert(ace->Header.AceType == ACCESS_ALLOWED_ACE_TYPE);
    system += IsWellKnownSid(&ace->SidStart, WinLocalSystemSid) ? 1 : 0;
    explicit_user += EqualSid(&ace->SidStart,user->User.Sid) ? 1 : 0;
  }
  assert(explicit_user == 1 && system == 1);
  free(user); CloseHandle(token);
  SECURITY_DESCRIPTOR_CONTROL control; DWORD revision;
  assert(GetSecurityDescriptorControl(descriptor, &control, &revision) && (control & SE_DACL_PROTECTED));
  LocalFree(descriptor);
  assert(xp_unlink(renamed) == 0 && xp_unlink(second) == 0 && xp_unlink("large.sparse") == 0);
  assert(xp_chdir("..") == 0 && xp_rmdir(root) == 0 && xp_chdir(original) == 0);
  puts("files: Unicode paths/enumeration, binary IO, identity, no-clobber link, locking, rename, 5GiB sparse size, private ACL passed");
  return 0;
}
#else
int main(void) { return 0; }
#endif

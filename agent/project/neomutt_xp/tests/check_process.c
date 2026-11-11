/* Exercise native child IO, inheritance, environment, status and cleanup.
 * SPDX-License-Identifier: GPL-2.0-or-later */
#include <assert.h>
#include <stdio.h>
#include <string.h>
#include <stdlib.h>
#include "xp_process.h"
#ifdef _WIN32
#include <windows.h>
#include <wchar.h>
#endif
int main(int argc, char **argv)
{
#ifdef _WIN32
  if (argc > 1)
  {
    if (strcmp(argv[1], "--environment") == 0)
    {
      const wchar_t *value = _wgetenv(L"XP_VALUE");
      char utf8[1024];
      assert(value);
      int n = WideCharToMultiByte(CP_UTF8, 0, value, -1, utf8, sizeof(utf8), NULL, NULL);
      assert(n > 0);
      fwrite(utf8, 1, (size_t) n - 1, stdout);
      return 0;
    }
    char data[1024];
    while (fgets(data, sizeof(data), stdin)) fputs(data, stdout);
    return 0;
  }
  DWORD before = 0, after = 0;
  GetProcessHandleCount(GetCurrentProcess(), &before);
  char *env[] = { "XP_VALUE=Grüße 日本語 & | % !", "SystemRoot=C:\\Windows", NULL };
  FILE *in = NULL, *out = NULL, *err = NULL;
  pid_t child = xp_filter_create("echo stdout-stream & echo error-stream 1>&2 & exit /b 37",
                                NULL, &out, &err, -1, -1, -1, env);
  assert(child > 0 && out && err);
  char value[512] = {0}, error[512] = {0};
  /* cmd's redirected output uses its configured OEM code page; use ASCII
   * assertions here. Exact UTF-8 inheritance is tested by a native child below. */
  assert(fgets(value, sizeof(value), out));
  assert(fgets(error, sizeof(error), err));
  fclose(out); fclose(err);
  assert(strstr(error, "error-stream"));
  int status;
  assert(xp_waitpid(child, &status, 0) == child && status == (37 << 8));
  wchar_t executable[32768];
  char path[65536], command[65580];
  assert(GetModuleFileNameW(NULL, executable, 32768));
  assert(WideCharToMultiByte(CP_UTF8, 0, executable, -1, path, sizeof(path), NULL, NULL));
  snprintf(command, sizeof(command), "\"%s\" --environment", path);
  child = xp_filter_create(command, NULL, &out, NULL, -1, -1, -1, env);
  assert(child > 0);
  size_t n = fread(value, 1, sizeof(value) - 1, out);
  value[n] = 0;
  fclose(out);
  assert(strcmp(value, "Grüße 日本語 & | % !") == 0);
  assert(xp_waitpid(child, &status, 0) == child && status == 0);
  snprintf(command, sizeof(command), "\"%s\" --echo", path);
  for (int i = 0; i < 100; i++)
  {
    child = xp_filter_create(command, &in, &out, NULL, -1, -1, -1, NULL);
    assert(child > 0);
    fputs("pipe-roundtrip\r\n", in);
    fclose(in);
    memset(value, 0, sizeof(value));
    assert(fgets(value, sizeof(value), out));
    fclose(out);
    assert(strstr(value, "pipe-roundtrip"));
    assert(xp_waitpid(child, &status, 0) == child && status == 0);
    if (i == 0) GetProcessHandleCount(GetCurrentProcess(), &before);
  }
  GetProcessHandleCount(GetCurrentProcess(), &after);
  printf("handles: warm=%lu final=%lu\n", (unsigned long) before, (unsigned long) after);
  assert(after == before);
  puts("process: stdout/stderr, stdin roundtrip, exit37, 100 children, handle count passed");
#else
  (void) argc; (void) argv;
  puts("process: Windows-specific test; Linux uses upstream POSIX implementation");
#endif
  return 0;
}

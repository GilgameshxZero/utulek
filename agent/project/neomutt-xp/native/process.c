/* Native child processes with explicit handle inheritance and UTF-8 environment.
 * SPDX-License-Identifier: GPL-2.0-or-later */
#ifdef _WIN32
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <errno.h>
#include <fcntl.h>
#include <io.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <wchar.h>
#include "xp_process.h"

struct Child { DWORD pid; HANDLE process; struct Child *next; };
static struct Child *Children;

static wchar_t *wide(const char *text)
{
  int length = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, text, -1, NULL, 0);
  if (!length) { errno = EILSEQ; return NULL; }
  wchar_t *result = calloc((size_t) length, sizeof(*result));
  if (!result) return NULL;
  if (!MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, text, -1, result, length))
  { free(result); errno = EILSEQ; return NULL; }
  return result;
}


/* Encode one argument for the Windows argv convention. The shell receives the
 * original command string, including its POSIX quotes, without a cmd.exe pass. */
static wchar_t *quote_argument(const wchar_t *text)
{
  size_t length = wcslen(text);
  wchar_t *out = calloc(length * 2 + 3, sizeof(*out));
  if (!out) return NULL;
  wchar_t *at = out; *at++ = L'"';
  size_t slashes = 0;
  for (const wchar_t *p = text; ; p++)
  {
    if (*p == L'\\') { slashes++; continue; }
    size_t copies = (*p == L'"' || !*p) ? slashes * 2 : slashes;
    while (copies--) *at++ = L'\\';
    slashes = 0;
    if (!*p) break;
    if (*p == L'"') *at++ = L'\\';
    *at++ = *p;
  }
  *at++ = L'"'; *at = 0;
  return out;
}

static int env_compare(const void *a, const void *b)
{
  return _wcsicmp(*(const wchar_t *const *) a, *(const wchar_t *const *) b);
}

static wchar_t *environment_block(char **environment)
{
  size_t count = 0, length = 1;
  while (environment[count]) count++;
  wchar_t **entries = calloc(count + 1, sizeof(*entries));
  if (!entries) return NULL;
  for (size_t i = 0; i < count; i++)
  {
    entries[i] = wide(environment[i]);
    if (!entries[i])
    { for (size_t j = 0; j < i; j++) free(entries[j]); free(entries); return NULL; }
    length += wcslen(entries[i]) + 1;
  }
  qsort(entries, count, sizeof(*entries), env_compare);
  /* Windows requires a double NUL even for an empty environment. */
  wchar_t *block = calloc(length + 1, sizeof(*block));
  if (block)
  {
    wchar_t *at = block;
    for (size_t i = 0; i < count; i++)
    { size_t n = wcslen(entries[i]) + 1; memcpy(at, entries[i], n * sizeof(*at)); at += n; }
  }
  for (size_t i = 0; i < count; i++) free(entries[i]);
  free(entries);
  return block;
}

static HANDLE inherited_fd(int fd, DWORD standard, int writing)
{
  HANDLE original = fd >= 0 ? (HANDLE) _get_osfhandle(fd) : GetStdHandle(standard);
  HANDLE result = NULL;
  if (original && original != INVALID_HANDLE_VALUE &&
      DuplicateHandle(GetCurrentProcess(), original, GetCurrentProcess(), &result,
                      0, TRUE, DUPLICATE_SAME_ACCESS)) return result;
  SECURITY_ATTRIBUTES security = { sizeof(security), NULL, TRUE };
  return CreateFileW(L"NUL", writing ? GENERIC_WRITE : GENERIC_READ,
                     FILE_SHARE_READ | FILE_SHARE_WRITE, &security, OPEN_EXISTING, 0, NULL);
}

static pid_t spawn_process(const char *command, const char *application, const char *const *arguments,
                       FILE **input, FILE **output, FILE **error, int fdin, int fdout, int fderr, char **environment)
{
  FILE **streams[3] = { input, output, error };
  int descriptors[3] = { fdin, fdout, fderr };
  DWORD standards[3] = { STD_INPUT_HANDLE, STD_OUTPUT_HANDLE, STD_ERROR_HANDLE };
  HANDLE child[3] = { NULL }, parent[3] = { NULL };
  FILE *files[3] = { NULL };
  wchar_t *env = NULL, *line = NULL, *shell = NULL, *cmd = NULL;
  STARTUPINFOEXW startup = { 0 };
  PROCESS_INFORMATION process = { 0 };
  struct Child *entry = calloc(1, sizeof(*entry));
  pid_t result = -1;
  int saved_errno = EIO;
  for (int i = 0; i < 3; i++) if (streams[i]) *streams[i] = NULL;
  if ((!command && !application) || !entry) { saved_errno = entry ? EINVAL : ENOMEM; goto cleanup; }
  SECURITY_ATTRIBUTES security = { sizeof(security), NULL, TRUE };
  for (int i = 0; i < 3; i++)
  {
    if (streams[i])
    {
      HANDLE read_end, write_end;
      if (!CreatePipe(&read_end, &write_end, &security, 0)) goto cleanup;
      parent[i] = i == 0 ? write_end : read_end;
      child[i] = i == 0 ? read_end : write_end;
      if (!SetHandleInformation(parent[i], HANDLE_FLAG_INHERIT, 0)) goto cleanup;
      int fd = _open_osfhandle((intptr_t) parent[i], (i == 0 ? _O_WRONLY : _O_RDONLY) | _O_BINARY);
      if (fd < 0) goto cleanup;
      parent[i] = NULL; /* descriptor now owns the handle */
      files[i] = _fdopen(fd, i == 0 ? "wb" : "rb");
      if (!files[i]) { _close(fd); goto cleanup; }
    }
    else
    {
      child[i] = inherited_fd(descriptors[i], standards[i], i != 0);
      if (!child[i] || child[i] == INVALID_HANDLE_VALUE) { child[i] = NULL; goto cleanup; }
    }
  }
  if (application)
  {
    wchar_t *name = wide(application);
    if (!name) goto cleanup;
    wchar_t resolved[32768];
    DWORD found = SearchPathW(NULL, name, L".exe", 32768, resolved, NULL);
    free(name);
    if (!found || found >= 32768) { saved_errno = ENOENT; goto cleanup; }
    shell = _wcsdup(resolved);
    size_t length = 1;
    for (size_t i = 0; arguments[i]; i++)
    {
      wchar_t *value = wide(arguments[i]);
      if (!value) goto cleanup;
      wchar_t *quoted = quote_argument(value); free(value);
      if (!quoted) goto cleanup;
      size_t count = wcslen(quoted);
      wchar_t *grown = realloc(line, (length + count + 1) * sizeof(*line));
      if (!grown) { free(quoted); goto cleanup; }
      line = grown;
      if (length == 1) line[0] = 0; else wcscat(line, L" ");
      wcscat(line, quoted); length += count + 1;
      free(quoted);
    }
    if (!line) { saved_errno = EINVAL; goto cleanup; }
  }
  else
  {
  wchar_t shell_path[32768];
  DWORD n = GetEnvironmentVariableW(L"NEOMUTT_XP_SHELL", shell_path, 32768);
  int posix_shell = n > 0 && n < 32768;
  if (!posix_shell)
  {
    n = GetEnvironmentVariableW(L"COMSPEC", shell_path, 32768);
    if (!n || n >= 32768)
    {
      n = GetSystemDirectoryW(shell_path, 32750);
      if (!n || n >= 32750) goto cleanup;
      wcscat(shell_path, L"\\cmd.exe");
    }
  }
  shell = _wcsdup(shell_path);
  if (posix_shell)
  {
    /* Bash finds native .exe programs itself. Add Windows' implicit batch
     * suffixes only on lookup failure, keeping the original POSIX command and
     * argument quoting intact. Shared extensionless helpers still win. */
    static const char fallback[] =
      "command_not_found_handle() { local xp_command xp_candidate xp_dir IFS=:; xp_command=$1; shift; "
      "for xp_dir in $PATH; do for xp_candidate in "
      "\"${xp_dir:-.}/$xp_command.bat\" \"${xp_dir:-.}/$xp_command.cmd\"; do "
      "if [ -f \"$xp_candidate\" ]; then \"$xp_candidate\" \"$@\"; return $?; fi; done; done; "
      "printf '%s: command not found\\n' \"$xp_command\" >&2; return 127; }; ";
    size_t length = sizeof(fallback) + strlen(command);
    char *expanded = malloc(length);
    if (!expanded) goto cleanup;
    strcpy(expanded, fallback); strcat(expanded, command);
    cmd = wide(expanded); free(expanded);
  }
  else cmd = wide(command);
  if (!shell || !cmd) goto cleanup;
  wchar_t *argument = posix_shell ? quote_argument(cmd) : _wcsdup(cmd);
  if (!argument) goto cleanup;
  size_t length = wcslen(shell) + wcslen(argument) + 64;
  line = calloc(length, sizeof(*line));
  if (!line) { free(argument); goto cleanup; }
  if (posix_shell)
    swprintf(line, length, L"\"%ls\" --noprofile --norc --posix -c %ls", shell, argument);
  else
    swprintf(line, length, L"\"%ls\" /d /v:off /s /c \"%ls\"", shell, argument);
  free(argument);
  }
  if (environment && !(env = environment_block(environment))) goto cleanup;
  startup.StartupInfo.cb = sizeof(startup);
  startup.StartupInfo.dwFlags = STARTF_USESTDHANDLES;
  startup.StartupInfo.hStdInput = child[0];
  startup.StartupInfo.hStdOutput = child[1];
  startup.StartupInfo.hStdError = child[2];
  SIZE_T bytes = 0;
  InitializeProcThreadAttributeList(NULL, 1, 0, &bytes);
  startup.lpAttributeList = malloc(bytes);
  if (!startup.lpAttributeList) goto cleanup;
  if (!InitializeProcThreadAttributeList(startup.lpAttributeList, 1, 0, &bytes))
  { free(startup.lpAttributeList); startup.lpAttributeList = NULL; goto cleanup; }
  if (!UpdateProcThreadAttribute(startup.lpAttributeList, 0,
          PROC_THREAD_ATTRIBUTE_HANDLE_LIST, child, sizeof(child), NULL, NULL)) goto cleanup;
  if (!CreateProcessW(shell, line, NULL, NULL, TRUE,
         EXTENDED_STARTUPINFO_PRESENT | CREATE_UNICODE_ENVIRONMENT, env, NULL,
         &startup.StartupInfo, &process)) goto cleanup;
  CloseHandle(process.hThread);
  entry->pid = process.dwProcessId;
  entry->process = process.hProcess;
  entry->next = Children;
  Children = entry;
  entry = NULL;
  for (int i = 0; i < 3; i++) if (streams[i]) { *streams[i] = files[i]; files[i] = NULL; }
  result = (pid_t) process.dwProcessId;
cleanup:
  if (startup.lpAttributeList)
  { DeleteProcThreadAttributeList(startup.lpAttributeList); free(startup.lpAttributeList); }
  for (int i = 0; i < 3; i++)
  {
    if (child[i]) CloseHandle(child[i]);
    if (parent[i]) CloseHandle(parent[i]);
    if (files[i]) fclose(files[i]);
  }
  free(entry); free(env); free(line); free(shell); free(cmd);
  if (result == -1) errno = saved_errno;
  return result;
}

pid_t xp_filter_create(const char *command, FILE **input, FILE **output,
                       FILE **error, int fdin, int fdout, int fderr, char **environment)
{ return spawn_process(command, NULL, NULL, input, output, error, fdin, fdout, fderr, environment); }
pid_t xp_spawn_arguments(const char *application, const char *const *arguments,
                         int input, int output, int error, char **environment)
{ return spawn_process(NULL, application, arguments, NULL, NULL, NULL, input, output, error, environment); }

int xp_waitpid_timeout(pid_t pid, int *status, int milliseconds)
{
  struct Child **at = &Children;
  while (*at && (*at)->pid != (DWORD) pid) at = &(*at)->next;
  if (!*at) { errno = ECHILD; return -1; }
  struct Child *child = *at;
  DWORD wait = WaitForSingleObject(child->process, milliseconds < 0 ? INFINITE : (DWORD) milliseconds);
  if (wait == WAIT_TIMEOUT) return 0;
  if (wait != WAIT_OBJECT_0) { errno = EIO; return -1; }
  DWORD code;
  if (!GetExitCodeProcess(child->process, &code)) { errno = EIO; return -1; }
  if (status) *status = (int) ((code & 255) << 8);
  *at = child->next;
  CloseHandle(child->process);
  free(child);
  return (int) pid;
}
int xp_waitpid(pid_t pid, int *status, int flags)
{ return xp_waitpid_timeout(pid, status, flags & 1 ? 0 : -1); }
int xp_detach_process(pid_t pid)
{
  struct Child **at = &Children;
  while (*at && (*at)->pid != (DWORD) pid) at = &(*at)->next;
  if (!*at) { errno = ECHILD; return -1; }
  struct Child *child = *at; *at = child->next;
  CloseHandle(child->process); free(child); return 0;
}
#endif

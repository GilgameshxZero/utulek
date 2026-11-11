/* Embedded read-only resources; temporary backing files are delete-pending.
 * SPDX-License-Identifier: GPL-2.0-or-later */
#ifdef _WIN32
#include "compat.h"
#include <errno.h>

const char *xp_resource_data(unsigned id, size_t *size)
{
  HRSRC resource = FindResourceW(NULL, MAKEINTRESOURCEW(id), MAKEINTRESOURCEW(10));
  HGLOBAL loaded = resource ? LoadResource(NULL, resource) : NULL;
  const char *data = loaded ? LockResource(loaded) : NULL;
  *size = resource ? SizeofResource(NULL, resource) : 0;
  if (!data || !*size) { errno = ENOENT; return NULL; }
  return data;
}

char *xp_resource_fgets(char *buffer, size_t capacity, const char **cursor, size_t *left)
{
  if (!*left || capacity < 2) return NULL;
  size_t count = *left < capacity - 1 ? *left : capacity - 1;
  const char *newline = memchr(*cursor, '\n', count);
  if (newline) count = (size_t) (newline - *cursor) + 1;
  memcpy(buffer, *cursor, count);
  buffer[count] = 0;
  *cursor += count;
  *left -= count;
  return buffer;
}

FILE *xp_resource_open(unsigned id)
{
  size_t size;
  const char *data = xp_resource_data(id, &size);
  if (!data) return NULL;
  const char *directory = getenv("TMPDIR");
  if (!directory || !*directory) directory = getenv("TEMP");
  if (!directory || !*directory) { errno = ENOENT; return NULL; }
  size_t length = strlen(directory) + 40;
  char *name = malloc(length);
  if (!name) { errno = ENOMEM; return NULL; }
  snprintf(name, length, "%s/neomutt-resource-XXXXXX", directory);
  int fd = xp_mkstemp(name);
  if (fd < 0) { free(name); return NULL; }
  /* All handles share deletion: remove the directory entry immediately, and
   * let Windows reclaim its storage even if the application terminates early. */
  if (xp_unlink(name) < 0) { _close(fd); xp_unlink(name); free(name); return NULL; }
  free(name);
  FILE *file = _fdopen(fd, "w+b");
  if (!file) { _close(fd); return NULL; }
  if (fwrite(data, 1, size, file) != size || fseek(file, 0, SEEK_SET) != 0)
  { fclose(file); return NULL; }
  return file;
}
#endif

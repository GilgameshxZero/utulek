/* Capture stdin/argv to a fixture path; this program never delivers mail.
 * SPDX-License-Identifier: GPL-2.0-or-later */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#ifdef _WIN32
#include <windows.h>
#include <io.h>
#include <fcntl.h>
#else
#include <time.h>
#endif
int main(int argc, char **argv)
{
  const char *path = getenv("NEOMUTT_XP_CAPTURE");
  if (!path || !*path) return 2;
#ifdef _WIN32
  _setmode(_fileno(stdin), _O_BINARY);
#endif
  FILE *capture = fopen(path, "wb");
  if (!capture) return 3;
  unsigned char buffer[4096];
  size_t count;
  while ((count = fread(buffer, 1, sizeof(buffer), stdin)))
    if (fwrite(buffer, 1, count, capture) != count) return 4;
  if (ferror(stdin) || fclose(capture)) return 5;
  char arguments[32768];
  if (snprintf(arguments, sizeof(arguments), "%s.args", path) >= (int) sizeof(arguments)) return 6;
  capture = fopen(arguments, "wb"); if (!capture) return 7;
  for (int i = 0; i < argc; i++) fprintf(capture, "%s\n", argv[i]);
  if (fclose(capture)) return 8;
  const char *delay = getenv("NEOMUTT_XP_SENDMAIL_DELAY_MS");
  if (delay)
  {
    unsigned ms = (unsigned) strtoul(delay, NULL, 10);
#ifdef _WIN32
    Sleep(ms);
#else
    struct timespec time = { ms / 1000, (ms % 1000) * 1000000L };
    nanosleep(&time, NULL);
#endif
  }
  const char *status = getenv("NEOMUTT_XP_SENDMAIL_EXIT");
  return status ? atoi(status) : 0;
}

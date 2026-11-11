/* Synthetic noninteractive editor; touches only its supplied fixture draft.
 * SPDX-License-Identifier: GPL-2.0-or-later */
#include <stdio.h>
#ifdef _WIN32
#include <wchar.h>
int wmain(int argc, wchar_t **argv)
{
  if (argc != 2) return 2;
  FILE *draft = _wfopen(argv[1], L"ab");
#else
int main(int argc, char **argv)
{
  if (argc != 2) return 2;
  FILE *draft = fopen(argv[1], "ab");
#endif
  if (!draft) return 3;
  fputs("\nEditor fixture: Grüße 日本語\n", draft);
  if (fclose(draft)) return 4;
  FILE *marker = fopen("editor-called.txt", "wb");
  if (!marker) return 5;
  fputs("editor-handoff-ok\n", marker);
  return fclose(marker) ? 6 : 0;
}

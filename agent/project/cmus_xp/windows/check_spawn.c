/* External command behavior checks for the native Windows build.
 * GPL-2.0-or-later. */
#include "compat.h"
#include "../spawn.h"

#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <wchar.h>

static int failures;

static void check(int condition, const char *name)
{
	if (!condition) {
		fprintf(stderr, "FAIL: %s\n", name);
		failures++;
	} else {
		printf("ok: %s\n", name);
	}
}

static char *module_path(void)
{
	wchar_t wide[32768];
	char *utf8 = malloc(4 * 32768);
	DWORD length = GetModuleFileNameW(NULL, wide, 32768);
	if (!length || !utf8)
		return NULL;
	int bytes = WideCharToMultiByte(CP_UTF8, 0, wide, (int)length, utf8,
			(int)(4 * 32768) - 1, NULL, NULL);
	if (bytes <= 0) {
		free(utf8);
		return NULL;
	}
	utf8[bytes] = 0;
	return utf8;
}

static char *temp_path(const wchar_t *name)
{
	wchar_t directory[32768], path[32768];
	DWORD length = GetTempPathW(32768, directory);
	char *utf8 = malloc(32768);
	if (!length || !utf8)
		return NULL;
	swprintf(path, 32768, L"%s%ls", directory, name);
	int bytes = WideCharToMultiByte(CP_UTF8, 0, path, -1, utf8, 32767, NULL, NULL);
	if (bytes <= 0) {
		free(utf8);
		return NULL;
	}
	return utf8;
}

static char *read_file(const char *path)
{
	FILE *file = fopen(path, "rb");
	char *data;
	long size;
	if (!file)
		return NULL;
	fseek(file, 0, SEEK_END);
	size = ftell(file);
	fseek(file, 0, SEEK_SET);
	data = malloc((size_t)size + 1);
	if (!data || fread(data, 1, (size_t)size, file) != (size_t)size) {
		free(data);
		fclose(file);
		return NULL;
	}
	data[size] = 0;
	fclose(file);
	return data;
}

static int run_child(const char *self, const char *output,
		const char *const *arguments, int count)
{
	char *argv[32];
	int n = 0;
	argv[n++] = (char *)self;
	argv[n++] = "--echo";
	argv[n++] = (char *)output;
	for (int i = 0; i < count; i++)
		argv[n++] = (char *)arguments[i];
	argv[n] = NULL;
	int status = -1;
	int rc = spawn(argv, &status, 1);
	return rc ? -1 : (status >> 8) & 255;
}

int main(int argc, char **argv)
{
	if (argc > 2 && !strcmp(argv[1], "--echo")) {
		FILE *file = fopen(argv[2], "wb");
		if (!file)
			return 100;
		for (int i = 3; i < argc; i++) {
			fwrite(argv[i], 1, strlen(argv[i]), file);
			fputc('\n', file);
		}
		fclose(file);
		return 0;
	}
	if (argc == 3 && !strcmp(argv[1], "--exit"))
		return atoi(argv[2]);

	char *self = module_path();
	char *output = temp_path(L"cmus_spawn_echo.txt");
	char *batch_output = temp_path(L"cmus_spawn_batch.txt");
	char *batch = temp_path(L"cmus_spawn_test.bat");
	char *shell_output = temp_path(L"cmus_spawn_shell.txt");
	char *missing = "definitely-not-a-cmus-command.exe";
	int status = -1;
	int rc;

	check(self && output && batch && batch_output && shell_output, "temp paths");
	if (!self || !output || !batch || !batch_output || !shell_output)
		return 1;

	const char *edge[] = { "plain", "two words", "", "quote\"inside",
		"trail\\\\", "back\\slash", "\xc3\xbcnicode \xe2\x9c\x93" };
	check(run_child(self, output, edge, 7) == 0, "direct spawn exit status");
	char *contents = read_file(output);
	check(contents && strstr(contents, "\xc3\xbcnicode \xe2\x9c\x93\n"), "direct spawn unicode");
	check(contents && strstr(contents, "quote\"inside\n"), "direct spawn quote");
	check(contents && strstr(contents, "back\\slash\n"), "direct spawn backslash");
	check(contents && strstr(contents, "\ntrail\\\\\n"), "direct spawn trailing backslashes");

	char *exit_argv[] = { self, "--exit", "7", NULL };
	rc = spawn(exit_argv, &status, 1);
	check(rc == 0 && ((status >> 8) & 255) == 7, "nonzero exit status");

	char *missing_argv[] = { missing, NULL };
	errno = 0;
	rc = spawn(missing_argv, NULL, 1);
	check(rc == -1 && errno == ENOENT, "missing executable");

	FILE *file = fopen(batch, "wb");
	if (file) {
		fprintf(file, "@echo off\r\n> \"%%~2\" echo %%~1\r\n");
		fclose(file);
	}
	char *batch_argv[] = { batch, "batch value", batch_output, NULL };
	rc = spawn(batch_argv, &status, 1);
	contents = read_file(batch_output);
	check(rc == 0 && contents && strstr(contents, "batch value"), "batch dispatch");

	char command[4096];
	snprintf(command, sizeof(command), "echo shell-value> \"%s\"", shell_output);
	check(spawn_shell(command) == 0, "spawn_shell");
	contents = read_file(shell_output);
	check(contents && strstr(contents, "shell-value"), "shell output");

	DeleteFileA(output);
	DeleteFileA(batch_output);
	DeleteFileA(batch);
	DeleteFileA(shell_output);
	printf("%s (%d failures)\n", failures ? "FAILED" : "PASS", failures);
	return failures ? 1 : 0;
}

int win_resource(const char *name, const char **data, size_t *size)
{
	(void)name; (void)data; (void)size;
	return 0;
}

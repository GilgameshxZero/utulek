/* Legacy metadata encoding checks for the native Windows build.
 * GPL-2.0-or-later. */
#include "compat.h"
#include "../convert.h"
#include "../id3.h"

#include <errno.h>
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

char *id3_default_charset = "ISO-8859-1";

static int failures;

static void check(int condition, const char *name, const char *detail)
{
	if (!condition) {
		fprintf(stderr, "FAIL: %s (%s)\n", name, detail ? detail : "");
		failures++;
	} else {
		printf("ok: %s\n", name);
	}
}

static void expect_convert(const char *name, const char *input, int size,
		const char *from, const char *expected)
{
	char *output = NULL;
	ssize_t length = convert(input, size, &output, -1, "UTF-8", from);
	check(length >= 0 && output && !memcmp(output, expected, (size_t)length) &&
			output[length] == 0, name, output ? output : "convert failed");
	free(output);
}

static char *temp_name(const wchar_t *name)
{
	wchar_t directory[32768], path[32768];
	char *utf8 = malloc(32768);
	DWORD length = GetTempPathW(32768, directory);
	if (!length || !utf8)
		return NULL;
	swprintf(path, 32768, L"%s%ls", directory, name);
	if (WideCharToMultiByte(CP_UTF8, 0, path, -1, utf8, 32767, NULL, NULL) <= 0) {
		free(utf8);
		return NULL;
	}
	return utf8;
}

static void put_be32(unsigned char *p, unsigned int value)
{
	p[0] = (unsigned char)(value >> 24);
	p[1] = (unsigned char)(value >> 16);
	p[2] = (unsigned char)(value >> 8);
	p[3] = (unsigned char)value;
}

static int write_id3v2(const char *path, const unsigned char *text, int text_size)
{
	unsigned char tag[256];
	int frame_size = text_size + 1;
	int body_size = 10 + frame_size;
	int tag_size = 10 + body_size;
	tag[0] = 'I'; tag[1] = 'D'; tag[2] = '3';
	tag[3] = 3; tag[4] = 0; tag[5] = 0;
	tag[6] = (unsigned char)((body_size >> 21) & 0x7f);
	tag[7] = (unsigned char)((body_size >> 14) & 0x7f);
	tag[8] = (unsigned char)((body_size >> 7) & 0x7f);
	tag[9] = (unsigned char)(body_size & 0x7f);
	memcpy(tag + 10, "TPE1", 4);
	put_be32(tag + 14, (unsigned int)frame_size);
	tag[18] = 0; tag[19] = 0;
	tag[20] = 0;
	memcpy(tag + 21, text, (size_t)text_size);
	int fd = open(path, O_CREAT | O_TRUNC | O_WRONLY | O_BINARY, 0600);
	if (fd == -1)
		return -1;
	int written = (int)write(fd, tag, (size_t)tag_size);
	close(fd);
	return written == tag_size ? 0 : -1;
}

static int write_id3v1(const char *path, const char *artist)
{
	unsigned char tag[128];
	memset(tag, 0, sizeof(tag));
	memcpy(tag, "TAG", 3);
	size_t length = strlen(artist);
	if (length > 30)
		length = 30;
	memcpy(tag + 33, artist, length);
	int fd = open(path, O_WRONLY | O_BINARY);
	if (fd == -1)
		return -1;
	lseek(fd, 0, SEEK_END);
	int written = (int)write(fd, tag, sizeof(tag));
	close(fd);
	return written == (int)sizeof(tag) ? 0 : -1;
}

int main(void)
{
	char *text_file = temp_name(L"cmus_legacy_v2.mp3");
	char *v1_file = temp_name(L"cmus_legacy_v1.mp3");
	struct id3tag id3;
	int fd;

	expect_convert("ISO-8859-1 to UTF-8", "Caf\xe9", 4, "ISO-8859-1", "Caf\xc3\xa9");
	expect_convert("Windows-1251 to UTF-8",
			"\xcf\xf0\xe8\xe2\xe5\xf2", 6, "CP1251", "\xd0\x9f\xd1\x80\xd0\xb8\xd0\xb2\xd0\xb5\xd1\x82");
	expect_convert("Shift_JIS to UTF-8", "\x93\xfa\x96\x7b", 4, "Shift_JIS",
			"\xe6\x97\xa5\xe6\x9c\xac");
	expect_convert("UTF-8 lower ASCII", "plain", 5, "UTF-8", "plain");
	{
		char *output = NULL;
		ssize_t length = convert("Caf\xc3\xa9", 5, &output, -1, "ISO-8859-1", "UTF-8");
		check(length == 4 && output && !memcmp(output, "Caf\xe9", 4), "UTF-8 to ISO-8859-1", "");
		free(output);
	}

	if (!text_file || !v1_file) {
		fprintf(stderr, "temp paths failed\n");
		return 1;
	}
	if (write_id3v2(text_file, (const unsigned char *)"Caf\xe9", 4)) {
		fprintf(stderr, "could not write ID3v2 fixture\n");
		return 1;
	}
	id3_init(&id3);
	fd = open(text_file, O_RDONLY | O_BINARY);
	int rc = fd == -1 ? -1 : id3_read_tags(&id3, fd, ID3_V2);
	if (fd != -1)
		close(fd);
	char *artist = rc == 0 ? id3_get_comment(&id3, ID3_ARTIST) : NULL;
	check(artist && !strcmp(artist, "Caf\xc3\xa9"), "ID3v2 ISO-8859-1 tag", artist ? artist : "missing");
	free(artist);
	id3_free(&id3);

	if (write_id3v2(v1_file, (const unsigned char *)"", 0) ||
			write_id3v1(v1_file, "\xcf\xf0\xe8\xe2\xe5\xf2")) {
		fprintf(stderr, "could not write ID3v1 fixture\n");
		return 1;
	}
	id3_default_charset = "CP1251";
	id3_init(&id3);
	fd = open(v1_file, O_RDONLY | O_BINARY);
	rc = fd == -1 ? -1 : id3_read_tags(&id3, fd, ID3_V1);
	if (fd != -1)
		close(fd);
	artist = rc == 0 ? id3_get_comment(&id3, ID3_ARTIST) : NULL;
	check(artist && !strcmp(artist, "\xd0\x9f\xd1\x80\xd0\xb8\xd0\xb2\xd0\xb5\xd1\x82"),
			"ID3v1 CP1251 tag", artist ? artist : "missing");
	free(artist);
	id3_free(&id3);

	unlink(text_file);
	unlink(v1_file);
	free(text_file);
	free(v1_file);
	printf("%s (%d failures)\n", failures ? "FAILED" : "PASS", failures);
	return failures ? 1 : 0;
}

const char *program_name = "check_legacy_tags";
char *charset = "UTF-8";
int using_utf8 = 1;
char *clipped_text_internal;
int win_resource(const char *name, const char **data, size_t *size)
{
	(void)name; (void)data; (void)size;
	return 0;
}

/* Exercise the same FFmpeg plugin locally, without a terminal or audio device.
 * GPL-2.0-or-later. Arguments: input file, decoded PCM output.
 */
#include "../ip.h"
#include "../keyval.h"
#include <fcntl.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <time.h>

extern const struct input_plugin_ops ip_ops;
/* UI globals referenced by shared text helpers on PE linkers. */
char *charset = "UTF-8";
char *clipped_text_internal = "...";
int using_utf8 = 1;

int main(int argc, char **argv)
{
	if (argc != 3 && argc != 4)
		return 2;
	struct input_plugin_data data = {0};
	data.filename = argv[1];
	data.fd = open(argv[1], O_RDONLY
#ifdef _WIN32
		| O_BINARY
#endif
	);
	if (ip_ops.open(&data) != 0)
		return 3;
	int discard = strcmp(argv[2], "-") == 0;
	FILE *out = discard ? NULL : fopen(argv[2], "wb");
	if (!discard && !out)
		return 4;
	FILE *reference = argc == 4 ? fopen(argv[3], "rb") : NULL;
	if (argc == 4 && !reference)
		return 4;
	int reference_ok = 1;
	char buffer[8192];
	char first[8192];
	int first_size = 0;
	clock_t start = clock();
	uint64_t total = 0;
	int bytes;
	while ((bytes = ip_ops.read(&data, buffer, sizeof(buffer))) > 0) {
		if (bytes > sizeof(buffer) || (!discard && fwrite(buffer, 1, bytes, out) != bytes))
			return 5;
		if (!first_size) {
			first_size = bytes;
			memcpy(first, buffer, bytes);
		}
		if (reference) {
			char expected[sizeof(buffer)];
			if (fread(expected, 1, bytes, reference) != (size_t)bytes) {
				reference_ok = 0;
			} else {
				int width = sf_get_bits(data.sf) / 8;
				int64_t tolerance = width == 2 ? 1 : 21474;
				for (int i = 0; i < bytes; i += width) {
					int64_t a, b;
					if (width == 2) {
						int16_t x, y;
						memcpy(&x, buffer + i, 2); memcpy(&y, expected + i, 2);
						a = x; b = y;
					} else {
						int32_t x, y;
						memcpy(&x, buffer + i, 4); memcpy(&y, expected + i, 4);
						a = x; b = y;
					}
					if (llabs(a - b) > tolerance) reference_ok = 0;
				}
			}
		}
		total += bytes;
		if (total > (uint64_t)1024 * 1024 * 1024)
			return 6;
	}
	double decode_cpu_seconds = (double)(clock() - start) / CLOCKS_PER_SEC;
	if (out) fclose(out);
	if (reference) {
		if (fgetc(reference) != EOF) reference_ok = 0;
		fclose(reference);
	}
	if (bytes < 0)
		return 7;
	struct keyval *comments = NULL;
	if (ip_ops.read_comments(&data, &comments) != 0)
		return 8;
	const char *title = keyvals_get_val(comments, "title");
	int title_ok = title && strcmp(title, "Decoder check") == 0;
	int seek_ok = ip_ops.seek(&data, 1.0) == 0 &&
		ip_ops.read(&data, buffer, sizeof(buffer)) > 0;
	/* Seeking to the beginning after EOF must restart the decoder. */
	int restart_ok = ip_ops.seek(&data, 0.0) == 0 &&
		ip_ops.read(&data, buffer, sizeof(buffer)) > 0;
	int restart_pcm_ok = restart_ok && !memcmp(first, buffer, first_size);
	/* Read the entire restarted stream using deliberately unaligned requests. */
	uint64_t restarted = restart_ok ? first_size : 0;
	while (restart_ok && (bytes = ip_ops.read(&data, buffer, sizeof(buffer) - 1)) > 0)
		restarted += bytes;
	restart_ok = restart_ok && bytes == 0 && restarted == total;
	char *codec = ip_ops.codec(&data);
	printf("{\"bytes\":%llu,\"rate\":%u,\"channels\":%u,\"bits\":%u,"
		"\"duration\":%d,\"codec\":\"%s\",\"title_ok\":%d,\"seek_ok\":%d,\"restart_ok\":%d,"
		"\"restart_pcm_ok\":%d,\"reference_ok\":%d,\"restarted_bytes\":%llu,\"decode_cpu_seconds\":%.6f}\n",
		(unsigned long long)total, sf_get_rate(data.sf), sf_get_channels(data.sf),
		sf_get_bits(data.sf), ip_ops.duration(&data), codec, title_ok, seek_ok, restart_ok,
		restart_pcm_ok, reference_ok, (unsigned long long)restarted, decode_cpu_seconds);
	free(codec);
	keyvals_free(comments);
	ip_ops.close(&data);
	if (data.fd >= 0) close(data.fd);
	return !total || !seek_ok || !restart_ok || !restart_pcm_ok || !reference_ok;
}

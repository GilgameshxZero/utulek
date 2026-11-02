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

extern const struct input_plugin_ops ip_ops;

int main(int argc, char **argv)
{
	if (argc != 3)
		return 2;
	struct input_plugin_data data = {0};
	data.filename = argv[1];
	data.fd = open(argv[1], O_RDONLY);
	if (data.fd < 0 || ip_ops.open(&data) != 0)
		return 3;
	FILE *out = fopen(argv[2], "wb");
	if (!out)
		return 4;
	char buffer[8192];
	uint64_t total = 0;
	int bytes;
	while ((bytes = ip_ops.read(&data, buffer, sizeof(buffer))) > 0) {
		if (bytes > sizeof(buffer) || fwrite(buffer, 1, bytes, out) != bytes)
			return 5;
		total += bytes;
		if (total > 64 * 1024 * 1024)
			return 6;
	}
	fclose(out);
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
	char *codec = ip_ops.codec(&data);
	printf("{\"bytes\":%llu,\"rate\":%u,\"channels\":%u,\"bits\":%u,"
		"\"duration\":%d,\"codec\":\"%s\",\"title_ok\":%d,\"seek_ok\":%d,\"restart_ok\":%d}\n",
		(unsigned long long)total, sf_get_rate(data.sf), sf_get_channels(data.sf),
		sf_get_bits(data.sf), ip_ops.duration(&data), codec, title_ok, seek_ok, restart_ok);
	free(codec);
	keyvals_free(comments);
	ip_ops.close(&data);
	close(data.fd);
	return !total || !title_ok || !seek_ok || !restart_ok;
}

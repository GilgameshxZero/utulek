/* Native Windows CD-DA URL, TOC and error-path checks. GPL-2.0-or-later. */
#include "compat.h"
#include "../discid.h"
#include "../ip.h"

#include <errno.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

char *cdda_device = "D:";

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

int main(void)
{
	char device[16];
	char *disc_id = NULL;
	char *url;
	int tracks = -1;
	int rc;

	win_cdda_default_device(device, sizeof(device));
	check(device[0] != 0, "default device", device);

	rc = win_cdda_disc_info(device, &disc_id, &tracks);
	check(rc == 0 || rc == 1, "disc info returns without crashing", disc_id ? disc_id : "");
	free(disc_id);

	url = complete_cdda_url(device, "cdda://");
	check(url && !strncmp(url, "cdda://", 7) && strstr(url, "/1"), "range URL completion", url ? url : "null");
	free(url);

	int start = -1, end = -1;
	char *parsed = NULL;
	check(parse_cdda_url("cdda://D:/3-5", &parsed, &start, &end) && start == 3 && end == 5,
			"URL parsing", parsed ? parsed : "null");
	free(parsed);

	struct input_plugin_data data = {0};
	data.fd = -1;
	data.filename = "cdda://";
	rc = ip_ops.open(&data);
	check(rc == -IP_ERROR_INVALID_URI, "missing track rejected", "");
	data.filename = "not-a-cdda-url";
	rc = ip_ops.open(&data);
	check(rc == -IP_ERROR_INVALID_URI, "non-cdda URL rejected", "");

	data.filename = complete_cdda_url(device, "cdda://1");
	rc = ip_ops.open(&data);
	check(rc <= 0, "drive open returns an input error", "");
	if (rc == 0) {
		char buffer[588];
		ip_ops.read(&data, buffer, sizeof(buffer));
		ip_ops.close(&data);
	}
	free(data.filename);

	printf("%s (%d failures)\n", failures ? "FAILED" : "PASS", failures);
	return failures ? 1 : 0;
}

const char *program_name = "check_cdda";
int win_resource(const char *name, const char **data, size_t *size)
{
	(void)name; (void)data; (void)size;
	return 0;
}

char *charset = "UTF-8";
int using_utf8 = 1;
char *clipped_text_internal = "...";

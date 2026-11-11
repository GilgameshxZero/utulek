/* Portable Linux runtime paths. GPL-2.0-or-later. */
#include "embedded.h"
#include "xmalloc.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

static void bundle_default(const char *variable, const char *bundle, const char *suffix)
{
	if (getenv(variable))
		return;
	size_t size = strlen(bundle) + strlen(suffix) + 2;
	char *path = xnew(char, size);
	snprintf(path, size, "%s/%s", bundle, suffix);
	setenv(variable, path, 0);
	free(path);
}

void cmus_bundle_init(void)
{
	const char *bundle = getenv("STATICX_BUNDLE_DIR");
	if (!bundle)
		return;
	/* Device/user configuration stays on the host; modules match bundled libs. */
	bundle_default("ALSA_PLUGIN_DIR", bundle, ".");
	bundle_default("ALSA_CONFIG_DIR", bundle, "alsa");
	bundle_default("PIPEWIRE_MODULE_DIR", bundle, ".");
	bundle_default("SPA_PLUGIN_DIR", bundle, "spa");
	bundle_default("PIPEWIRE_CONFIG_DIR", bundle, "pipewire");
	if (!getenv("SSL_CERT_FILE") && !getenv("CURL_CA_BUNDLE") && !getenv("SSL_CERT_DIR")) {
		const char *host_certificates[] = {"/etc/ssl/certs/ca-certificates.crt",
			"/etc/pki/tls/certs/ca-bundle.crt", "/etc/ssl/cert.pem", NULL};
		int found = 0;
		for (int i = 0; host_certificates[i]; i++)
			if (access(host_certificates[i], R_OK) == 0) {
				setenv("SSL_CERT_FILE", host_certificates[i], 0);
				found = 1;
				break;
			}
		if (!found)
			bundle_default("SSL_CERT_FILE", bundle, "certs/ca-certificates.crt");
	}
	/* An empty first terminfo directory retains ncurses's normal host lookup. */
	const char *dirs = getenv("TERMINFO_DIRS");
	if (!dirs)
		dirs = "";
	size_t size = strlen(dirs) + strlen(bundle) + sizeof(":/terminfo");
	char *path = xnew(char, size);
	snprintf(path, size, "%s:%s/terminfo", dirs, bundle);
	setenv("TERMINFO_DIRS", path, 1);
	free(path);
}

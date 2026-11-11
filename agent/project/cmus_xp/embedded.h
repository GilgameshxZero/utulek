/* Embedded Linux plugins and data. GPL-2.0-or-later. */
#ifndef CMUS_EMBEDDED_H
#define CMUS_EMBEDDED_H
#include <stddef.h>
#include "windows/static_plugins.h"
#include "op.h"
#include "mixer.h"

struct cmus_static_output {
	const char *name;
	const int *priority;
	const unsigned *abi_version;
	const struct output_plugin_ops *pcm_ops;
	const struct output_plugin_opt *pcm_options;
	const struct mixer_plugin_ops *mixer_ops;
	const struct mixer_plugin_opt *mixer_options;
};
extern const struct cmus_static_output cmus_static_outputs[];
extern const char *const cmus_embedded_themes[];
int cmus_resource(const char *name, const char **data, size_t *size);
int cmus_resource_exists(const char *name);
void cmus_bundle_init(void);
#endif

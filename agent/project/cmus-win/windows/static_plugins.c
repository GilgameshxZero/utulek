/* Statically linked input plugin registry. GPL-2.0-or-later. */
#include "static_plugins.h"

/* The build namespaces each plugin's public symbols so they can share an EXE. */
#define DECLARE_INPUT(name) \
	extern const int name##_ip_priority; \
	extern const struct input_plugin_ops name##_ip_ops; \
	extern const struct input_plugin_opt name##_ip_options[]; \
	extern const char * const name##_ip_extensions[]; \
	extern const char * const name##_ip_mime_types[]

#define INPUT(name) { #name, &name##_ip_priority, &name##_ip_ops, \
	name##_ip_options, name##_ip_extensions, name##_ip_mime_types }

DECLARE_INPUT(wav);
DECLARE_INPUT(ffmpeg);

const struct cmus_static_input cmus_static_inputs[] = {
	INPUT(wav),
	INPUT(ffmpeg),
	{ NULL }
};

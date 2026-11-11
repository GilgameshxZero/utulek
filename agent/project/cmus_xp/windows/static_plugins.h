/* Statically linked input plugin registry. GPL-2.0-or-later. */
#ifndef CMUS_STATIC_PLUGINS_H
#define CMUS_STATIC_PLUGINS_H

#include "../ip.h"

struct cmus_static_input {
	const char *name;
	const int *priority;
	const struct input_plugin_ops *ops;
	const struct input_plugin_opt *options;
	const char * const *extensions;
	const char * const *mime_types;
};

extern const struct cmus_static_input cmus_static_inputs[];

#endif

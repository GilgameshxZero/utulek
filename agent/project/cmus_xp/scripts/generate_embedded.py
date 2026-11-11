#!/usr/bin/env python3
"""Generate Linux plugin registries and embedded defaults in the build tree."""
import argparse
import json
from pathlib import Path


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--inputs', nargs='*', default=[])
    parser.add_argument('--outputs', nargs='*', default=[])
    args = parser.parse_args()
    registry = ['#include "embedded.h"']
    for kind, names, symbols in (
        ('ip', args.inputs, [('int', 'priority'), ('struct input_plugin_ops', 'ops'),
                            ('struct input_plugin_opt', 'options[]'),
                            ('char *const', 'extensions[]'), ('char *const', 'mime_types[]')]),
        ('op', args.outputs, [('int', 'priority'), ('unsigned', 'abi_version'),
                             ('struct output_plugin_ops', 'pcm_ops'),
                             ('struct output_plugin_opt', 'pcm_options[]')]),
    ):
        for name in names:
            for ctype, symbol in symbols:
                registry.append(f'extern const {ctype} {name}_{kind}_{symbol};')
    registry.append('const struct cmus_static_input cmus_static_inputs[] = {')
    for name in args.inputs:
        registry.append('{"%s", &%s_ip_priority, &%s_ip_ops, %s_ip_options, '
                        '%s_ip_extensions, %s_ip_mime_types},' % ((name,) * 6))
    registry.append('{NULL}};')
    entries = []
    for name in args.outputs:
        has_mixer = any('const struct mixer_plugin_ops op_mixer_ops' in p.read_text()
                        for p in (Path('op') / (name + '.c'),
                                  Path('op') / ('mixer_' + name + '.c')) if p.exists())
        mixer = 'NULL, NULL'
        if has_mixer:
            registry += [f'extern const struct mixer_plugin_ops {name}_op_mixer_ops;',
                         f'extern const struct mixer_plugin_opt {name}_op_mixer_options[];']
            mixer = f'&{name}_op_mixer_ops, {name}_op_mixer_options'
        entries.append('{"%s", &%s_op_priority, &%s_op_abi_version, &%s_op_pcm_ops, '
                       '%s_op_pcm_options, %s},' % ((name,) * 5 + (mixer,)))
    registry += ['const struct cmus_static_output cmus_static_outputs[] = {',
                 *entries, '{NULL}};']
    Path('embedded_plugins.c').write_text('\n'.join(registry) + '\n')
    resources = ['#include "embedded.h"', '#include <string.h>']
    entries, themes = [], []
    for i, path in enumerate(sorted(Path('data').iterdir())):
        if not path.is_file():
            continue
        raw = path.read_bytes()
        resources.append('static const char data_%d[] = {%s,0};' %
                         (i, ','.join(map(str, raw))))
        entries.append('{%s, data_%d, %d},' %
                       (json.dumps(':cmus-data:/' + path.name), i, len(raw)))
        if path.suffix == '.theme':
            themes.append(json.dumps(path.stem))
    resources += ['const char *const cmus_embedded_themes[] = {' +
                  ','.join(themes + ['NULL']) + '};',
                  'static const struct { const char *name, *data; size_t size; } resources[] = {',
                  *entries, '};', '''
int cmus_resource(const char *name, const char **data, size_t *size)
{
    for (size_t i = 0; i < sizeof(resources) / sizeof(resources[0]); i++)
        if (!strcmp(name, resources[i].name)) {
            *data = resources[i].data;
            *size = resources[i].size;
            return 1;
        }
    return 0;
}
int cmus_resource_exists(const char *name)
{
    const char *data;
    size_t size;
    return cmus_resource(name, &data, &size);
}
''']
    Path('embedded_data.c').write_text('\n'.join(resources))


if __name__ == '__main__':
    main()

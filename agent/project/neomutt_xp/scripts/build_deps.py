#!/usr/bin/env python3
"""Build static native Windows dependencies, retaining logs under .obj/."""
import argparse
from pathlib import Path

from deps import (MANIFEST, ROOT, Toolchain, jobs, prefix, run, source,
                  source_identity, stamp_matches, write_stamp)

LIBRARIES = {
    'ncurses': {
        'outputs': ['lib/libncursesw.a'],
        'options': ['--enable-widec', '--enable-term-driver', '--enable-sp-funcs',
                    '--without-shared', '--without-debug', '--without-ada',
                    '--without-cxx', '--without-cxx-binding', '--without-tests',
                    '--without-progs', '--disable-db-install', '--with-build-cc=cc',
                    '--with-fallbacks=xterm-256color',
                    '--with-build-cflags=-include wchar.h'],
        'make_args': ['BUILD_CCFLAGS=-include wchar.h'],
    },
    'iconv': {
        'outputs': ['lib/libiconv.a'],
        'options': ['--disable-nls'],
        'make_args': [],
    },
    'pcre2': {
        'outputs': ['lib/libpcre2-8.a', 'lib/libpcre2-posix.a'],
        'options': ['--enable-unicode', '--disable-pcre2grep-libz',
                    '--disable-pcre2grep-libbz2'],
        'make_args': [],
    },
}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('library', choices=sorted(LIBRARIES))
    parser.add_argument('--toolchain', type=Path, required=True)
    parser.add_argument('-j', '--jobs', type=int)
    args = parser.parse_args()

    spec = LIBRARIES[args.library]
    manifest_name = 'libiconv' if args.library == 'iconv' else args.library
    item = MANIFEST[manifest_name]
    toolchain = Toolchain(args.toolchain)
    build = ROOT / ('.obj/build-' + args.library)
    target = prefix('windows')
    options = ['--host=x86_64-w64-mingw32', '--prefix=' + str(target),
               '--disable-shared', '--enable-static', *spec['options']]
    snapshot = {'library': args.library, 'version': item['version'],
                'toolchain': str(toolchain.path), 'options': options,
                'source': source_identity(manifest_name)}
    outputs = [target / name for name in spec['outputs']]
    if stamp_matches('deps-' + args.library, snapshot) and all(path.exists() for path in outputs):
        print(f'{args.library}: up to date')
        return

    environment = toolchain.env(CFLAGS='-O2')
    run([source(manifest_name) / 'configure', *options], build, 'configure.log', environment)
    run(['make', '-j' + str(jobs(args.jobs)), *spec['make_args']], build, 'build.log', environment)
    run(['make', 'install'], build, 'install.log', environment)
    write_stamp('deps-' + args.library, snapshot)
    print(f'{args.library}: installed into {target}')


if __name__ == '__main__':
    main()

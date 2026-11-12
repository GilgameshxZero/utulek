#!/usr/bin/env python3
"""Build the upstream application on Linux or native Windows (cross compilation)."""
import argparse
import os
from pathlib import Path
import shutil
import subprocess
import sys

from deps import (ROOT, Toolchain, jobs, libraries, prefix, run, source,
                  stamp_matches, tree_fingerprint, write_stamp)


def native_objects(toolchain, build):
    headers = list((ROOT / 'native').rglob('*.h'))
    latest_header = max(path.stat().st_mtime for path in headers)
    objects = []
    for source_file in sorted((ROOT / 'native').glob('*.c')):
        obj = build / ('xp-' + source_file.stem + '.o')
        if not obj.exists() or obj.stat().st_mtime < max(source_file.stat().st_mtime,
                                                         latest_header):
            run([toolchain.clang, '-std=c11', '-O2', '-Wall', '-Wextra', '-Werror',
                 '-D_FILE_OFFSET_BITS=64', '-I' + str(ROOT / 'native'),
                 '-I' + str(ROOT / 'native/include'),
                 '-I' + str(prefix('windows') / 'include'),
                 '-c', source_file, '-o', obj],
                build, 'xp-' + source_file.stem + '.log')
        objects.append(obj)
    resource = build / 'xp-manifest.o'
    inputs = [ROOT / 'native/neomutt.rc', ROOT / 'native/neomutt.manifest',
              ROOT / 'runtime/mime.types', ROOT / 'runtime/neomutt-system.rc']
    if not resource.exists() or resource.stat().st_mtime < max(
            path.stat().st_mtime for path in inputs):
        run([toolchain.tool('x86_64-w64-mingw32-windres'), '-I', ROOT / 'native',
             '-I', ROOT / 'runtime', ROOT / 'native/neomutt.rc', '-o', resource],
            build, 'xp-manifest.log')
    return objects + [resource]


def configure_linux(build):
    command = [source('neomutt') / 'configure', '--autocrypt',
               '--with-sqlite=' + str(prefix('linux')), '--disable-doc',
               '--disable-nls', '--disable-idn2', '--ssl', '--lmdb',
               '--with-lmdb=' + str(prefix('linux'))]
    environment = dict(os.environ,
                       PKG_CONFIG_LIBDIR=str(prefix('linux') / 'lib/pkgconfig'),
                       CPPFLAGS='-I' + str(prefix('linux') / 'include'),
                       LDFLAGS='-L' + str(prefix('linux') / 'lib'),
                       LIBS=('-lgpgme -lassuan -lgpg-error -lsqlite3 '
                             '-lpthread -ldl'))
    snapshot = {'target': 'linux', 'command': [str(item) for item in command],
                'environment': {key: environment[key] for key in
                                ('CPPFLAGS', 'LDFLAGS', 'LIBS', 'PKG_CONFIG_LIBDIR')},
                'source': tree_fingerprint(source('neomutt'))}
    if not (stamp_matches('configure-linux', snapshot)
            and (build / 'config.status').exists()):
        run(command, build, 'configure.log', environment)
        write_stamp('configure-linux', snapshot)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('target', choices=['linux', 'windows'])
    parser.add_argument('--toolchain', type=Path)
    parser.add_argument('-j', '--jobs', type=int)
    args = parser.parse_args()
    build = ROOT / '.obj' / args.target
    build.mkdir(parents=True, exist_ok=True)
    make_jobs = str(jobs(args.jobs))

    if args.target == 'windows':
        if not args.toolchain:
            parser.error('--toolchain is required for Windows')
        toolchain = Toolchain(args.toolchain)
        subprocess.run([sys.executable, ROOT / 'scripts/configure_windows.py',
                        '--toolchain', toolchain.path], check=True)
        objects = native_objects(toolchain, build)
        dependencies = objects + [prefix('windows') / 'lib' / f'lib{name}.a'
                                  for name in libraries('windows')]
        extra = ['XP_PLATFORM_OBJECTS=' + ' '.join(map(str, dependencies)),
                 'EXTRA_CFLAGS=-I' + str(ROOT / 'native') + ' -include ' +
                 str(ROOT / 'native/compat.h'),
                 'EXTRA_LDFLAGS=' + ' '.join(map(str, objects)) +
                 ' -ladvapi32 -lbcrypt -lws2_32']
        executable = 'neomutt.exe'
    else:
        configure_linux(build)
        extra = ['XP_PLATFORM_OBJECTS=' + ' '.join(
            str(prefix('linux') / 'lib' / f'lib{name}.a') for name in libraries('linux'))]
        executable = 'neomutt'

    run(['make', '-j' + make_jobs, executable, *extra], build, 'build.log')
    destination = ROOT / '.bin' / args.target
    destination.mkdir(parents=True, exist_ok=True)
    shutil.copy2(build / executable, destination / executable)
    if args.target == 'windows':
        legacy = destination / 'neomutt'
        if legacy.exists():
            shutil.rmtree(legacy)
    print(destination / executable)


if __name__ == '__main__':
    main()

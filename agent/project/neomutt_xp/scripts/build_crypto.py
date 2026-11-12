#!/usr/bin/env python3
"""Build pinned GPGME/Autocrypt dependencies for Linux and native Windows."""
import argparse
import os
from pathlib import Path
import shutil

from deps import (MANIFEST, ROOT, Toolchain, jobs, prefix, run, source,
                  source_identity, stamp_matches, write_stamp)

CRYPTO = ('libgpg-error', 'libassuan', 'gpgme')
ARCHIVES = {'libgpg-error': 'libgpg-error.a', 'libassuan': 'libassuan.a',
            'gpgme': 'libgpgme.a'}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('target', choices=['linux', 'windows'])
    parser.add_argument('--toolchain', type=Path)
    parser.add_argument('-j', '--jobs', type=int)
    args = parser.parse_args()
    windows = args.target == 'windows'
    if windows and not args.toolchain:
        parser.error('--toolchain is required for Windows')

    toolchain = Toolchain(args.toolchain) if args.toolchain else None
    target = prefix(args.target)
    target.mkdir(parents=True, exist_ok=True)
    prefix_flags = {'CPPFLAGS': '-I' + str(target / 'include'),
                    'LDFLAGS': '-L' + str(target / 'lib'),
                    'PKG_CONFIG_LIBDIR': str(target / 'lib/pkgconfig'),
                    'CFLAGS': '-O2'}
    environment = toolchain.env(**prefix_flags) if toolchain else dict(
        os.environ, CC='cc', AR='ar', RANLIB='ranlib',
        CC_FOR_BUILD='cc', **prefix_flags)
    compiler = str(toolchain.clang) if toolchain else 'cc'
    archiver = str(toolchain.tool('llvm-ar')) if toolchain else 'ar'
    make_jobs = str(jobs(args.jobs))

    for name in CRYPTO:
        item = MANIFEST[name]
        source_dir = source(name)
        build = ROOT / f'.obj/build-{name}-{args.target}'
        outputs = [target / 'lib' / ARCHIVES[name]]
        options = ['--prefix=' + str(target), '--disable-shared', '--enable-static',
                   '--disable-nls', '--disable-doc', '--disable-tests']
        if windows:
            options.append('--host=x86_64-w64-mingw32')
        if name == 'libassuan':
            options.append('--with-libgpg-error-prefix=' + str(target))
        if name == 'gpgme':
            options += ['--with-libgpg-error-prefix=' + str(target),
                        '--with-libassuan-prefix=' + str(target),
                        '--disable-gpg-test', '--disable-gpgsm-test',
                        '--disable-gpgconf-test', '--disable-g13-test']
        snapshot = {'name': name, 'version': item['version'], 'target': args.target,
                    'compiler': compiler, 'options': options,
                    'source': source_identity(name)}
        if stamp_matches(f'{name}-{args.target}', snapshot) and all(
                path.exists() for path in outputs):
            print(f'{name} {args.target}: up to date')
            continue
        run([source_dir / 'configure', *options], build, 'configure.log', environment)
        run(['make', '-j' + make_jobs], build, 'build.log', environment)
        install = (['make', '-C', 'src', 'install'] if name == 'libgpg-error'
                   else ['make', '-j' + make_jobs, 'install'])
        run(install, build, 'install.log', environment)
        write_stamp(f'{name}-{args.target}', snapshot)
        print(name, args.target, 'built', flush=True)

    source_dir = source('sqlite')
    build = ROOT / f'.obj/build-sqlite-{args.target}'
    library = target / 'lib/libsqlite3.a'
    flags = ['-O2', '-DSQLITE_THREADSAFE=1', '-DSQLITE_OMIT_LOAD_EXTENSION']
    snapshot = {'version': MANIFEST['sqlite']['version'], 'target': args.target,
                'compiler': compiler, 'flags': flags,
                'source': source_identity('sqlite')}
    for sub in ('include', 'lib/pkgconfig'):
        (target / sub).mkdir(parents=True, exist_ok=True)
    if not (stamp_matches('sqlite-' + args.target, snapshot) and library.exists()):
        run([compiler, *flags, '-c', source_dir / 'sqlite3.c', '-o', build / 'sqlite3.o'],
            build, 'build.log')
        run([archiver, 'rcs', library, build / 'sqlite3.o'], build, 'archive.log')
        write_stamp('sqlite-' + args.target, snapshot)
        print('sqlite', args.target, 'built')
    shutil.copy2(source_dir / 'sqlite3.h', target / 'include/sqlite3.h')
    (target / 'lib/pkgconfig/sqlite3.pc').write_text(
        f'prefix={target}\nlibdir=${{prefix}}/lib\nincludedir=${{prefix}}/include\n'
        f'Name: SQLite\nDescription: SQLite\nVersion: {MANIFEST["sqlite"]["version"]}\n'
        'Libs: -L${libdir} -lsqlite3\nCflags: -I${includedir}\n')


if __name__ == '__main__':
    main()

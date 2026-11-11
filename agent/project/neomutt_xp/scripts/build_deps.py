#!/usr/bin/env python3
"""Build static native Windows dependencies, retaining logs under .obj/."""
import argparse
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[1]

def run(command, cwd, env, log):
    with (cwd / log).open('w') as output:
        subprocess.run(list(map(str, command)), cwd=cwd, env=env,
                       stdout=output, stderr=subprocess.STDOUT, check=True)

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('library', choices=['ncurses', 'iconv', 'pcre2'])
    parser.add_argument('--toolchain', type=Path, required=True)
    args = parser.parse_args()
    tools = args.toolchain.resolve() / 'bin'
    env = dict(os.environ, CC=str(tools / 'x86_64-w64-mingw32-clang'),
               AR=str(tools / 'llvm-ar'), RANLIB=str(tools / 'llvm-ranlib'),
               STRIP=str(tools / 'llvm-strip'), WINDRES=str(tools / 'x86_64-w64-mingw32-windres'), RC=str(tools / 'x86_64-w64-mingw32-windres'), CFLAGS='-O2')
    prefix = ROOT / '.obj/windows-prefix'
    options = ['--host=x86_64-w64-mingw32', '--prefix=' + str(prefix),
               '--disable-shared', '--enable-static']
    if args.library == 'ncurses':
        source = ROOT / '.obj/deps/ncurses-6.5'
        options += ['--enable-widec', '--enable-term-driver', '--enable-sp-funcs',
                    '--without-shared', '--without-debug', '--without-ada',
                    '--without-cxx', '--without-cxx-binding', '--without-tests',
                    '--without-progs', '--disable-db-install', '--with-build-cc=cc',
                    '--with-fallbacks=xterm-256color', '--with-build-cflags=-include wchar.h']
    elif args.library == 'iconv':
        source = ROOT / '.obj/deps/libiconv-1.19'
        options += ['--disable-nls']
    else:
        source = ROOT / '.obj/deps/pcre2-10.46'
        options += ['--enable-unicode', '--disable-pcre2grep-libz', '--disable-pcre2grep-libbz2']
    build = ROOT / ('.obj/build-' + args.library)
    build.mkdir(parents=True, exist_ok=True)
    run([source / 'configure', *options], build, env, 'configure.log')
    make_args = ['BUILD_CCFLAGS=-include wchar.h'] if args.library == 'ncurses' else []
    run(['make', '-j4', *make_args], build, env, 'build.log')
    run(['make', 'install'], build, env, 'install.log')

if __name__ == '__main__':
    main()

#!/usr/bin/env python3
"""Configure upstream NeoMutt for the native Windows build spike."""
import argparse
import os
from pathlib import Path
import subprocess
ROOT = Path(__file__).resolve().parents[1]
p = argparse.ArgumentParser()
p.add_argument('--toolchain', type=Path, required=True)
a = p.parse_args()
tool = a.toolchain.resolve() / 'bin'
prefix = ROOT / '.obj/windows-prefix'
build = ROOT / '.obj/windows'
build.mkdir(exist_ok=True)
env = dict(os.environ, CC=str(tool / 'x86_64-w64-mingw32-clang'),
           CC_FOR_BUILD='cc', AR=str(tool / 'llvm-ar'),
           RANLIB=str(tool / 'llvm-ranlib'),
           CFLAGS=f'-O2 -D_FILE_OFFSET_BITS=64 -DNCURSES_STATIC -DPCRE2_STATIC -I{ROOT / "native"} -I{ROOT / "native/include"} -I{prefix / "include"}',
           LDFLAGS=f'-L{prefix / "lib"}',
           LIBS='-lpcre2-posix -lpcre2-8 -lgpgme -lassuan -lgpg-error -lsqlite3 -lws2_32 -lcrypt32 -lbcrypt',
           PKG_CONFIG_LIBDIR=str(prefix / 'lib/pkgconfig'), PKG_CONFIG_SYSROOT_DIR='/')
with (build / 'configure.log').open('w') as out:
 subprocess.run([ROOT / '.obj/upstream/configure', '--host=x86_64-w64-mingw32', '--sysroot=/',
                 '--disable-doc', '--disable-nls', '--disable-idn2', '--disable-inotify',
                 '--autocrypt', '--with-sqlite=' + str(prefix), '--lmdb', '--with-lmdb=' + str(prefix), '--ssl', '--with-ssl=' + str(prefix), '--pcre2', '--with-ncurses=' + str(prefix), '--with-iconv=' + str(prefix)],
                cwd=build, env=env, stdout=out, stderr=subprocess.STDOUT, check=True)

#!/usr/bin/env python3
"""Configure upstream NeoMutt for the native Windows build."""
import argparse
from pathlib import Path

from deps import (ROOT, Toolchain, prefix, run, source, stamp_matches,
                  tree_fingerprint, write_stamp)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--toolchain', type=Path, required=True)
    args = parser.parse_args()
    toolchain = Toolchain(args.toolchain)
    target = prefix('windows')
    build = ROOT / '.obj/windows'
    command = [source('neomutt') / 'configure', '--host=x86_64-w64-mingw32',
               '--sysroot=/', '--disable-doc', '--disable-nls', '--disable-idn2',
               '--disable-inotify', '--autocrypt',
               '--with-sqlite=' + str(target), '--lmdb',
               '--with-lmdb=' + str(target), '--ssl',
               '--with-ssl=' + str(target), '--pcre2',
               '--with-ncurses=' + str(target), '--with-iconv=' + str(target)]
    environment = toolchain.env(
        CC_FOR_BUILD='cc',
        CFLAGS=(f'-O2 -D_FILE_OFFSET_BITS=64 -DNCURSES_STATIC -DPCRE2_STATIC '
                f'-I{ROOT / "native"} -I{ROOT / "native/include"} -I{target / "include"}'),
        LDFLAGS=f'-L{target / "lib"}',
        LIBS=('-lpcre2-posix -lpcre2-8 -lgpgme -lassuan -lgpg-error -lsqlite3 '
              '-lws2_32 -lcrypt32 -lbcrypt'),
        PKG_CONFIG_LIBDIR=str(target / 'lib/pkgconfig'), PKG_CONFIG_SYSROOT_DIR='/')
    snapshot = {'command': [str(item) for item in command],
                'environment': {key: environment[key] for key in
                                ('CC', 'CFLAGS', 'LDFLAGS', 'LIBS', 'PKG_CONFIG_LIBDIR')},
                'source': tree_fingerprint(source('neomutt'))}
    if stamp_matches('configure-windows', snapshot) and (build / 'Makefile').exists():
        print('configure windows: up to date')
        return
    run(command, build, 'configure.log', environment)
    write_stamp('configure-windows', snapshot)
    print('configure windows: configured')


if __name__ == '__main__':
    main()

#!/usr/bin/env python3
"""Build supplied curl source as a private static HTTP(S)-only library.

Windows uses Schannel and the native certificate store; Linux uses OpenSSL.
No downloads or system installation. Keep the source archive for provenance.
"""
import argparse
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parent.parent
VERSION = '8.23.0'
SOURCE_SHA256 = '8dadd7d2536a314f982ad05a651b713f426bf7d02cc2c76cffb9796f2218e6c5'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', type=Path, default=ROOT / ('.obj/deps/curl-' + VERSION))
    parser.add_argument('--host', action='store_true')
    parser.add_argument('--jobs', type=int, default=4)
    args = parser.parse_args()
    if args.jobs < 1:
        parser.error('--jobs must be positive')
    platform = 'host' if args.host else 'windows'
    build = ROOT / ('.obj/curl_' + platform)
    prefix = ROOT / ('.obj/deps/http_' + platform)
    build.mkdir(parents=True, exist_ok=True)
    source = args.source.resolve()
    if not (source / 'configure').is_file():
        parser.error('Missing supplied curl source: ' + str(source))
    env = dict(os.environ, LC_ALL='C')
    command = [str(source / 'configure'), '--prefix=' + str(prefix),
               '--disable-shared', '--enable-static', '--enable-http',
               '--disable-manual', '--disable-docs', '--disable-ipfs',
               '--disable-libcurl-option', '--enable-threaded-resolver']
    for protocol in ('ftp', 'file', 'ldap', 'ldaps', 'rtsp', 'dict', 'telnet', 'tftp',
                     'pop3', 'imap', 'smtp', 'gopher', 'mqtt', 'websockets'):
        command.append('--disable-' + protocol)
    for dependency in ('libidn2', 'libpsl', 'libssh2', 'libssh',
                       'nghttp2', 'nghttp3', 'zlib', 'brotli', 'zstd'):
        command.append('--without-' + dependency)
    if args.host:
        command += ['--with-openssl', '--with-ca-bundle=/etc/ssl/certs/ca-certificates.crt']
    else:
        tc = ROOT / '.obj/deps/llvm_mingw/bin'
        env.update(CC=str(tc / 'x86_64-w64-mingw32-clang'), AR=str(tc / 'llvm-ar'),
                   RANLIB=str(tc / 'llvm-ranlib'), NM=str(tc / 'llvm-nm'),
                   RC=str(tc / 'x86_64-w64-mingw32-windres'),
                   PATH=str(tc) + os.pathsep + env.get('PATH', ''))
        command += ['--host=x86_64-w64-mingw32', '--with-schannel']
    for cmd in (command, ['make', '-C', 'lib', '-j' + str(args.jobs)],
                ['make', '-C', 'include', 'install'], ['make', '-C', 'lib', 'install']):
        subprocess.run(cmd, cwd=build, env=env, check=True)
    print(prefix)


if __name__ == '__main__':
    main()

#!/usr/bin/env python3
"""Build the pinned static OpenSSL libraries for native Windows."""
import argparse
from pathlib import Path

from deps import (MANIFEST, ROOT, Toolchain, jobs, prefix, run, source,
                  source_identity, stamp_matches, write_stamp)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--toolchain', type=Path, required=True)
    parser.add_argument('-j', '--jobs', type=int)
    args = parser.parse_args()
    toolchain = Toolchain(args.toolchain)
    source_dir = source('openssl')
    build = ROOT / '.obj/build-openssl'
    target = prefix('windows')
    commands = [
        ('configure', ['perl', source_dir / 'Configure', 'mingw64', 'no-shared',
                       'no-tests', 'no-module', 'no-apps', 'no-docs',
                       '--prefix=' + str(target), '--libdir=lib']),
        ('build', ['make', '-j' + str(jobs(args.jobs)), 'build_libs']),
        ('install', ['make', 'install_dev']),
    ]
    outputs = [target / 'lib/libssl.a', target / 'lib/libcrypto.a']
    snapshot = {'version': MANIFEST['openssl']['version'],
                'toolchain': str(toolchain.path),
                'commands': [[str(item) for item in command] for _, command in commands],
                'source': source_identity('openssl')}
    if stamp_matches('openssl-windows', snapshot) and all(path.exists() for path in outputs):
        print(f'openssl: up to date, {target}')
        return
    environment = toolchain.env()
    for label, command in commands:
        run(command, build, label + '.log', environment)
    write_stamp('openssl-windows', snapshot)
    print(target)


if __name__ == '__main__':
    main()

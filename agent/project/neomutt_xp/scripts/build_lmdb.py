#!/usr/bin/env python3
"""Build the same pinned LMDB cache backend for Linux and native Windows."""
import argparse
from pathlib import Path

from deps import (MANIFEST, ROOT, Toolchain, jobs, prefix, run, source,
                  source_identity, stamp_matches, write_stamp)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('target', choices=['linux', 'windows'])
    parser.add_argument('--toolchain', type=Path)
    args = parser.parse_args()
    if args.target == 'windows' and not args.toolchain:
        parser.error('--toolchain is required for Windows')
    toolchain = Toolchain(args.toolchain) if args.toolchain else None
    compiler = str(toolchain.clang) if toolchain else 'cc'
    archiver = str(toolchain.tool('llvm-ar')) if toolchain else 'ar'
    build = ROOT / ('.obj/build-lmdb-' + args.target)
    target = prefix(args.target)
    (target / 'include').mkdir(parents=True, exist_ok=True)
    (target / 'lib').mkdir(parents=True, exist_ok=True)
    library = target / 'lib/liblmdb.a'
    source_dir = source('lmdb') / 'libraries/liblmdb'
    flags = ['-std=c11', '-D_GNU_SOURCE', '-O2']
    snapshot = {'version': MANIFEST['lmdb']['version'], 'compiler': compiler,
                'flags': flags, 'source': source_identity('lmdb')}
    if stamp_matches('lmdb-' + args.target, snapshot) and library.exists():
        print(library)
        return
    objects = []
    for name in ('mdb', 'midl'):
        obj = build / (name + '.o')
        objects.append(obj)
        run([compiler, *flags, '-c', source_dir / (name + '.c'), '-o', obj],
            build, 'build.log')
    run([archiver, 'rcs', library, *objects], build, 'archive.log')
    (target / 'include/lmdb.h').write_bytes((source_dir / 'lmdb.h').read_bytes())
    write_stamp('lmdb-' + args.target, snapshot)
    print(library)


if __name__ == '__main__':
    main()

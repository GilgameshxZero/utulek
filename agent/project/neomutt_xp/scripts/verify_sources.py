#!/usr/bin/env python3
"""Reconstruct pinned upstream sources and verify all archive files against work."""
import argparse
import json
from pathlib import Path
import shutil
import subprocess
import tarfile

from deps import CHUNK, MANIFEST, ROOT, sha256_file, source, verify_archive

VERIFICATION = ROOT / '.obj/source-verification'
STAMPS = VERIFICATION / '.stamps.json'


def patches_for(name):
    return sorted((ROOT / 'patches').glob(name + '-*.patch'))


def extract(name, item, archive, target):
    if target.exists():
        shutil.rmtree(target)
    target.mkdir(parents=True)
    originals = []
    with tarfile.open(archive) as tar:
        for member in tar.getmembers():
            member.name = '/'.join(member.name.split('/')[1:])
            if not member.name:
                continue
            if member.isfile():
                originals.append(member.name)
            tar.extract(member, target, filter='data')
    for patch in patches_for(name):
        subprocess.run(['patch', '--batch', '-p1', '-i', str(patch)], cwd=target,
                       check=True, stdout=subprocess.DEVNULL)
    return originals


def differs(left, right):
    if left.stat().st_size != right.stat().st_size:
        return True
    with left.open('rb') as one, right.open('rb') as two:
        while True:
            a, b = one.read(CHUNK), two.read(CHUNK)
            if a != b:
                return True
            if not a:
                return False


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--refresh', action='store_true',
                        help='re-extract reference trees instead of reusing them')
    args = parser.parse_args()
    VERIFICATION.mkdir(parents=True, exist_ok=True)
    stamps = json.loads(STAMPS.read_text()) if STAMPS.exists() else {}
    results = []
    for name in MANIFEST['source_verification']:
        item = MANIFEST[name]
        archive = ROOT / '.obj/downloads' / item['archive']
        verify_archive(name, archive)
        target = VERIFICATION / name
        stamp = {'archive': item['sha256'],
                 'patches': [[patch.name, sha256_file(patch)]
                             for patch in patches_for(name)]}
        if args.refresh or stamps.get(name) != stamp or not target.exists():
            originals = extract(name, item, archive, target)
            stamps[name] = stamp
        else:
            with tarfile.open(archive) as tar:
                originals = ['/'.join(member.name.split('/')[1:])
                             for member in tar.getmembers()
                             if member.isfile() and member.name.split('/')[1:]]
        work = source(name)
        differences = [relative for relative in originals
                       if differs(work / relative, target / relative)]
        results.append({'source': name, 'files': len(originals),
                        'patches': [patch.name for patch in patches_for(name)],
                        'passed': not differences, 'differences': differences})
    STAMPS.write_text(json.dumps(stamps, indent=2, sort_keys=True) + '\n')
    (VERIFICATION / 'results.json').write_text(json.dumps(results, indent=2) + '\n')
    print(json.dumps(results, indent=2))
    raise SystemExit(0 if all(row['passed'] for row in results) else 1)


if __name__ == '__main__':
    main()

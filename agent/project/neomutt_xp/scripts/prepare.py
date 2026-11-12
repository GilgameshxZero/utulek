#!/usr/bin/env python3
"""Fetch pinned source archives, verify hashes, and apply tracked port patches."""
import argparse
import json
from pathlib import Path
import shutil
import subprocess
import tarfile
import urllib.request

from deps import MANIFEST, ROOT, sha256_file, verify_archive

STAMPS = ROOT / '.obj/source-stamps'


def patch_stamp(name):
    patches = sorted((ROOT / 'patches').glob(name + '-*.patch'))
    return {patch.name: sha256_file(patch) for patch in patches}


def extract(name, item, archive, destination):
    partial = destination.with_name(destination.name + '.partial')
    if partial.exists():
        shutil.rmtree(partial)
    partial.mkdir(parents=True)
    with tarfile.open(archive) as source:
        for member in source.getmembers():
            member.name = '/'.join(member.name.split('/')[1:])
            if member.name:
                source.extract(member, partial, filter='data')
    for patch in sorted((ROOT / 'patches').glob(name + '-*.patch')):
        subprocess.run(['patch', '--batch', '-p1', '-i', str(patch)],
                       cwd=partial, check=True)
    destination.parent.mkdir(parents=True, exist_ok=True)
    partial.rename(destination)
    return patch_stamp(name)


def download(name, item, archive):
    if archive.exists():
        return
    archive.parent.mkdir(parents=True, exist_ok=True)
    temporary = archive.with_suffix('.partial')
    with urllib.request.urlopen(item['url'], timeout=60) as response, temporary.open('wb') as output:
        while chunk := response.read(1024 * 1024):
            output.write(chunk)
    temporary.replace(archive)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('names', nargs='*')
    parser.add_argument('--platform', choices=sorted(MANIFEST['platforms']),
                        help='prepare the dependencies for one build target')
    parser.add_argument('--refresh', action='store_true',
                        help='discard an existing source tree and re-extract it')
    args = parser.parse_args()
    names = args.names or (MANIFEST['platforms'][args.platform] if args.platform
                           else list(MANIFEST['platforms']['windows']))
    for name in names:
        item = MANIFEST[name]
        archive = ROOT / '.obj/downloads' / item['archive']
        download(name, item, archive)
        verify_archive(name, archive)
        destination = ROOT / ('.obj/upstream' if name == 'neomutt'
                              else f'.obj/deps/{name}-{item["version"]}')
        stamp_path = STAMPS / (name + '.json')
        stamp = patch_stamp(name) if destination.exists() else {}
        if destination.exists() and not args.refresh:
            previous = json.loads(stamp_path.read_text()) if stamp_path.exists() else None
            if previous is not None and previous != stamp:
                raise SystemExit(
                    f'{name}: patches changed since the source tree was prepared; '
                    f're-run with --refresh')
            STAMPS.mkdir(parents=True, exist_ok=True)
            stamp_path.write_text(json.dumps(stamp, indent=2, sort_keys=True) + '\n')
            print(f'{name}: archive verified; existing source preserved')
            continue
        if destination.exists():
            shutil.rmtree(destination)
        stamp = extract(name, item, archive, destination)
        STAMPS.mkdir(parents=True, exist_ok=True)
        stamp_path.write_text(json.dumps(stamp, indent=2, sort_keys=True) + '\n')
        print(f'{name}: prepared {destination}')


if __name__ == '__main__':
    main()

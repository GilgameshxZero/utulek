#!/usr/bin/env python3
"""Fetch pinned source archives, verify hashes, and apply tracked port patches."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import tarfile
import urllib.request
ROOT = Path(__file__).resolve().parents[1]

def main():
    p = argparse.ArgumentParser()
    p.add_argument('names', nargs='*', default=['neomutt', 'ncurses', 'libiconv', 'pcre2', 'openssl', 'lmdb'])
    args = p.parse_args()
    manifest = json.loads((ROOT / 'dependencies.json').read_text())
    for name in args.names:
        item = manifest[name]
        archive = ROOT / '.obj/downloads' / item['archive']
        archive.parent.mkdir(parents=True, exist_ok=True)
        if not archive.exists():
            temporary = archive.with_suffix('.partial')
            with urllib.request.urlopen(item['url'], timeout=60) as response, temporary.open('wb') as output:
                while chunk := response.read(1024 * 1024):
                    output.write(chunk)
            temporary.replace(archive)
        actual = hashlib.sha256(archive.read_bytes()).hexdigest()
        if actual != item['sha256']:
            raise SystemExit(f'{archive}: SHA256 mismatch: {actual}')
        destination = ROOT / ('.obj/upstream' if name == 'neomutt' else f'.obj/deps/{name}-{item["version"]}')
        if destination.exists():
            print(f'{name}: archive verified; existing source preserved')
            continue
        destination.mkdir(parents=True)
        with tarfile.open(archive) as source:
            for member in source.getmembers():
                member.name = '/'.join(member.name.split('/')[1:])
                if member.name:
                    source.extract(member, destination, filter='data')
        for patch in sorted((ROOT / 'patches').glob(name + '-*.patch')):
            subprocess.run(['patch', '-p1', '-i', str(patch)], cwd=destination, check=True)
        print(f'{name}: prepared {destination}')

if __name__ == '__main__':
    main()

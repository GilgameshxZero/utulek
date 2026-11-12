#!/usr/bin/env python3
"""Save a reviewable source/binary/evidence checkpoint without Git commits."""
import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile

from deps import ROOT, sha256_file

BLOBS = ROOT / '.obj/blobs'


def store(source_file, destination):
    """Keep one immutable blob per content hash and link it into the checkpoint."""
    digest = sha256_file(source_file)
    blob = BLOBS / digest[:2] / digest
    if not blob.exists():
        blob.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(source_file, blob)
        blob.chmod(0o444)
    destination.parent.mkdir(parents=True, exist_ok=True)
    try:
        os.link(blob, destination)
    except OSError:
        shutil.copy2(blob, destination)
    return digest


def store_tree(source, destination):
    for current, directories, files in os.walk(source):
        relative = Path(current).relative_to(source)
        (destination / relative).mkdir(parents=True, exist_ok=True)
        directories.sort()
        for name in sorted(files):
            path = Path(current) / name
            if path.is_file():
                store(path, destination / relative / name)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('name')
    parser.add_argument('--note', required=True)
    parser.add_argument('--verification', type=Path, action='append',
                        help='Retain only these evidence directories instead of the full verification tree')
    args = parser.parse_args()
    if not args.name or any(char not in 'abcdefghijklmnopqrstuvwxyz0123456789_-'
                            for char in args.name):
        parser.error('name must contain only lowercase letters, digits, underscores and hyphens')
    subprocess.run([sys.executable, ROOT / 'scripts/export_patches.py'], check=True)
    destination = ROOT / '.obj/checkpoints' / args.name
    destination.mkdir(parents=True, exist_ok=False)
    files = [path for path in ROOT.rglob('*')
             if path.is_file()
             and not any(part in ('.obj', '.bin', '__pycache__')
                         for part in path.relative_to(ROOT).parts)]
    with tarfile.open(destination / 'source.tar.gz', 'w:gz') as archive:
        for path in sorted(files):
            archive.add(path, arcname='neomutt_xp/' + str(path.relative_to(ROOT)))
    if (ROOT / '.bin').exists():
        store_tree(ROOT / '.bin', destination / 'binaries')
    sources = (args.verification if args.verification is not None
               else [ROOT / '.obj/verification'])
    for evidence in sources:
        if not evidence.exists():
            continue
        target = (destination / 'verification' / evidence.name
                  if args.verification is not None
                  else destination / 'verification')
        store_tree(evidence, target)
    hashes = {str(path.relative_to(destination)): sha256_file(path)
              for path in destination.rglob('*') if path.is_file()}
    metadata = {'name': args.name,
                'created_utc': datetime.now(timezone.utc).isoformat(),
                'note': args.note, 'sha256': hashes}
    (destination / 'checkpoint.json').write_text(json.dumps(metadata, indent=2) + '\n')
    print(destination)


if __name__ == '__main__':
    main()

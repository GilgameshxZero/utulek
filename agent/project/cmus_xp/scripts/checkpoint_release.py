#!/usr/bin/env python3
"""Save a source/binary/evidence checkpoint and optionally install its exact files.

No Git commits, network transfers, or Windows remote operations. Checkpoints
stay under ignored .obj/checkpoints; installation follows the app/bin convention.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re
import shutil
import subprocess
import tarfile

ROOT = Path(__file__).resolve().parent.parent


def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def write_source_archive(destination):
    with tarfile.open(destination, 'w:gz') as archive:
        def source_only(info):
            return None if any(p in ('.obj', '.bin', '.git', '__pycache__', 'out')
                               for p in Path(info.name).parts) else info
        archive.add(ROOT, arcname='cmus_xp', filter=source_only)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--name', required=True)
    parser.add_argument('--linux-dir', type=Path, required=True)
    parser.add_argument('--windows-exe', type=Path, required=True)
    parser.add_argument('--windows-remote', type=Path)
    parser.add_argument('--evidence', type=Path, action='append', default=[])
    parser.add_argument('--install', action='store_true')
    parser.add_argument('--install-dir', type=Path, default=ROOT.parents[3] / 'monochrome/app')
    args = parser.parse_args()
    if not re.fullmatch(r'[a-z0-9_]+', args.name):
        parser.error('Checkpoint name must contain lowercase letters, numbers and underscores')
    checkpoint = ROOT / '.obj/checkpoints' / args.name
    checkpoint.mkdir(parents=True, exist_ok=False)
    files = {}
    artifacts = [('cmus', args.linux_dir / 'cmus'),
                 ('cmus-remote', args.linux_dir / 'cmus-remote'), ('cmus.exe', args.windows_exe)]
    if args.windows_remote:
        artifacts.append(('cmus-remote.exe', args.windows_remote))
    for name, source in artifacts:
        shutil.copy2(source, checkpoint / name)
        files[name] = digest(checkpoint / name)
    write_source_archive(checkpoint / 'source.tar.gz')
    version = subprocess.check_output([str(checkpoint / 'cmus'), '--version'], text=True).splitlines()[0]
    report = {'name': args.name, 'version': version, 'files': files,
              'source_sha256': digest(checkpoint / 'source.tar.gz'),
              'evidence': {str(p.resolve()): digest(p) for p in args.evidence},
              'installed_local': False, 'windows_synced_verified': False}
    report_path = checkpoint / 'checkpoint.json'
    report_path.write_text(json.dumps(report, indent=2) + '\n')
    if args.install:
        reused = []
        for name in files:
            target = args.install_dir / name
            if target.is_file() and digest(target) == files[name]:
                reused.append(name)
                continue
            pending = target.with_name('.' + name + '.install')
            shutil.copy2(checkpoint / name, pending)
            pending.replace(target)
            if digest(target) != files[name]:
                raise SystemExit('Installed checksum mismatch: ' + name)
        report['installed_local'] = True
        report['already_installed'] = reused
        report_path.write_text(json.dumps(report, indent=2) + '\n')
    print(report_path)


if __name__ == '__main__':
    main()

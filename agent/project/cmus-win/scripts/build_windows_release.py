#!/usr/bin/env python3
"""Fetch verified dependencies, build locally, and install the Windows player.

Requires Linux x86_64, Python 3.11+, GCC and GNU make. Dependencies and build
outputs stay in ignored .obj/. The final binary/wrapper use the local app/bin
layout. No SSH, Windows uploads, or remote execution is performed.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import sys
import tarfile
import tempfile
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
DEPS = ROOT / '.obj/deps'


def digest(path):
    with path.open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()


def prepare(name, spec, offline):
    archive = DEPS / spec['archive']
    if not archive.is_file():
        if offline:
            raise RuntimeError('Offline cache missing: ' + str(archive))
        partial = archive.with_suffix(archive.suffix + '.part')
        print('Downloading ' + name + ' ' + spec['version'], flush=True)
        request = urllib.request.Request(spec['url'], headers={'User-Agent': 'cmus-win-build'})
        try:
            with urllib.request.urlopen(request, timeout=60) as response, partial.open('wb') as target:
                shutil.copyfileobj(response, target)
            if digest(partial) != spec['sha256']:
                raise RuntimeError('SHA-256 mismatch: ' + name)
            partial.replace(archive)
        finally:
            partial.unlink(missing_ok=True)
    if digest(archive) != spec['sha256']:
        raise RuntimeError('SHA-256 mismatch in cache: ' + str(archive))
    destination = DEPS / name
    stamp = destination / '.cmus_dependency.json'
    if stamp.is_file() and json.loads(stamp.read_text()) == spec:
        print('Verified cached ' + name, flush=True)
        return
    print('Extracting verified ' + name, flush=True)
    with tempfile.TemporaryDirectory(prefix='extract_', dir=DEPS) as temp:
        stage = Path(temp)
        with tarfile.open(archive) as source:
            source.extractall(stage, filter='data')
        children = list(stage.iterdir())
        if len(children) != 1 or not children[0].is_dir():
            raise RuntimeError('Expected one archive root: ' + name)
        if destination.exists():
            shutil.rmtree(destination)
        children[0].rename(destination)
    stamp.write_text(json.dumps(spec, indent=2) + '\n')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--offline', action='store_true', help='Require cached, verified archives')
    parser.add_argument('--prepare-only', action='store_true', help='Verify and extract dependencies only')
    parser.add_argument('--jobs', type=int, default=min(os.cpu_count() or 2, 8))
    parser.add_argument('--install-dir', type=Path, default=ROOT.parents[3] / 'monochrome/app')
    parser.add_argument('--launcher-dir', type=Path, default=ROOT.parents[3] / 'monochrome/bin')
    args = parser.parse_args()
    if platform.system() != 'Linux' or platform.machine() != 'x86_64':
        parser.error('The pinned cross compiler requires Linux x86_64')
    if args.jobs < 1:
        parser.error('--jobs must be positive')
    for tool in ['gcc', 'make']:
        if not shutil.which(tool):
            parser.error('Missing build prerequisite: ' + tool)
    DEPS.mkdir(parents=True, exist_ok=True)
    specs = json.loads((ROOT / 'scripts/windows_dependencies.json').read_text())
    for name, spec in specs.items():
        prepare(name, spec, args.offline)
    if args.prepare_only:
        return
    env = dict(os.environ, SOURCE_DATE_EPOCH='1735689600', LC_ALL='C')
    subprocess.run([sys.executable, str(ROOT / 'scripts/build_audio_deps.py'),
                    '--jobs', str(args.jobs)], env=env, check=True)
    subprocess.run([sys.executable, str(ROOT / 'scripts/build_native_windows.py'),
                    '--jobs', str(args.jobs), '--install-dir', str(args.install_dir.resolve()),
                    '--launcher-dir', str(args.launcher_dir.resolve())], env=env, check=True)


if __name__ == '__main__':
    try:
        main()
    except (RuntimeError, OSError, tarfile.TarError) as error:
        raise SystemExit(str(error)) from error

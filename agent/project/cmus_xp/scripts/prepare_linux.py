#!/usr/bin/env python3
"""Prepare pinned portable-build dependencies locally; never install system packages."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
DEPS = ROOT / '.obj/deps'


def main():
    DEPS.mkdir(parents=True, exist_ok=True)
    temporary = DEPS / 'tmp'
    temporary.mkdir(exist_ok=True)
    os.environ['TMPDIR'] = str(temporary)
    archives = DEPS / 'sources'
    archives.mkdir(exist_ok=True)
    manifest = json.loads((ROOT / 'scripts/windows_dependencies.json').read_text())
    sources = [dict(manifest['ffmpeg']), dict(manifest['curl-8.23.0']),
        {'archive': 'libao-1.2.2.tar.gz',
         'url': 'https://deb.debian.org/debian/pool/main/liba/libao/libao_1.2.2+20180113.orig.tar.gz',
         'sha256': 'cc9e68134e74f62a1feb6d07e4a0fb565351d80205d1e5d48d540b6abc4cf07d'},
        {'archive': 'libcddb-1.3.2.tar.gz',
         'url': 'https://deb.debian.org/debian/pool/main/libc/libcddb/libcddb_1.3.2.orig.tar.gz',
         'sha256': '93f7860bdafa26f21d60a4d0ce2a8e55a0573841116aade79d90b64651b2b664'}]
    if not shutil.which('nasm') and not (DEPS / 'nasm/nasm').exists():
        sources.append(dict(manifest['nasm']))
    for item in sources:
        archive = archives / item['archive']
        if not archive.exists():
            pending = archive.with_suffix(archive.suffix + '.download')
            print('Downloading ' + item['url'], flush=True)
            urllib.request.urlretrieve(item['url'], pending)
            pending.replace(archive)
        if hashlib.sha256(archive.read_bytes()).hexdigest() != item['sha256']:
            raise SystemExit('Source checksum mismatch: ' + str(archive))
    ffmpeg = DEPS / 'sources' / ('ffmpeg-' + manifest['ffmpeg']['version'])
    if not ffmpeg.is_dir():
        with tarfile.open(archives / manifest['ffmpeg']['archive']) as tf:
            tf.extractall(archives, filter='data')
    if any(item['archive'] == manifest['nasm']['archive'] for item in sources):
        with tarfile.open(archives / manifest['nasm']['archive']) as tf:
            tf.extractall(archives, filter='data')
        nasm_source = archives / ('nasm-' + manifest['nasm']['version'])
        subprocess.run(['./configure', '--prefix=' + str(DEPS / 'nasm')], cwd=nasm_source, check=True)
        subprocess.run(['make', '-j4', 'install'], cwd=nasm_source, check=True)
        (DEPS / 'nasm/nasm').symlink_to('bin/nasm')
    packaging = DEPS / 'packaging_env'
    if not (packaging / 'bin/python').exists():
        subprocess.run([sys.executable, '-m', 'venv', str(packaging)], check=True)
    subprocess.run([sys.executable, '-m', 'pip', '--python', str(packaging / 'bin/python'),
                    'install', '--no-cache-dir', '-r', str(ROOT / 'scripts/linux_packaging_requirements.txt')],
                   check=True)
    subprocess.run([sys.executable, str(ROOT / 'scripts/build_linux_support.py')], check=True)
    curl = archives / ('curl-' + manifest['curl-8.23.0']['version'])
    if not curl.is_dir():
        with tarfile.open(archives / manifest['curl-8.23.0']['archive']) as tf:
            tf.extractall(archives, filter='data')
    subprocess.run([sys.executable, str(ROOT / 'scripts/build_http_deps.py'),
                    '--host', '--source', str(curl)], check=True)
    subprocess.run([sys.executable, str(ROOT / 'scripts/build_audio_deps.py'),
                    '--host', '--source', str(ffmpeg)], check=True)


if __name__ == '__main__':
    main()

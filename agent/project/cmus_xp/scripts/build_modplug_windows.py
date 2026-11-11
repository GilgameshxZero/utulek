#!/usr/bin/env python3
"""Build the pinned public-domain libmodplug static library for Windows.

No system packages are installed. Sources stay under the ignored .obj tree.
"""
import hashlib
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
DEPS = ROOT / '.obj/deps/modplug'
VERSION = '0.8.9.0'
URL = ('https://downloads.sourceforge.net/project/modplug-xmms/libmodplug/'
       f'{VERSION}/libmodplug-{VERSION}.tar.gz')
SHA256 = '457ca5a6c179656d66c01505c0d95fafaead4329b9dbaa0f997d00a3508ad9de'

CONFIG_H = '''#define HAVE_CONFIG_H 1
#define HAVE_INTTYPES_H 1
#define HAVE_STDINT_H 1
#define HAVE_STDLIB_H 1
#define HAVE_STRING_H 1
#define HAVE_STRINGS_H 1
#define HAVE_SYS_STAT_H 1
#define HAVE_SYS_TYPES_H 1
#define STDC_HEADERS 1
#define HAVE_SINF 1
#define VERSION "0.8.9.0"
'''


def main():
    toolchain = ROOT / '.obj/deps/llvm_mingw'
    cxx = toolchain / 'bin/x86_64-w64-mingw32-clang++'
    ar = toolchain / 'bin/llvm-ar'
    if not cxx.is_file():
        sys.exit('LLVM-MinGW compiler not found: ' + str(cxx))
    DEPS.mkdir(parents=True, exist_ok=True)
    archive = DEPS / f'libmodplug-{VERSION}.tar.gz'
    source = DEPS / f'libmodplug-{VERSION}'
    if not archive.is_file():
        print('downloading', URL)
        urllib.request.urlretrieve(URL, archive)
    digest = hashlib.sha256(archive.read_bytes()).hexdigest()
    if digest != SHA256:
        sys.exit(f'libmodplug archive checksum mismatch: {digest}')
    if not (source / 'src/modplug.h').is_file():
        with tarfile.open(archive) as tar:
            tar.extractall(DEPS)
    build = source / 'build_windows'
    obj = build / 'obj'
    include = build / 'include'
    shutil.rmtree(obj, ignore_errors=True)
    obj.mkdir(parents=True)
    include.mkdir(parents=True, exist_ok=True)
    (include / 'config.h').write_text(CONFIG_H)
    header = include / 'libmodplug'
    header.mkdir(exist_ok=True)
    shutil.copy2(source / 'src/modplug.h', header / 'modplug.h')
    flags = [str(cxx), '-std=gnu++11', '-O2', '-DNDEBUG', '-DHAVE_CONFIG_H',
             '-DMODPLUG_BUILD', '-DMODPLUG_STATIC', '-fno-exceptions', '-fno-rtti',
             '-D_CRT_SECURE_NO_WARNINGS', '-I' + str(include),
             '-I' + str(source / 'src'), '-I' + str(source / 'src/libmodplug')]
    objects = []
    for path in sorted((source / 'src').glob('*.cpp')):
        target = obj / (path.stem + '.o')
        subprocess.run(flags + ['-c', str(path), '-o', str(target)], check=True)
        objects.append(str(target))
    library = build / 'libmodplug.a'
    library.unlink(missing_ok=True)
    subprocess.run([str(ar), 'rcs', str(library), *objects], check=True)
    print(library)


if __name__ == '__main__':
    main()

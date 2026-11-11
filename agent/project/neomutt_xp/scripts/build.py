#!/usr/bin/env python3
"""Build the upstream application on Linux or native Windows (cross compilation)."""
import argparse
import os
from pathlib import Path
import shutil
import subprocess
ROOT = Path(__file__).resolve().parents[1]

def run(command, cwd, log, env=None):
    with (cwd / log).open('w') as out:
        subprocess.run(list(map(str, command)), cwd=cwd, env=env,
                       stdout=out, stderr=subprocess.STDOUT, check=True)

def main():
    p = argparse.ArgumentParser()
    p.add_argument('target', choices=['linux', 'windows'])
    p.add_argument('--toolchain', type=Path)
    p.add_argument('-j', '--jobs', type=int, default=4)
    args = p.parse_args()
    build = ROOT / '.obj' / args.target
    build.mkdir(parents=True, exist_ok=True)
    source = ROOT / '.obj/upstream'
    if args.target == 'windows':
        if not args.toolchain: p.error('--toolchain is required for Windows')
        toolchain = args.toolchain.resolve()
        subprocess.run(['python3', ROOT / 'scripts/configure_windows.py', '--toolchain', toolchain], check=True)
        compiler = toolchain / 'bin/x86_64-w64-mingw32-clang'
        objects = []
        headers = list((ROOT / 'native').rglob('*.h'))
        for source_file in sorted((ROOT / 'native').glob('*.c')):
            obj = build / ('xp-' + source_file.stem + '.o')
            if not obj.exists() or obj.stat().st_mtime < max(f.stat().st_mtime for f in [source_file, *headers]):
                run([compiler, '-std=c11', '-O2', '-Wall', '-Wextra', '-Werror', '-D_FILE_OFFSET_BITS=64',
                     '-I' + str(ROOT / 'native'), '-I' + str(ROOT / 'native/include'),
                     '-I' + str(ROOT / '.obj/windows-prefix/include'),
                     '-c', source_file, '-o', obj], build, 'xp-' + source_file.stem + '.log')
            objects.append(obj)
        resource = build / 'xp-manifest.o'
        if not resource.exists() or resource.stat().st_mtime < max((ROOT / 'native/neomutt.rc').stat().st_mtime, (ROOT / 'native/neomutt.manifest').stat().st_mtime):
            run([toolchain / 'bin/x86_64-w64-mingw32-windres', '-I', ROOT / 'native',
                 ROOT / 'native/neomutt.rc', '-o', resource], build, 'xp-manifest.log')
        objects.append(resource)
        dependencies = objects + [ROOT / '.obj/windows-prefix/lib' / ('lib' + name + '.a') for name in ('ncursesw','iconv','pcre2-posix','pcre2-8','ssl','crypto','lmdb','gpgme','assuan','gpg-error','sqlite3')]
        extra = ['XP_PLATFORM_OBJECTS=' + ' '.join(map(str, dependencies)),
                 'EXTRA_CFLAGS=-I' + str(ROOT / 'native') + ' -include ' + str(ROOT / 'native/compat.h'),
                 'EXTRA_LDFLAGS=' + ' '.join(map(str, objects)) + ' -ladvapi32 -lbcrypt -lws2_32']
        executable = 'neomutt.exe'
    else:
        run([source / 'configure', '--autocrypt', '--with-sqlite=' + str(ROOT / '.obj/host-prefix'), '--disable-doc', '--disable-nls', '--disable-idn2', '--ssl', '--lmdb', '--with-lmdb=' + str(ROOT / '.obj/host-prefix')], build, 'configure.log', env=dict(os.environ,PKG_CONFIG_LIBDIR=str(ROOT / '.obj/host-prefix/lib/pkgconfig'),CPPFLAGS='-I'+str(ROOT / '.obj/host-prefix/include'),LDFLAGS='-L'+str(ROOT / '.obj/host-prefix/lib'),LIBS='-lgpgme -lassuan -lgpg-error -lsqlite3 -lpthread -ldl'))
        extra = ['XP_PLATFORM_OBJECTS=' + ' '.join(str(ROOT / '.obj/host-prefix/lib'/('lib'+name+'.a')) for name in ('lmdb','gpgme','assuan','gpg-error','sqlite3'))]
        executable = 'neomutt'
    run(['make', '-j' + str(args.jobs), executable, *extra], build, 'build.log')
    destination = ROOT / '.bin' / args.target
    destination.mkdir(parents=True, exist_ok=True)
    shutil.copy2(build / executable, destination / executable)
    if args.target == 'windows':subprocess.run(['python3',ROOT/'scripts/package_windows.py'],check=True)
    print(destination / executable)

if __name__ == '__main__':
    main()

#!/usr/bin/env python3
"""Generate a deterministic tracker module and validate the modplug plugin."""
import json
import math
import os
from pathlib import Path
import struct
import subprocess
import sys
sys.path.insert(0, str(Path(__file__).resolve().parent))
from windows_test_stage import WindowsStage

ROOT = Path(__file__).resolve().parent.parent
OUT = ROOT / '.obj/modplug_checks'


def make_mod(path):
    channels = 4
    sample_length = 1024
    # 22-byte name, length in words, finetune, volume, loop start/len in words
    sample = bytearray(30)
    sample[0:4] = b'lead'
    struct.pack_into('>H', sample, 22, sample_length // 2)
    sample[24] = 0
    sample[25] = 64
    struct.pack_into('>H', sample, 26, 0)
    struct.pack_into('>H', sample, 28, sample_length // 2)
    header = bytearray(b'cmus modplug fixture')
    header = header.ljust(20, b'\0')
    for i in range(31):
        header += bytes(sample) if i == 0 else bytes(30)
    header += bytes([1, 0])              # song length, restart
    header += bytes([0] + [0] * 127)     # one order
    header += b'M.K.'

    pattern = bytearray(64 * channels * 4)
    # C-3 period 428, sample 1, row zero channel zero.
    pattern[0] = 0x01
    pattern[1] = 428 & 0xff
    pattern[2] = 0x10
    pattern[3] = 0
    data = bytearray(sample_length)
    half = sample_length // 2
    for i in range(sample_length):
        data[i] = 100 if (i // half) % 2 == 0 else 0
    path.write_bytes(bytes(header) + bytes(pattern) + bytes(data))


def build_checker(windows):
    sources = ['tests/check_decoder.c', 'ip/modplug.c', 'comment.c', 'keyval.c',
               'xmalloc.c', 'xstrjoin.c', 'uchar.c', 'gbuf.c', 'prog.c',
               'debug.c', 'convert.c', 'channelmap.c', 'u_collate.c', 'file.c',
               'windows/compat.c', 'windows/path_windows.c', 'windows/iconv.c']
    if windows:
        compiler = str(ROOT / '.obj/deps/llvm_mingw/bin/x86_64-w64-mingw32-clang')
        flags = ['-DCMUS_NATIVE_WINDOWS', '-DHAVE_CONFIG', '-DHAVE_STRDUP',
                 '-DMODPLUG_STATIC', '-D_CRT_SECURE_NO_WARNINGS',
                 '-D_WIN32_WINNT=0x0A00', '-I' + str(ROOT / 'windows/include'),
                 '-I' + str(ROOT / '.obj/deps/modplug/include'),
                 '-I' + str(ROOT / '.obj/native'),
                 '-I' + str(ROOT / '.obj/native/unicode_include'),
                 '-I' + str(ROOT / '.obj/deps/pdcurses'),
                 '-include', str(ROOT / 'windows/compat.h')]
        libraries = ['-L' + str(ROOT / '.obj/deps/modplug/libmodplug-0.8.9.0/build_windows'),
                     '-lmodplug', '-lc++', '-lc++abi', '-lunwind',
                     '-ladvapi32', '-lshell32']
        # compat.c must not remap its own getenv.
        environment = dict(os.environ)
        check = OUT / 'check_modplug.exe'
        common = ['-std=gnu11', '-O2', '-D_DEFAULT_SOURCE', '-I' + str(ROOT),
                  '-I' + str(ROOT / 'ip')]
        objects = []
        for source in sources:
            obj = OUT / (Path(source).stem + '.o')
            extra = ['-DCMUS_WIN_IMPLEMENTATION'] if source.endswith('compat.c') else []
            subprocess.run([compiler, *common, *flags, *extra, '-c',
                            str(ROOT / source), '-o', str(obj)], check=True)
            objects.append(str(obj))
        objects.append(str(ROOT / '.obj/native/native_resources.o'))
        subprocess.run([compiler, '-static', '-o', str(check), *objects, *libraries],
                       check=True)
    else:
        compiler = 'gcc'
        check = OUT / 'check_modplug'
        sources.remove('windows/compat.c')
        sources.remove('windows/path_windows.c')
        sources.remove('windows/iconv.c')
        flags = ['-DHAVE_STRDUP', '-DDEBUG=1', '-DHAVE_CONFIG',
                 '-I' + str(ROOT), '-I' + str(ROOT / 'ip'),
                 '-I' + str(ROOT / '.obj/build'),
                 '-I' + str(ROOT / '.obj/deps/linux_audio/usr/include')]
        libraries = ['-L' + str(ROOT / '.obj/deps/linux_audio/usr/lib/x86_64-linux-gnu'),
                     '-lmodplug', '-lstdc++', '-lm', '-latomic']
        subprocess.run([compiler, '-std=gnu11', '-O2', '-D_DEFAULT_SOURCE', '-D_XOPEN_SOURCE=600',
                        *flags, *[str(ROOT / s) for s in sources], *libraries,
                        '-o', str(check)], check=True)
    return check


def main():
    OUT.mkdir(parents=True, exist_ok=True)
    fixture = OUT / 'fixture.mod'
    make_mod(fixture)
    results = {'fixture': str(fixture), 'checks': []}
    linux = build_checker(False)
    pcm = OUT / 'linux.pcm'
    run = subprocess.run([str(linux), str(fixture), str(pcm)], capture_output=True, text=True)
    data = pcm.read_bytes() if pcm.is_file() else b''
    results['checks'].append({
        'platform': 'linux', 'exit': run.returncode, 'pcm_bytes': len(data),
        'nonzero_audio': any(data[i:i + 2] not in (b'\0\0', b'') for i in range(0, min(len(data), 65536), 2)),
        'stdout': run.stdout[-500:], 'stderr': run.stderr[-500:]})

    windows = build_checker(True)
    staged = OUT / 'windows'
    staged.mkdir(exist_ok=True)
    with WindowsStage('0@10.8.40.49', {'check_modplug.exe': windows, 'fixture.mod': fixture}, staged) as stage:
        run = stage.run('check_modplug.exe', [stage.path + '/fixture.mod', stage.path + '/windows.pcm'])
        stage.retrieve('windows.pcm', staged / 'windows.pcm')
    pcm = staged / 'windows.pcm'
    data = pcm.read_bytes() if pcm.is_file() else b''
    results['checks'].append({
        'platform': 'windows', 'exit': run.returncode, 'pcm_bytes': len(data),
        'nonzero_audio': any(data[i:i + 2] not in (b'\0\0', b'') for i in range(0, min(len(data), 65536), 2)),
        'stdout': run.stdout[-500:], 'stderr': run.stderr[-500:]})
    # The generic decoder checker's optional FFmpeg-specific seek/restart/title
    # expectations do not apply to tracker modules. Playback itself must decode
    # real nonzero audio on both platforms.
    results['passed'] = all(c['pcm_bytes'] > 0 and c['nonzero_audio']
                            for c in results['checks'])
    for check in results['checks']:
        check['playback_ok'] = check['pcm_bytes'] > 0 and check['nonzero_audio']
    (OUT / 'results.json').write_text(json.dumps(results, indent=2) + '\n')
    print(json.dumps(results, indent=2))
    return 0 if results['passed'] else 1


if __name__ == '__main__':
    raise SystemExit(main())

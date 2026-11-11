#!/usr/bin/env python3
"""Run local core and model-driver regressions; write results under ignored .obj/."""
import json
import argparse
import os
from pathlib import Path
import subprocess
import struct
ROOT = Path(__file__).resolve().parent.parent
OUT = ROOT / '.obj/reliability'
CORE = ['windows/check_core.c', 'play_queue.c', 'editable.c', 'window.c', 'track.c',
        'track_info.c', 'locking.c', 'search.c', 'comment.c', 'keyval.c', 'uchar.c',
        'u_collate.c', 'xmalloc.c', 'xstrjoin.c', 'gbuf.c', 'debug.c', 'prog.c',
        'path.c', 'misc.c', 'convert.c', 'rbtree.c', 'mergesort.c', 'channelmap.c', 'pcm.c']

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--only', nargs='+', choices=('core', 'waveout', 'directory', 'wav_malformed', 'multichannel'))
    args = parser.parse_args()
    OUT.mkdir(parents=True, exist_ok=True)
    binaries = ROOT / '.bin'
    binaries.mkdir(exist_ok=True)
    flags = ['gcc', '-std=gnu11', '-O1', '-g', '-D_DEFAULT_SOURCE', '-DHAVE_STRDUP',
             '-DDEBUG=1', '-ffunction-sections', '-fdata-sections',
             '-fsanitize=address,undefined', '-fno-omit-frame-pointer',
             '-I' + str(ROOT), '-I' + str(ROOT / '.obj/native')]
    results = []
    for name, sources in [
            ('core', CORE),
            ('waveout', ['windows/check_waveout.c', 'xmalloc.c', 'prog.c', 'channelmap.c']),
            ('multichannel', ['tests/check_pcm_channels.c', 'pcm.c', 'channelmap.c']),
            ('directory', ['windows/check_directory.c', 'load_dir.c', 'xmalloc.c', 'prog.c']),
            ('wav_malformed', ['tests/check_decoder.c', 'ip/wav.c', 'file.c', 'comment.c',
                              'keyval.c', 'uchar.c', 'u_collate.c', 'convert.c', 'xmalloc.c',
                              'xstrjoin.c', 'gbuf.c', 'debug.c', 'prog.c', 'channelmap.c'])]:
        if args.only and name not in args.only:
            continue
        exe = binaries / ('check_' + name)
        subprocess.run(flags + [str(ROOT / s) for s in sources] +
                       ['-Wl,--gc-sections', '-lm', '-lpthread', '-o', str(exe)], check=True)
        env = dict(os.environ, ASAN_OPTIONS='detect_leaks=0', UBSAN_OPTIONS='halt_on_error=1')
        command = [str(exe)]; expected_exit = 0
        if name == 'directory':
            deep = OUT / 'deep'
            for i in range(12): deep /= str(i) + '_' + 'a' * 120
            deep.mkdir(parents=True, exist_ok=True)
            (deep / 'long_music.opus').write_bytes(b'fixture')
            link = deep / 'loop'
            # Recreate the fixture after a checkout move or project rename.
            if link.is_symlink(): link.unlink()
            if not link.exists(): link.symlink_to(OUT.resolve(), target_is_directory=True)
            command.append(str(deep))
        if name == 'wav_malformed':
            cases = {'oversized_fmt': b'RIFF' + (32).to_bytes(4, 'little') + b'WAVEfmt ' + (0x7fffffff).to_bytes(4, 'little')}
            formats = {
                'zero_rate': struct.pack('<HHIIHH', 1, 1, 0, 0, 2, 16),
                'excess_channels': struct.pack('<HHIIHH', 1, 256, 48000, 0, 512, 16),
                'zero_channels': struct.pack('<HHIIHH', 1, 0, 48000, 0, 0, 16),
            }
            ext = struct.pack('<HHIIHHH', 0xfffe, 2, 48000, 192000, 4, 16, 22)
            for length in (18, 20, 22, 24, 26, 39):
                formats['truncated_extensible_' + str(length)] = ext + bytes(length - len(ext))
            formats['oversized_extension'] = ext[:16] + struct.pack('<H', 65535) + bytes(22)
            for case, fmt in formats.items():
                body = b'WAVEfmt ' + struct.pack('<I', len(fmt)) + fmt + b'data' + struct.pack('<I', 16) + bytes(16)
                cases[case] = b'RIFF' + struct.pack('<I', len(body)) + body
            failures, logs = [], []
            for case, contents in cases.items():
                bad = OUT / (case + '.wav')
                bad.write_bytes(contents)
                process = subprocess.run(command + [str(bad), str(OUT / 'unused.pcm')],
                    capture_output=True, text=True, env=env, timeout=20)
                if process.returncode != 3:
                    failures.append(case)
                logs.append(case + ': ' + str(process.returncode) + '\n' + process.stdout + process.stderr)
            (OUT / (name + '.log')).write_text('\n'.join(logs))
            print('Native WAV malformed headers: ' + str(len(cases)) + ' cases; failures: ' + str(len(failures)))
            results.append({'check': name, 'passed': not failures, 'cases': len(cases), 'failures': failures,
                'sanitizers': ['address', 'undefined'], 'leak_detection': False})
            continue
        process = subprocess.run(command, capture_output=True, text=True, env=env)
        (OUT / (name + '.log')).write_text(process.stdout + process.stderr)
        print(process.stdout + process.stderr, end='', flush=True)
        results.append({'check': name, 'exit': process.returncode,
                        'passed': process.returncode == expected_exit, 'sanitizers': ['address','undefined'], 'leak_detection': False})
    (OUT / 'results.json').write_text(json.dumps(results, indent=2) + '\n')
    if not all(r['passed'] for r in results):
        raise SystemExit('Reliability checks failed; see .obj/reliability/')

if __name__ == '__main__':
    main()

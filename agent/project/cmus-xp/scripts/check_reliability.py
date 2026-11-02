#!/usr/bin/env python3
"""Run local core and model-driver regressions; write results under ignored .obj/."""
import json
import os
from pathlib import Path
import subprocess
ROOT = Path(__file__).resolve().parent.parent
OUT = ROOT / '.obj/reliability'
CORE = ['windows/check_core.c', 'play_queue.c', 'editable.c', 'window.c', 'track.c',
        'track_info.c', 'locking.c', 'search.c', 'comment.c', 'keyval.c', 'uchar.c',
        'u_collate.c', 'xmalloc.c', 'xstrjoin.c', 'gbuf.c', 'debug.c', 'prog.c',
        'path.c', 'misc.c', 'convert.c', 'rbtree.c', 'mergesort.c']

def main():
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
            ('waveout', ['windows/check_waveout.c', 'xmalloc.c', 'prog.c']),
            ('directory', ['windows/check_directory.c', 'load_dir.c', 'xmalloc.c', 'prog.c']),
            ('wav_malformed', ['tests/check_decoder.c', 'ip/wav.c', 'file.c', 'comment.c',
                              'keyval.c', 'uchar.c', 'u_collate.c', 'convert.c', 'xmalloc.c',
                              'xstrjoin.c', 'gbuf.c', 'debug.c', 'prog.c', 'channelmap.c'])]:
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
            bad = OUT / 'oversized_fmt.wav'
            bad.write_bytes(b'RIFF' + (32).to_bytes(4, 'little') + b'WAVEfmt ' + (0x7fffffff).to_bytes(4, 'little'))
            command += [str(bad), str(OUT / 'unused.pcm')]; expected_exit = 3
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

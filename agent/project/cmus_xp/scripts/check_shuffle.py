#!/usr/bin/env python3
"""Sanitized shuffle regression using the Windows 15-bit RNG and forced collisions."""
import argparse
import json
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parent.parent

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source', type=Path, default=ROOT / 'track.c')
    parser.add_argument('--report', type=Path)
    parser.add_argument('--timeout', type=float, default=30)
    args = parser.parse_args()
    out = ROOT / '.obj/hardening'
    out.mkdir(parents=True, exist_ok=True)
    cc = 'gcc'
    exe = ROOT / '.bin/check_shuffle'
    flags = ['-fsanitize=address,undefined', '-fno-omit-frame-pointer']
    subprocess.run([cc, '-std=gnu11', '-O1', '-g', '-D_DEFAULT_SOURCE', '-DHAVE_STRDUP',
        '-DDEBUG=1', '-include', str(ROOT / 'tests/shuffle_rng.h'), '-ffunction-sections', '-fdata-sections',
        '-I'+str(ROOT), '-I'+str(ROOT / '.obj/native'), *flags,
        str(ROOT / 'tests/check_shuffle.c'), str(args.source), str(ROOT / 'misc.c'),
        str(ROOT / 'rbtree.c'), str(ROOT / 'xmalloc.c'), str(ROOT / 'prog.c'),
        '-Wl,--gc-sections', '-lm', '-o', str(exe)], check=True)
    try:
        result = subprocess.run([str(exe)], capture_output=True, text=True, timeout=args.timeout,
            env=dict(os.environ, ASAN_OPTIONS='detect_leaks=0', UBSAN_OPTIONS='halt_on_error=1'))
        report = {'exit': result.returncode, 'passed': result.returncode == 0,
                  'stdout': result.stdout, 'stderr': result.stderr}
    except subprocess.TimeoutExpired:
        report = {'passed': False, 'timeout_seconds': args.timeout}
    (args.report or out / 'shuffle_results.json').write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps(report, indent=2))
    if not report['passed']: raise SystemExit(1)

if __name__ == '__main__':
    main()

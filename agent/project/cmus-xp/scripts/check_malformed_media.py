#!/usr/bin/env python3
"""Bounded deterministic mutation checks of the actual decoder under ASan/UBSan."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import random
import subprocess

ROOT = Path(__file__).resolve().parent.parent

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--checker', type=Path, default=ROOT / '.bin/check_decoder_sanitized')
    parser.add_argument('--fixtures', type=Path, default=ROOT / '.obj/hardening/decoders')
    parser.add_argument('--cases', type=int, default=400)
    args = parser.parse_args()
    out = ROOT / '.obj/hardening/malformed'
    out.mkdir(parents=True, exist_ok=True)
    seeds = [p for p in sorted(args.fixtures.iterdir()) if p.is_file() and
             p.suffix in ('.wav', '.opus', '.flac', '.m4a', '.mkv', '.mp3', '.wma', '.aiff', '.flv')]
    assert seeds
    rng = random.Random(532948)
    results = []
    env = dict(os.environ, ASAN_OPTIONS='detect_leaks=0:abort_on_error=1', UBSAN_OPTIONS='halt_on_error=1')
    for i in range(args.cases):
        seed = seeds[i % len(seeds)]
        data = bytearray(seed.read_bytes()[:65536])
        kind = i % 5
        if kind == 0:
            data = data[:rng.randrange(len(data)+1)]
        elif kind == 1:
            for _ in range(min(12, len(data))):
                j = rng.randrange(len(data)); data[j] ^= 1 << rng.randrange(8)
        elif kind == 2:
            j = rng.randrange(min(128, len(data))+1)
            data[j:j+8] = b'\xff' * 8
        elif kind == 3:
            data = data[:min(len(data), rng.choice([0, 1, 8, 24, 44, 128, 1024]))]
        else:
            data = bytearray(rng.randbytes(rng.randrange(4096)))
        path = out / ('case_%04d' % i + seed.suffix)
        path.write_bytes(data)
        row = {'case': path.name, 'seed': seed.name, 'mutation': kind,
               'sha256': hashlib.sha256(data).hexdigest()}
        try:
            process = subprocess.run([str(args.checker.resolve()), str(path), '-'],
                                     capture_output=True, text=True, timeout=3, env=env)
            row['exit'] = process.returncode
            row['passed'] = process.returncode in (0, 1, 3, 7, 8) and not any(
                text in process.stderr for text in ('AddressSanitizer', 'runtime error:'))
            if not row['passed']:
                row['stderr'] = process.stderr[-12000:]
        except subprocess.TimeoutExpired:
            row.update(passed=False, timeout_seconds=3)
        results.append(row)
        if (i+1) % 50 == 0: print(i+1, 'cases;', sum(not r['passed'] for r in results), 'failures', flush=True)
    (out / 'results.json').write_text(json.dumps(results, indent=2)+'\n')
    if not all(r['passed'] for r in results): raise SystemExit('Malformed-media checks failed')

if __name__ == '__main__':
    main()

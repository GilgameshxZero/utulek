#!/usr/bin/env python3
"""Compare initial-decode CPU time of saved baseline and candidate checkers.

Both checkers must use tests/check_decoder.c, which reports CPU time separately
from opening, seeking and restarting. This is a local throughput benchmark, not
a measurement of hardware playback latency or power use.
"""
import argparse
import json
from pathlib import Path
import statistics
import subprocess

ROOT = Path(__file__).resolve().parent.parent
OUT = ROOT / '.obj/efficiency'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--baseline', type=Path, required=True)
    parser.add_argument('--candidate', type=Path, default=ROOT / '.bin/check_decoder')
    parser.add_argument('--runs', type=int, default=7)
    parser.add_argument('--seconds', type=int, default=60)
    args = parser.parse_args()
    if args.runs < 3 or args.seconds < 10:
        parser.error('Use at least three runs and ten seconds of audio')
    OUT.mkdir(parents=True, exist_ok=True)
    results = []
    for extension, encoder in [('opus', 'libopus'), ('mp3', 'libmp3lame'),
                                ('flac', 'flac'), ('m4a', 'aac'), ('wav', 'pcm_s16le')]:
        path = OUT / ('benchmark.' + extension)
        subprocess.run(['ffmpeg', '-hide_banner', '-loglevel', 'error', '-y',
                        '-f', 'lavfi', '-i', 'anoisesrc=color=pink:sample_rate=48000:duration=' + str(args.seconds),
                        '-ac', '2', '-c:a', encoder, str(path)], check=True)
        timings = {'baseline': [], 'candidate': []}
        decoded = {}
        exits = {}
        # Alternate order to reduce systematic cache/scheduling bias; warm up first.
        for run in range(args.runs + 1):
            names = ['baseline', 'candidate'] if run % 2 else ['candidate', 'baseline']
            for name in names:
                process = subprocess.run([str(getattr(args, name).resolve()), str(path), '-'],
                                         capture_output=True, text=True, timeout=30)
                # Older decoders can fail the new restart check; measure only
                # the initial decode and retain the exit code for transparency.
                report = json.loads(process.stdout)
                if not report['bytes']:
                    raise RuntimeError('Empty decode: ' + name)
                decoded[name] = report['bytes']
                exits[name] = process.returncode
                if run:
                    timings[name].append(report['decode_cpu_seconds'])
        if decoded['baseline'] != decoded['candidate']:
            raise RuntimeError('Decoded lengths differ: ' + extension)
        baseline = statistics.median(timings['baseline'])
        candidate = statistics.median(timings['candidate'])
        result = {'format': extension, 'audio_seconds': args.seconds,
                  'bytes': decoded['candidate'], 'runs': timings,
                  'checker_exit_codes': exits,
                  'baseline_cpu_seconds': baseline, 'candidate_cpu_seconds': candidate,
                  'speedup': baseline / candidate}
        results.append(result)
        print('%s: %.2fx throughput; %.6fs -> %.6fs CPU' %
              (extension, result['speedup'], baseline, candidate), flush=True)
    (OUT / 'results.json').write_text(json.dumps(results, indent=2) + '\n')


if __name__ == '__main__':
    main()

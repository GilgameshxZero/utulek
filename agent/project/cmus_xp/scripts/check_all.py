#!/usr/bin/env python3
"""Run related checks in parallel and index their outcomes.

Each check keeps its own detailed report; this runner only records the exit
status, duration and captured output under .obj/checks/. Groups use the
finished binaries in .bin, so build first.
"""
import argparse
import concurrent.futures
import json
import os
from pathlib import Path
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parent.parent
BIN = ROOT / '.bin'
RESULTS = ROOT / '.obj/checks'


def py(script, *arguments):
    return [sys.executable, str(ROOT / 'scripts' / script), *map(str, arguments)]


GROUPS = {
    'core': py('check_reliability.py'),
    'shuffle': py('check_shuffle.py'),
    'single-file': py('check_single_file.py', '--binary', BIN / 'cmus',
                      '--remote', BIN / 'cmus-remote'),
    'portable-runtime': py('check_portable_runtime.py', '--binary', BIN / 'cmus',
                           '--remote', BIN / 'cmus-remote'),
    'playlist-sources': py('check_playlist_sources.py', '--binary', BIN / 'cmus',
                           '--remote', BIN / 'cmus-remote'),
    'persistence': py('check_persistence.py', '--binary', BIN / 'cmus',
                      '--remote', BIN / 'cmus-remote'),
    'remote': py('check_remote.py', '--binary', BIN / 'cmus', '--remote', BIN / 'cmus-remote',
                 '--output', RESULTS / 'remote' / 'results.json'),
    'large-library': py('check_large_library.py', '--binary', BIN / 'cmus',
                        '--remote', BIN / 'cmus-remote'),
    'endurance': py('check_endurance.py', '--candidate', BIN,
                    '--work', RESULTS / 'endurance', '--output',
                    RESULTS / 'endurance' / 'results.json'),
    'linux-parity': py('check_linux_parity.py', '--binary', BIN / 'cmus',
                       '--remote', BIN / 'cmus-remote'),
    'linux-audio': py('check_linux_audio.py', '--binary', BIN / 'cmus',
                      '--remote', BIN / 'cmus-remote'),
    'decoders': py('check_audio_decoders.py', '--native-channels'),
    'modplug': py('check_modplug.py'),
}

ALIASES = {
    'quick': ['core', 'shuffle', 'single-file', 'playlist-sources', 'persistence'],
    'playback': ['endurance', 'large-library'],
    'all': list(GROUPS),
}


def expand(names):
    expanded = []
    for name in names:
        for item in ALIASES.get(name, [name]):
            if item not in expanded:
                expanded.append(item)
    unknown = [name for name in expanded if name not in GROUPS]
    if unknown:
        raise SystemExit('Unknown check group(s): ' + ', '.join(unknown))
    return expanded


def run_one(name, timeout):
    command = GROUPS[name]
    started = time.monotonic()
    try:
        process = subprocess.run(command, cwd=ROOT, capture_output=True, text=True,
                                 timeout=timeout, env=dict(os.environ, PYTHONPATH=str(ROOT / 'scripts')))
        code, output = process.returncode, process.stdout + process.stderr
    except subprocess.TimeoutExpired as error:
        code = 124
        output = (error.stdout or '') + (error.stderr or '') + f'\nTimed out after {timeout}s\n'
        if isinstance(output, bytes):
            output = output.decode(errors='replace')
    elapsed = round(time.monotonic() - started, 2)
    RESULTS.mkdir(parents=True, exist_ok=True)
    (RESULTS / (name + '.log')).write_text(output)
    print(f'{"PASS" if code == 0 else "FAIL"}  {name}  {elapsed}s')
    return {'check': name, 'passed': code == 0, 'exit': code, 'seconds': elapsed,
            'log': str(RESULTS / (name + '.log'))}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('groups', nargs='*', default=['quick'],
                        help='Group names; defaults to "quick". Use --list to see them.')
    parser.add_argument('--list', action='store_true')
    parser.add_argument('--parallel', type=int, default=min(os.cpu_count() or 2, 4))
    parser.add_argument('--timeout', type=int, default=1800)
    parser.add_argument('--output', type=Path, default=RESULTS / 'results.json')
    args = parser.parse_args()
    if args.list:
        for name, command in GROUPS.items():
            print(f'{name:20} {Path(command[1]).name}')
        print('\nAliases: ' + ', '.join(f'{k}={"+".join(v)}' for k, v in ALIASES.items()))
        return
    names = expand(args.groups)
    with concurrent.futures.ThreadPoolExecutor(max_workers=max(1, args.parallel)) as pool:
        results = list(pool.map(lambda name: run_one(name, args.timeout), names))
    payload = {'passed': all(item['passed'] for item in results),
               'checks': results, 'groups': names}
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(payload, indent=2) + '\n')
    print(json.dumps({'passed': payload['passed'], 'results': str(args.output)}, indent=2))
    return 0 if payload['passed'] else 1


if __name__ == '__main__':
    raise SystemExit(main())

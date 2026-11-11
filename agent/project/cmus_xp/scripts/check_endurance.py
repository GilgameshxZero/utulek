#!/usr/bin/env python3
"""Sustained local playback, format transitions and failure recovery on Linux."""
import argparse
import json
from pathlib import Path
import subprocess
import time

from linux_test_session import Session

ROOT = Path(__file__).resolve().parent.parent


def wait_status(session, needle, limit=100):
    for _ in range(limit):
        text = session.command('status')
        if needle in text:
            return True, text
        session.drain(.03)
    return False, text


def process_tree(pid):
    seen = []
    pending = [pid]
    while pending:
        current = pending.pop()
        seen.append(current)
        try:
            children = (Path('/proc') / str(current) / 'task' / str(current) /
                        'children').read_text().split()
        except OSError:
            continue
        pending.extend(int(child) for child in children)
    return seen


def memory(pid):
    rss = 0
    fds = 0
    for current in process_tree(pid):
        try:
            for line in (Path('/proc') / str(current) / 'status').read_text().splitlines():
                if line.startswith('VmRSS:'):
                    rss = max(rss, int(line.split()[1]) * 1024)
            fds = max(fds, len(list((Path('/proc') / str(current) / 'fd').iterdir())))
        except OSError:
            pass
    return rss, fds


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--candidate', type=Path, default=ROOT / '.obj/ck13/linux_candidate/app')
    parser.add_argument('--work', type=Path, default=ROOT / '.obj/ck13/linux_work')
    parser.add_argument('--output', type=Path, default=ROOT / '.obj/ck13/linux_endurance/results.json')
    args = parser.parse_args()
    work = args.work
    work.mkdir(parents=True, exist_ok=True)
    cases = [('stereo16.wav', 'stereo', 48000, 'pcm_s16le', '2'),
             ('mono32.wav', 'mono', 44100, 'pcm_s32le', '1'),
             ('surround24.wav', '5.1', 48000, 'pcm_s24le', '6'),
             ('surround71.flac', '7.1', 48000, 'flac', '8'),
             ('stereo.opus', 'stereo', 48000, 'libopus', '2'),
             ('long.wav', 'stereo', 48000, 'pcm_s16le', '2')]
    for name, layout, rate, codec, _ in cases:
        duration = '30' if name == 'long.wav' else '2'
        subprocess.run(['ffmpeg', '-v', 'error', '-y', '-f', 'lavfi', '-i',
                        f'anullsrc=sample_rate={rate}:channel_layout={layout}',
                        '-t', duration, '-c:a', codec, str(work / name)], check=True)
    config = work / 'config'
    config.mkdir(exist_ok=True)
    (config / 'rc').write_text('set mpris=false\nset softvol=true\nset softvol_state=0 0\n'
                               'set resume=false\nset repeat=false\nset output_plugin=pulse\n')
    checks = []

    def check(label, value):
        checks.append({'check': label, 'passed': bool(value)})

    fixtures = [name for name, *_ in cases if name != 'long.wav']
    session = Session(args.candidate / 'cmus', args.candidate / 'cmus-remote', config)
    try:
        session.start()
        baseline_rss, baseline_fds = memory(session.process.pid)
        for cycle in range(60):
            name = fixtures[cycle % len(fixtures)]
            session.client('-f', str(work / name))
            ok, text = wait_status(session, 'status playing\n')
            if not ok:
                check(f'cycle {cycle} starts', False)
                break
            session.command('player-pause')
            session.command('seek +1')
            session.command('player-pause')
            session.command('toggle repeat')
            session.command('player-stop')
            if cycle % 10 == 0:
                rss, fds = memory(session.process.pid)
                checks.append({'check': f'cycle {cycle} resource sample',
                               'passed': True, 'rss': rss, 'fds': fds})
        else:
            check('60 mixed-format playback cycles', True)
        final_rss, final_fds = memory(session.process.pid)
        check('memory growth bounded', final_rss > 0 and final_rss - baseline_rss < 48 * 1024 * 1024)
        check('descriptor growth bounded', final_fds <= baseline_fds + 6)

        session.client('-f', str(work / 'long.wav'))
        ok, _ = wait_status(session, 'status playing\n')
        check('long fixture playback', ok)
        with open(work / 'long.wav', 'wb'):
            pass
        time.sleep(.8)
        try:
            status = session.command('status')
            responsive = 'status ' in status
        except AssertionError:
            responsive = False
        check('player responsive after truncated input', responsive)
        session.command('player-stop')

        for _ in range(40):
            session.command('toggle repeat')
        try:
            responsive = 'status ' in session.command('status')
        except AssertionError:
            responsive = False
        check('rapid command storm leaves player responsive', responsive)
        session.stop()
        check('player clean exit', session.process.returncode == 0)
    finally:
        if session.process.poll() is None:
            session.stop()
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(json.dumps({'passed': all(c['passed'] for c in checks),
                                           'checks': checks}, indent=2) + '\n')
    print(json.dumps({'passed': all(c['passed'] for c in checks)}, indent=2))
    return 0 if all(c['passed'] for c in checks) else 1


if __name__ == '__main__':
    raise SystemExit(main())

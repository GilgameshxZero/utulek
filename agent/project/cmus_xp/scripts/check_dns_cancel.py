#!/usr/bin/env python3
"""Delay only the isolated player's DNS lookup; verify Stop and Quit bounds."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import time
import uuid
from linux_test_session import Session

ROOT = Path(__file__).resolve().parent.parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--candidate', type=Path, required=True)
    parser.add_argument('--asan', action='store_true')
    args = parser.parse_args()
    out = ROOT / '.obj/multichannel' / ('dns_' + uuid.uuid4().hex[:10])
    config = out / 'config'
    config.mkdir(parents=True)
    (config / 'rc').write_text('set mpris=false\nset output_plugin=pulse\nset resume=false\n')
    preload = out / 'delay_dns.so'
    subprocess.run(['gcc', '-std=gnu11', '-shared', '-fPIC', str(ROOT / 'tests/delay_dns.c'),
        '-ldl', '-o', str(preload)], check=True)
    environment = dict(os.environ)
    os.environ['LD_PRELOAD'] = str(preload)
    if args.asan:
        library = subprocess.check_output(['gcc', '-print-file-name=libasan.so'], text=True).strip()
        os.environ['LD_PRELOAD'] = library + ':' + str(preload)
    marker = out / 'entered.txt'
    os.environ['CMUS_DNS_MARKER'] = str(marker)
    for variable in ('http_proxy', 'https_proxy', 'HTTP_PROXY', 'HTTPS_PROXY', 'ALL_PROXY', 'all_proxy'):
        os.environ.pop(variable, None)
    os.environ['no_proxy'] = '*'
    session = Session(args.candidate / 'cmus', args.candidate / 'cmus-remote', config)
    report = {'passed': False}
    try:
        session.start()
        session.command('player-play http://cmus-delay-one.invalid/audio.mp3')
        deadline = time.monotonic() + 3
        while not marker.exists() and time.monotonic() < deadline:
            session.drain(.02)
        assert marker.exists(), 'Delayed resolver was not entered'
        start = time.monotonic()
        session.command('player-stop')
        report['stop_seconds'] = time.monotonic() - start
        report['stop_responsive'] = report['stop_seconds'] < 1.5
        # New lookups may queue behind the bounded pool; cancellation must
        # discard them and process exit must not join the delayed resolver.
        if report['stop_responsive']:
            for attempt in range(5):
                session.command('player-play http://cmus-delay-' + str(attempt) + '.invalid/audio.mp3')
                session.command('player-stop')
        start = time.monotonic()
        session.stop()
        report['quit_seconds'] = time.monotonic() - start
        report['quit_responsive'] = report['quit_seconds'] < 2
        report['resolver_calls'] = len(marker.read_text().splitlines())
        report['pool_bounded'] = report['resolver_calls'] <= 2
        report['passed'] = report['stop_responsive'] and report['quit_responsive'] and report['pool_bounded']
    finally:
        if hasattr(session, 'process') and session.process.poll() is None:
            session.stop()
        os.environ.clear()
        os.environ.update(environment)
        (out / 'screen.txt').write_bytes(session.screen)
        (out / 'results.json').write_text(json.dumps(report, indent=2) + '\n')
    print(out / 'results.json')
    print(json.dumps(report))
    assert report['passed'], report


if __name__ == '__main__':
    main()

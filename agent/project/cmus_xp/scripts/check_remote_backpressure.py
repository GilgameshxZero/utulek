#!/usr/bin/env python3
"""Verify complete large stdout exports and UI responsiveness to a slow reader.

Reuses supplied distinct-file fixtures; never creates another large collection.
"""
import argparse
import json
from pathlib import Path
import socket
import time

from linux_test_session import Session


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', type=Path, required=True)
    parser.add_argument('--remote', type=Path, required=True)
    parser.add_argument('--fixtures', type=Path, required=True)
    parser.add_argument('--count', type=int, default=50000)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    out = args.output.resolve()
    (out / 'config').mkdir(parents=True, exist_ok=True)
    (out / 'config/rc').write_text('set mpris=false\nset softvol=true\n')
    session = Session(args.binary, args.remote, out / 'config')
    checks = []
    report = {'passed': False, 'checks': checks}
    try:
        session.start()
        session.command('add -l ' + str(args.fixtures.resolve()))
        end = time.monotonic() + 60
        while time.monotonic() < end:
            target = out / 'library.pl'
            try:
                session.command('save -l ' + str(target))
            except AssertionError as error:
                if "can't save when tracks are being added" not in str(error): raise
                session.drain(.1)
                continue
            if sum(1 for _ in target.open()) == args.count:
                break
            session.drain(.1)
        else:
            raise AssertionError('Fixture import did not finish')
        answer = session.command('save -l -')
        assert len(answer.splitlines()) == args.count
        checks.append('complete_%d_file_remote_stdout_export' % args.count)
        report['export_bytes'] = len(answer.encode())
        with socket.socket(socket.AF_UNIX) as slow:
            slow.setsockopt(socket.SOL_SOCKET, socket.SO_RCVBUF, 4096)
            slow.connect(str(session.socket))
            slow.sendall(b'save -l -\n')
            session.drain(.2)
            start = time.monotonic()
            session.keys(':set repeat=true\r')
            assert 'set repeat true\n' in session.command('status')
            report['slow_reader_other_client_seconds'] = time.monotonic() - start
            assert report['slow_reader_other_client_seconds'] < 2
            checks.append('unread_large_response_does_not_stall_keyboard_or_other_client')
        session.drain(.1)
        assert 'status stopped\n' in session.command('status')
        checks.append('disconnected_slow_client_does_not_kill_server')
        session.stop()
        report['passed'] = True
    finally:
        try:
            if hasattr(session, 'process') and session.process.poll() is None:
                session.stop()
        finally:
            (out / 'results.json').write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()

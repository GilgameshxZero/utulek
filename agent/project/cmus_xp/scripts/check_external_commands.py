#!/usr/bin/env python3
"""Exercise :shell and :run through the real Linux player terminal."""
import json
import os
from pathlib import Path
import shutil
import tempfile
import wave

from linux_test_session import Session

ROOT = Path(__file__).resolve().parent.parent


def make_wav(path):
    with wave.open(str(path), 'wb') as wav:
        wav.setnchannels(1)
        wav.setsampwidth(2)
        wav.setframerate(8000)
        wav.writeframes(b'\0\0' * 800)


def main():
    import argparse
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path,
                        default=ROOT / '.obj/external_commands/results.json')
    parser.add_argument('--candidate', type=Path,
                        default=ROOT / '.obj/ck11/linux_candidate/app')
    args = parser.parse_args()
    base = Path(tempfile.mkdtemp(prefix='cmus-external-'))
    config = base / 'config'
    config.mkdir()
    wav = base / 'track.wav'
    make_wav(wav)
    out = base / 'argv.txt'
    unicode_path = base / 'ü space file'
    helper = base / 'helper.sh'
    helper.write_text('#!/bin/sh\nprintf \'%s\\n\' "$@" > "$1"\n')
    helper.chmod(0o755)
    failing = base / 'failing.sh'
    failing.write_text('#!/bin/sh\nexit 3\n')
    failing.chmod(0o755)
    checks = []

    def record(name, condition, detail=''):
        checks.append({'name': name, 'passed': bool(condition), 'detail': detail})

    session = Session(args.candidate / 'cmus', args.candidate / 'cmus-remote', config)
    try:
        session.start()
        session.command('add -l ' + str(wav))
        session.command('set confirm_run=false')
        session.keys(':shell printf shell-ok > ' + str(base / 'shell.txt') + '\n')
        session.drain(.4)
        record('shell output', (base / 'shell.txt').read_text() == 'shell-ok'
               if (base / 'shell.txt').is_file() else False)
        session.keys(':shell touch "' + str(unicode_path) + '"\n')
        session.drain(.4)
        record('shell unicode path', unicode_path.is_file())

        session.keys(':run ' + str(helper) + ' ' + str(out) + '\n')
        session.drain(.8)
        argv = out.read_text().splitlines() if out.is_file() else []
        record('run arguments', argv[:2] == [str(out), str(wav)], repr(argv))

        session.keys(':run ' + str(failing) + '\n')
        session.drain(.8)
        screen = session.screen.decode(errors='replace')
        record('run nonzero exit reported', 'returned 3' in screen, screen[-500:])

        session.keys(':run /no/such/cmus-command\n')
        session.drain(.8)
        screen = session.screen.decode(errors='replace')
        record('run missing executable reported',
               'executing /no/such/cmus-command' in screen, screen[-500:])
    finally:
        session.stop()
        shutil.rmtree(base, ignore_errors=True)

    result = {'passed': all(c['passed'] for c in checks), 'checks': checks}
    destination = args.output
    destination.parent.mkdir(parents=True, exist_ok=True)
    destination.write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps(result, indent=2))
    return 0 if result['passed'] else 1


if __name__ == '__main__':
    raise SystemExit(main())

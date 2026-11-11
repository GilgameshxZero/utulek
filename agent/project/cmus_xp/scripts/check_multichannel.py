#!/usr/bin/env python3
"""Validate native local PCM, format switches and Windows device selection."""
import argparse
import json
from pathlib import Path
import subprocess
import uuid
from linux_test_session import Session
from windows_test_stage import WindowsStage, ps_quote

ROOT = Path(__file__).resolve().parent.parent
CASES = [('stereo16.wav', 'stereo', 48000, 'pcm_s16le', 2, 16),
    ('mono32.wav', 'mono', 44100, 'pcm_s32le', 1, 32),
    ('surround8.wav', '5.1', 48000, 'pcm_u8', 6, 8),
    ('surround16.wav', '5.1', 48000, 'pcm_s16le', 6, 16),
    ('surround24.wav', '5.1', 48000, 'pcm_s24le', 6, 24),
    ('surround32.wav', '5.1', 48000, 'pcm_s32le', 6, 32),
    ('surround71.flac', '7.1', 48000, 'flac', 8, 16)]


def linux(candidate, root, out):
    config = root / 'config'
    config.mkdir()
    (config / 'rc').write_text('set mpris=false\nset softvol=true\nset softvol_state=0 0\nset resume=false\nset output_plugin=pulse\n')
    session = Session(candidate / 'cmus', candidate / 'cmus-remote', config)
    checks, passed = [], False

    def check(label, value):
        checks.append({'check': label, 'passed': bool(value)})
        assert value, label

    def wait(channels, bits, rate):
        for _ in range(80):
            text = session.command('status')
            fields = dict(line.split(' ', 1) for line in text.splitlines() if ' ' in line)
            if (fields.get('status') == 'playing' and fields.get('output_channels') == str(channels) and
                    fields.get('output_bits') == str(bits) and fields.get('output_sample_rate') == str(rate) and
                    int(fields.get('position', -1)) >= 1):
                return text
            session.drain(.05)
        raise AssertionError('Native format did not start: ' + text)

    try:
        session.start()
        for name, _, rate, _, channels, bits in [*CASES, CASES[0]]:
            session.client('-f', str(root / name))
            wait(channels, bits, rate)
            check(name + ' native format and advancing playback', True)
            session.command('player-pause')
            check(name + ' pause', 'status paused\n' in session.command('status'))
            session.command('seek +4', 'player-pause')
            check(name + ' seek/resume', 'status playing\n' in session.command('status'))
            session.command('player-stop')
        session.stop()
        passed = True
    finally:
        if hasattr(session, 'process') and session.process.poll() is None:
            session.stop()
        (out / 'screen.txt').write_bytes(session.screen)
        (out / 'results.json').write_text(json.dumps({'passed': passed, 'checks': checks}, indent=2) + '\n')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--candidate', type=Path, required=True)
    parser.add_argument('--host')
    parser.add_argument('--harness', type=Path)
    args = parser.parse_args()
    out = ROOT / '.obj/multichannel' / ('native_' + uuid.uuid4().hex[:10])
    root = out / 'work 音'
    root.mkdir(parents=True)
    for name, layout, rate, codec, _, _ in CASES:
        subprocess.run(['ffmpeg', '-v', 'error', '-y', '-f', 'lavfi', '-i',
            'anullsrc=sample_rate=' + str(rate) + ':channel_layout=' + layout,
            '-t', '30', '-c:a', codec, str(root / name)], check=True)
    if not args.host:
        linux(args.candidate.resolve(), root, out)
    else:
        if not args.harness:
            parser.error('--harness required with --host')
        files = {str(p.relative_to(out)): p for p in root.rglob('*') if p.is_file()}
        files.update({name: args.candidate / name for name in ('cmus.exe', 'cmus-remote.exe')})
        files['check_multichannel.exe'] = args.harness
        with WindowsStage(args.host, files, out) as stage:
            listing = stage.powershell('$devices=& ' + ps_quote(stage.path + '/cmus.exe') +
                ' --list-output-devices; if($LASTEXITCODE){exit $LASTEXITCODE};' +
                '[IO.File]::WriteAllText(' + ps_quote(stage.path + '/devices.txt') +
                ',($devices -join "`n"),[Text.UTF8Encoding]::new($false))')
            listing.check_returncode()
            stage.retrieve('devices.txt', out / 'devices.txt')
            result = stage.run('check_multichannel.exe', [stage.path + '/cmus.exe', stage.path + '/cmus-remote.exe',
                stage.path + '/work 音', stage.path + '/report.txt'])
            stage.retrieve('report.txt', out / 'report.txt')
        report = (out / 'report.txt').read_text()
        (out / 'results.json').write_text(json.dumps({'passed': result.returncode == 0 and 'FAIL' not in report,
            'exit': result.returncode, 'report': report, 'cleanup': json.loads((out / 'cleanup.json').read_text())}, indent=2) + '\n')
        print(report)
        result.check_returncode()
    print(out / 'results.json')


if __name__ == '__main__':
    main()

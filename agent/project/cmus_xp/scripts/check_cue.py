#!/usr/bin/env python3
"""Actual Linux/Windows UI, imports, transport and persistence CUE checks."""
import argparse
import json
from pathlib import Path
import subprocess
import time
import uuid
import wave
from linux_test_session import Session
from windows_test_stage import WindowsStage

ROOT = Path(__file__).resolve().parent.parent


def fixtures(root):
    for directory in ('disc', 'browser', 'compressed'):
        (root / directory).mkdir(parents=True)
    audio = root / 'disc/audio ü.wav'
    with wave.open(str(audio), 'wb') as f:
        f.setparams((1, 1, 8000, 0, 'NONE', 'not compressed'))
        f.writeframes(bytes([128]) * 8000 * 30)
    template = ('\ufeffTITLE "Album"\r\nPERFORMER "Artist"\r\nFILE "FILE_NAME" WAVE\r\n'
        'TRACK 01 AUDIO\r\nTITLE "First"\r\nINDEX 01 00:00:00\r\n'
        'TRACK 02 AUDIO\r\nTITLE "Second"\r\nINDEX 01 00:10:00\r\n')
    (root / 'disc/album.CUE').write_text(template.replace('FILE_NAME', audio.name))
    (root / 'browser/album.cue').write_text(template.replace('FILE_NAME', '../disc/' + audio.name))
    subprocess.run(['ffmpeg', '-v', 'error', '-y', '-i', str(audio),
                    str(root / 'compressed/audio.flac')], check=True)
    (root / 'compressed/album.cue').write_text(template.replace('FILE_NAME', 'audio.flac'))


def linux(candidate, root, out):
    config = root / 'config'
    config.mkdir()
    (config / 'rc').write_text('set mpris=false\nset softvol=true\nset softvol_state=0 0\nset resume=false\nset output_plugin=pulse\n')
    session = Session(candidate / 'cmus', candidate / 'cmus-remote', config)
    checks, passed = [], False

    def check(name, value):
        checks.append({'check': name, 'passed': bool(value)})
        assert value, name

    def library():
        return session.command('save -l -').splitlines()

    def imported():
        for _ in range(40):
            entries = library()
            if len(entries) == 2:
                return entries
            session.drain(.05)
        return entries

    def wait(needle):
        for _ in range(60):
            text = session.command('status')
            if needle in text:
                return text
            session.drain(.05)
        raise AssertionError(needle + ': ' + text)

    try:
        session.start()
        session.keys(':add ' + str(root / 'disc/album.CUE') + '\r')
        check('typed uppercase CUE import', len(imported()) == 2)
        session.command('clear -l', 'cd ' + str(root / 'browser'), 'view 5', 'win-top', 'win-down')
        session.keys('a')
        check('browser a imports individual CUE', len(imported()) == 2)
        session.command('clear -l', 'add -l ' + str(root / 'disc'))
        entries = imported()
        check('directory import suppresses backing audio duplicate', len(entries) == 2 and all(x.startswith('cue://') for x in entries))
        for flag in ('p', 'q'):
            session.command('add -' + flag + ' ' + str(root / 'disc/album.CUE'))
            for _ in range(40):
                imported_entries = session.command('save -' + flag + ' -').splitlines()
                if len(imported_entries) == 2:
                    break
                session.drain(.05)
            check('CUE import through -' + flag, len(imported_entries) == 2 and all(x.startswith('cue://') for x in imported_entries))
        session.command('clear -q')
        session.client('-f', 'cue://' + str(root / 'disc/album.CUE') + '/2')
        text = wait('tag title Second\n')
        check('CUE track playback/metadata and duration', 'status playing\n' in text and 'duration 20\n' in text)
        session.command('player-pause')
        wait('status paused\n')
        session.command('seek +4')
        text = session.command('status')
        fields = dict(line.split(' ', 1) for line in text.splitlines() if ' ' in line)
        check('CUE paused seek relative to track', 4 <= int(fields['position']) < 8)
        session.command('player-pause')
        wait('status playing\n')
        check('CUE pause/resume', True)
        session.command('player-stop', 'player-play cue://' + str(root / 'disc/album.CUE') + '/1')
        text = wait('tag title First\n')
        check('first track boundary duration', 'duration 10\n' in text)
        session.command('view 2', 'win-top')
        session.keys('\r')
        wait('tag title First\n')
        session.command('seek 9')
        text = wait('tag title Second\n')
        check('Enter playback advances across CUE boundary', 'status playing\n' in text)
        session.command('player-stop', 'player-play cue://' + str(root / 'compressed/album.cue') + '/2')
        text = wait('tag title Second\n')
        check('FLAC-backed CUE playback', 'status playing\n' in text)
        session.command('player-stop')
        session.stop()
        (out / 'screen.txt').write_bytes(session.screen)
        session = Session(candidate / 'cmus', candidate / 'cmus-remote', config).start()
        check('CUE library restart persistence', imported() == entries)
        session.keys(':player-play cue://' + str(root / 'disc/album.CUE') + '/1\r')
        text = wait('tag title First\n')
        check('typed CUE playback after restart', 'status playing\n' in text)
        session.command('player-stop')
        session.stop()
        passed = True
    finally:
        if hasattr(session, 'process') and session.process.poll() is None:
            session.stop()
        (out / 'screen_final.txt').write_bytes(session.screen)
        (out / 'results.json').write_text(json.dumps({'passed': passed, 'checks': checks}, indent=2) + '\n')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--candidate', type=Path, required=True)
    parser.add_argument('--host', help='Explicitly authorized established Windows test host')
    parser.add_argument('--harness', type=Path)
    args = parser.parse_args()
    out = ROOT / '.obj/cue' / ('ui_' + uuid.uuid4().hex[:10])
    root = out / 'work 音'
    root.mkdir(parents=True)
    fixtures(root)
    if not args.host:
        linux(args.candidate.resolve(), root, out)
    else:
        if not args.harness:
            parser.error('--harness required with --host')
        files = {str(p.relative_to(out)): p for p in root.rglob('*') if p.is_file()}
        files.update({name: args.candidate / name for name in ('cmus.exe', 'cmus-remote.exe')})
        files['check_cue.exe'] = args.harness
        result = None
        with WindowsStage(args.host, files, out) as stage:
            result = stage.run('check_cue.exe', [stage.path + '/cmus.exe', stage.path + '/cmus-remote.exe',
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

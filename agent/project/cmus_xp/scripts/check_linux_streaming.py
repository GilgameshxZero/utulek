"""Actual Linux player checks using check_windows_streaming's shared fixtures."""
import json
import os
import subprocess
from pathlib import Path
import time
from linux_test_session import Session


def run_checks(candidate, out, http_port, tls_port):
    results = []
    config = out / 'config'
    config.mkdir()
    (config / 'rc').write_text('set softvol=true\nset resume=false\nset output_plugin=pulse\n')
    session = Session(candidate / 'cmus', candidate / 'cmus-remote', config)

    def check(label, assertion):
        results.append({'check': label, 'passed': bool(assertion)})
        assert assertion, label

    def wait(state, position=-1):
        for _ in range(60):
            text = session.command('status')
            fields = dict(line.split(' ', 1) for line in text.splitlines() if ' ' in line)
            if fields.get('status') == state and int(fields.get('position', -1)) >= position:
                return text
            session.drain(.05)
        raise AssertionError('Expected ' + state + ': ' + text)

    def play(path, tls=False):
        url = ('https' if tls else 'http') + '://127.0.0.1:' + str(tls_port if tls else http_port) + path
        start = time.monotonic()
        session.command('player-play ' + url)
        check('play request responsive: ' + path, time.monotonic() - start < 1.5)

    passed = False
    try:
        session.start()
        for path in ('/audio.mp3', '/audio.opus', '/audio.flac', '/redirect', '/chunked.mp3', '/list.m3u8', '/list.pls', '/icy.mp3', '/legacy_icy.mp3', '/mono44.mp3', '/surround51.flac', '/surround71.flac', '/audio.aac', '/audio.ogg'):
            play(path)
            text = wait('playing', 1)
            check('playback advances: ' + path, True)
            if path in ('/mono44.mp3', '/surround51.flac', '/surround71.flac'):
                channels = {'/mono44.mp3': 1, '/surround51.flac': 6, '/surround71.flac': 8}[path]
                rate = 44100 if channels == 1 else 48000
                check('native decoded format: ' + path, 'output_channels ' + str(channels) + '\n' in text and
                    'output_sample_rate ' + str(rate) + '\n' in text)
            if path == '/audio.mp3':
                check('embedded title', 'tag title Network tone\n' in text)
            if path == '/icy.mp3':
                check('ICY title', 'stream Fixture artist - Fixture title\n' in text)
                check('station genre', 'tag genre Test genre\n' in text)
            start = time.monotonic()
            session.command('player-stop')
            wait('stopped')
            check('stop responsive: ' + path, time.monotonic() - start < 1.5)
        for path, channels in (('/surround51.flac', 6), ('/mono44.mp3', 1), ('/surround71.flac', 8), ('/audio.mp3', 2)):
            play(path)
            text = wait('playing', 1)
            check('active stream format switch: ' + path, 'output_channels ' + str(channels) + '\n' in text)
        session.command('player-stop')
        for attempt in range(10):
            play('/surround51.flac' if attempt % 2 else '/mono44.mp3')
            session.command('set mixer.pulse.restore_volume=true', 'player-stop')
            check('format opening/option/cancel stress ' + str(attempt), 'status stopped\n' in session.command('status'))
        for path in ('/stall_headers', '/stall_body'):
            play(path)
            start = time.monotonic()
            check('remote responsive during ' + path, 'responsive' in session.command('echo responsive'))
            session.command('player-stop')
            wait('stopped')
            check('cancellation bounded: ' + path, time.monotonic() - start < 1.5)
        for path in ('/missing', '/redirect_loop', '/huge.m3u', '/nested.m3u', '/hls.m3u8'):
            play(path)
            wait('stopped')
            check('clean failure: ' + path, True)
        play('/audio.mp3', tls=True)
        wait('stopped')
        check('untrusted certificate rejected', True)
        session.stop()
        (out / 'screen.txt').write_bytes(session.screen)
        # Trust only this fixture certificate in the isolated player's process.
        previous = os.environ.get('SSL_CERT_FILE')
        os.environ['SSL_CERT_FILE'] = str(out / 'test_cert.pem')
        try:
            session = Session(candidate / 'cmus', candidate / 'cmus-remote', config).start()
        finally:
            if previous is None:
                os.environ.pop('SSL_CERT_FILE', None)
            else:
                os.environ['SSL_CERT_FILE'] = previous
        play('/audio.mp3', tls=True)
        wait('playing', 1)
        check('verified HTTPS playback with explicit CA', True)
        session.command('player-stop')
        session.keys(':player-play http://127.0.0.1:' + str(http_port) + '/audio.mp3\r')
        wait('playing', 1)
        check('typed playback command shares streaming behavior', True)
        session.command('player-pause')
        wait('stopped')
        check('radio pause follows upstream stop', True)
        play('/stall_headers')
        start = time.monotonic()
        session.stop()
        check('quit cancels pending request', time.monotonic() - start < 2)
        passed = True
    except Exception:
        try:
            pid = session.process.pid
            children = Path('/proc') / str(pid) / 'task' / str(pid) / 'children'
            nested = children.read_text().split()
            if nested:
                pid = int(nested[0])
            trace = subprocess.run(['gdb', '-batch', '-p', str(pid), '-ex', 'thread apply all bt'],
                capture_output=True, text=True, timeout=10)
            (out / 'failure_stacks.txt').write_text(trace.stdout + trace.stderr)
        except Exception as error:
            (out / 'diagnostic_error.txt').write_text(str(error))
        raise
    finally:
        if hasattr(session, 'process') and session.process.poll() is None:
            try:
                session.stop()
            except Exception:
                pass
        (out / 'screen_final.txt').write_bytes(session.screen)
        (out / 'results.json').write_text(json.dumps({'passed': passed, 'checks': results}, indent=2) + '\n')

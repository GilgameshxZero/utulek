#!/usr/bin/env python3
"""Compare Linux plugins/startup and controls using isolated settings and D-Bus.

Run under dbus-run-session with host audio/socket access. No user settings are
written. Test audio is quiet; only the test player's own process is controlled.
"""
import argparse
import fcntl
import functools
import http.server
import json
import os
from pathlib import Path
import pty
import re
import select
import shutil
import struct
import subprocess
import termios
import threading
import time

ROOT = Path(__file__).resolve().parent.parent


def plugins(binary):
    text = subprocess.check_output([str(binary), '--plugins'], text=True)
    return set(re.findall(r'^  (\w+):?$', text, re.MULTILINE))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', type=Path, required=True)
    parser.add_argument('--remote', type=Path, required=True)
    parser.add_argument('--reference', type=Path, default=Path('/usr/bin/cmus'))
    parser.add_argument('--settings', type=Path, default=Path.home() / '.config/cmus/autosave')
    args = parser.parse_args()
    out = ROOT / '.obj/linux_parity'
    out.mkdir(parents=True, exist_ok=True)
    expected, actual = plugins(args.reference), plugins(args.binary)
    assert expected <= actual, ('Missing plugins', sorted(expected - actual))
    fixture = out / 'quiet track ü.mp3'
    subprocess.run(['ffmpeg', '-v', 'error', '-y', '-f', 'lavfi', '-i',
                    'sine=frequency=440:duration=20', '-af', 'volume=0.01',
                    '-metadata', 'title=Parity tone', str(fixture)], check=True)
    class Handler(http.server.SimpleHTTPRequestHandler):
        def log_message(self, *args):
            pass
    server = http.server.ThreadingHTTPServer(('127.0.0.1', 0),
        functools.partial(Handler, directory=str(out)))
    threading.Thread(target=server.serve_forever, daemon=True).start()
    results = []
    try:
        for label, binary, remote in [('system', args.reference, Path('/usr/bin/cmus-remote')),
                                      ('port', args.binary, args.remote)]:
            config = out / (label + '_validation_' + str(os.getpid()))
            config.mkdir(exist_ok=True)
            shutil.copy2(args.settings, config / 'autosave')
            # Keep the copied plugin options/bindings, but never resume a user's
            # track or change a system mixer while testing.
            (config / 'rc').write_text('set resume=false\nset softvol=true\n'
                                      'set softvol_state=100 100\nset mpris=true\n')
            socket = Path('/tmp/cmus-parity-%d.sock' % os.getpid())
            master, slave = pty.openpty()
            fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 30, 120, 0, 0))
            process = subprocess.Popen([str(binary), '--listen', str(socket)],
                stdin=slave, stdout=slave, stderr=slave,
                env=dict(os.environ, TERM='xterm-256color', CMUS_HOME=str(config)))
            os.close(slave)
            screen = bytearray()
            def wait(seconds=.2):
                end = time.monotonic() + seconds
                while time.monotonic() < end:
                    if select.select([master], [], [], .02)[0]:
                        try:
                            screen.extend(os.read(master, 65536))
                        except OSError:
                            break
            def command(*commands):
                value = subprocess.check_output([str(remote), '--server', str(socket),
                    '-C', *commands], text=True, timeout=5)
                assert 'Error:' not in value, value
                return value
            def status():
                return dict(line.split(' ', 1) for line in command('status').splitlines()
                            if ' ' in line)
            def mpris(*arguments):
                return subprocess.check_output(['busctl', '--user', *arguments],
                                                text=True, timeout=5)
            service, path = 'org.mpris.MediaPlayer2.cmus', '/org/mpris/MediaPlayer2'
            interface = 'org.mpris.MediaPlayer2.Player'
            try:
                wait(1)
                assert process.poll() is None
                assert b'Error:' not in screen and b'Press <enter>' not in screen, screen[:4000]
                assert 'Stopped' in mpris('get-property', service, path, interface, 'PlaybackStatus')
                command('add -l ' + str(fixture), 'add -q ' + str(fixture),
                        'add -p ' + str(fixture))
                wait(.5)
                assert str(fixture) in command('save -l -')
                assert str(fixture) in command('save -q -')
                assert str(fixture) in command('save -p -')
                command('player-play ' + str(fixture))
                wait(.5)
                assert status()['status'] == 'playing'
                assert 'Parity tone' in mpris('get-property', service, path, interface, 'Metadata')
                mpris('call', service, path, interface, 'Pause')
                wait()
                assert status()['status'] == 'paused'
                command('seek +5')
                wait()
                assert int(status()['position']) >= 5
                mpris('call', service, path, interface, 'Play')
                wait()
                assert status()['status'] == 'playing'
                command('vol 50%', 'set replaygain=track', 'toggle repeat', 'toggle shuffle')
                command('player-stop')
                assert status()['status'] == 'stopped'
                # Native MP3 streaming is part of Linux cmus, even though the
                # optional static FFmpeg build intentionally has no networking.
                from urllib.parse import quote
                url = 'http://127.0.0.1:%d/%s' % (server.server_port, quote(fixture.name))
                command('player-play ' + url)
                wait(.7)
                assert status()['status'] == 'playing', status()
                command('player-stop', 'quit')
                process.wait(timeout=5)
                assert process.returncode == 0
                assert str(fixture) in (config / 'lib.pl').read_text()
                assert 'set dsp.jack.resampling_quality=' in (config / 'autosave').read_text()
                # Reopen the saved library and playlist, rather than checking
                # only that a save file was written.
                master2, slave2 = pty.openpty()
                fcntl.ioctl(slave2, termios.TIOCSWINSZ, struct.pack('HHHH', 30, 120, 0, 0))
                os.close(master)
                master = master2
                process = subprocess.Popen([str(binary), '--listen', str(socket)],
                    stdin=slave2, stdout=slave2, stderr=slave2,
                    env=dict(os.environ, TERM='xterm-256color', CMUS_HOME=str(config)))
                os.close(slave2)
                wait(1)
                assert str(fixture) in command('save -l -')
                assert str(fixture) in command('save -p -')
                assert b'Error:' not in screen and b'Press <enter>' not in screen
                command('quit')
                process.wait(timeout=5)
                assert process.returncode == 0
                results.append({'binary': str(binary), 'passed': True,
                    'checks': ['copied_settings_startup', 'remote', 'library', 'playlist', 'queue',
                               'unicode_path', 'mpris_metadata_pause_play', 'seek',
                               'volume_command', 'replaygain_repeat_shuffle_options',
                               'http_mp3', 'settings_library_playlist_restart', 'clean_exit']})
                print(label + ': Linux feature checks PASS', flush=True)
            finally:
                if process.poll() is None:
                    process.terminate()
                    process.wait(timeout=5)
                wait(.05)
                os.close(master)
                socket.unlink(missing_ok=True)
                (out / (label + '_validated_screen.txt')).write_bytes(screen)
    finally:
        server.shutdown()
        server.server_close()
        (out / 'results.json').write_text(json.dumps({'expected_plugins': sorted(expected),
            'actual_plugins': sorted(actual), 'results': results}, indent=2) + '\n')


if __name__ == '__main__':
    main()

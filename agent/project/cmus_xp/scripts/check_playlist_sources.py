#!/usr/bin/env python3
"""Exercise the Linux application with isolated, mutable playlist fixtures."""
import argparse
import fcntl
import json
import os
from pathlib import Path
import pty
import select
import struct
import subprocess
import termios
import threading
import time
import wave

ROOT = Path(__file__).resolve().parent.parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', type=Path, required=True)
    parser.add_argument('--remote', type=Path, required=True)
    args = parser.parse_args()
    out = ROOT / '.obj/playlist_sources' / ('run-' + str(os.getpid()))
    config, sources = out / 'config', out / 'browser_parent/sources'
    config.mkdir(parents=True)
    (sources / 'sub').mkdir(parents=True)
    (config / 'rc').write_text('set mpris=false\nset softvol=true\nset output_plugin=pulse\n')
    tracks = []
    for name in ['one ü.wav', 'two.wav']:
        path = sources / name
        with wave.open(str(path), 'wb') as f:
            f.setnchannels(2)
            f.setsampwidth(2)
            f.setframerate(8000)
            f.writeframes(b'\0' * 4 * 8000 * 20)
        tracks.append(str(path.resolve()))
    a = sources / 'a.m3u'
    a.write_bytes(b'\xef\xbb\xbf#EXTM3U\r\n#EXTINF:20,one\r\n' +
                  '  one ü.wav\r\ntwo.wav\r\n'.encode())
    (sources / 'sub/a.m3u').write_text('../two.wav\n')
    (sources / 'b.pls').write_bytes(('\ufeff[playlist]\r\nFile2=two.wav\r\nFile1=one ü.wav\r\nNumberOfEntries=2\r\nVersion=2\r\n').encode())
    (sources / 'c.pl').write_text('one ü.wav\none ü.wav\n')
    (sources / 'loop').symlink_to(sources.resolve(), target_is_directory=True)
    external = out / 'individual_parent/individual.m3u8'
    external.parent.mkdir()
    external.write_text(Path(tracks[0]).as_uri() + '\n')
    originals = {p: p.read_bytes() for p in sources.rglob('*') if p.is_file()}
    socket = Path('/tmp/cmus-sources-%d.sock' % os.getpid())
    process = None
    master = None
    screen = bytearray()
    checks = []
    def wait(seconds=.1):
        end = time.monotonic() + seconds
        while time.monotonic() < end:
            if select.select([master], [], [], .02)[0]:
                try:
                    screen.extend(os.read(master, 65536))
                except OSError:
                    break
    def command(*commands):
        text = subprocess.check_output([str(args.remote.resolve()), '--server', str(socket),
            '-C', *commands], text=True, timeout=8)
        assert 'Error:' not in text, text
        return text
    def start():
        nonlocal process, master
        master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 30, 120, 0, 0))
        process = subprocess.Popen([str(args.binary.resolve()), '--listen', str(socket)],
            stdin=slave, stdout=slave, stderr=slave,
            env=dict(os.environ, TERM='xterm-256color', CMUS_HOME=str(config.resolve()),
                     CMUS_PLAYLIST_EMPTY=''))
        os.close(slave)
        wait(.5)
        assert process.poll() is None
        command('view 3')
    def stop():
        command('quit')
        process.wait(timeout=8)
        assert process.returncode == 0, process.returncode
        wait()
        os.close(master)
    def snapshot():
        command('view 3', 'save')
        return {p.name: p.read_text().splitlines() for p in (config / 'playlists').iterdir()}
    def expect(name, expected):
        end = time.monotonic() + 8
        while time.monotonic() < end:
            actual = snapshot()
            if all(actual.get(k) == v for k, v in expected.items()):
                checks.append(name)
                return actual
            wait()
        raise AssertionError((name, actual, expected))
    try:
        start()
        # Exercise the actual default browser key, not just :add commands.
        command('cd ' + str(sources.parent.resolve()), 'view 5', 'win-top', 'win-down')
        os.write(master, b'a')
        wait(.2)
        expect('browser a on folder: formats, relative Unicode paths, PLS ordering, duplicates', {
            'a.m3u': tracks, 'a.m3u.1': [tracks[1]], 'b.pls': tracks,
            'c.pl': [tracks[0], tracks[0]]})
        command('view 1', 'add ' + str(a.resolve()),
                'add -p ' + str(sources.resolve()), 'pl-import ' + str(a.resolve()),
                'cd ' + str(external.parent.resolve()), 'view 5', 'win-top', 'win-down')
        os.write(master, b'a')
        wait(.2)
        actual = expect('browser a on playlist and cross-entry-point deduplication', {'individual.m3u8': [tracks[0]]})
        assert set(actual) == {'Default', 'a.m3u', 'a.m3u.1', 'b.pls', 'c.pl', 'individual.m3u8'}, actual
        assert all(p.read_bytes() == content for p, content in originals.items())
        checks.append('imports never modify source files')
        # Start through the playlist UI, then refresh while its track is active.
        command('view 3', 'win-top', 'win-down', 'win-activate')
        wait(.3)
        assert 'status playing' in command('status')
        a.write_text('two.wav\none ü.wav\ntwo.wav\n')
        (sources / 'b.pls').write_bytes(b'')
        for _ in range(12):
            command('update')
        expect('repeated update replaces order, keeps duplicates and clears empty files', {
            'a.m3u': [tracks[1], tracks[0], tracks[1]], 'b.pls': []})
        assert 'status playing' in command('status')
        checks.append('update preserves current audio playback')
        command('player-stop', 'pl-create Local')
        (sources / 'sub/a.m3u').unlink()
        (sources / 'new.M3U8').write_text('two.wav\n')
        command('view 1', 'update')
        actual = expect('directory discovers new files', {'new.M3U8': [tracks[1]]})
        assert 'a.m3u.1' not in actual and 'Local' in actual
        checks.append('deleted source removed, local playlist retained')
        stop()
        # The root association and per-file source must both survive restart.
        a.write_text('one ü.wav\n')
        (sources / 'after_restart.pls').write_text('[playlist]\nFile1=two.wav\n')
        external.unlink()
        start()
        command('update')
        actual = expect('source and directory associations persist across restart', {
            'a.m3u': [tracks[0]], 'after_restart.pls': [tracks[1]]})
        assert 'individual.m3u8' not in actual
        checks.append('missing individual source removed after restart')
        # Delete during a queued reload: old worker results must not resurrect it.
        a.write_text(('two.wav\n' * 10000))
        command('update', 'pl-delete a.m3u')
        wait(.3)
        assert process.poll() is None
        command('update')
        expect('deleted while loading can be rediscovered safely', {'a.m3u': [tracks[1]] * 10000})
        def edit_repeatedly():
            for i in range(40):
                a.write_text('one ü.wav\n' * (200 if i % 2 else 0))
                time.sleep(.005)
        editor = threading.Thread(target=edit_repeatedly)
        editor.start()
        for _ in range(20):
            command('update')
            wait(.005)
        editor.join()
        a.write_text('one ü.wav\n')
        command('update')
        expect('concurrent truncate and reload is safe', {'a.m3u': [tracks[0]]})
        stop()
        assert b'Error:' not in screen and b'Press <enter>' not in screen, screen[-3000:]
        print('Playlist sources: %d checks PASS' % len(checks))
    finally:
        if process and process.poll() is None:
            process.terminate()
            process.wait(timeout=5)
        socket.unlink(missing_ok=True)
        (out / 'screen.txt').write_bytes(screen)
        (out / 'results.json').write_text(json.dumps({'checks': checks}, indent=2) + '\n')
        print(out)


if __name__ == '__main__':
    main()

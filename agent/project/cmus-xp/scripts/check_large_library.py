#!/usr/bin/env python3
"""Exercise a real Linux cmus library with distinct synthetic WAV files."""
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
import time
import wave

ROOT = Path(__file__).resolve().parent.parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', type=Path, required=True)
    parser.add_argument('--remote', type=Path, required=True)
    parser.add_argument('--count', type=int, default=50000)
    args = parser.parse_args()
    assert args.count >= 2
    out = ROOT / '.obj/release_validation' / ('large_library-' + str(os.getpid()))
    music, config = out / 'music ü', out / 'config'
    music.mkdir(parents=True)
    config.mkdir()
    (config / 'rc').write_text('set mpris=false\nset softvol=true\n')
    seed = out / 'seed.wav'
    with wave.open(str(seed), 'wb') as f:
        f.setnchannels(2)
        f.setsampwidth(2)
        f.setframerate(8000)
        f.writeframes(bytes(320))
    start = time.monotonic()
    # Separate inodes exercise the real file cache, rather than deduplication.
    data = seed.read_bytes()
    for i in range(args.count):
        (music / ('track_%05d.wav' % i)).write_bytes(data)
    results = {'count': args.count, 'fixture_seconds': time.monotonic()-start, 'checks': []}
    socket = Path('/tmp/cmus-scale-%d.sock' % os.getpid())
    screen = bytearray()
    process, master = None, None

    def drain(seconds=.02):
        end = time.monotonic()+seconds
        while time.monotonic() < end:
            if select.select([master], [], [], .005)[0]:
                try:
                    screen.extend(os.read(master, 65536))
                except OSError:
                    break

    def command(*commands):
        client = subprocess.Popen([str(args.remote.resolve()), '--server', str(socket), '-C', *commands],
                                  stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        # communicate drains potentially multi-megabyte library exports.
        while True:
            try:
                output, errors = client.communicate(timeout=.02)
                break
            except subprocess.TimeoutExpired:
                drain()
                if time.monotonic()-command.start > 120:
                    client.kill()
                    client.communicate()
                    raise TimeoutError(commands)
        assert client.returncode == 0, (commands, output, errors)
        return output.decode()

    def cmd(*commands):
        command.start = time.monotonic()
        return command(*commands)

    def start_player():
        nonlocal master, process
        master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 30, 120, 0, 0))
        process = subprocess.Popen([str(args.binary.resolve()), '--listen', str(socket)],
            stdin=slave, stdout=slave, stderr=slave,
            env=dict(os.environ, TERM='xterm-256color', CMUS_HOME=str(config)))
        os.close(slave)
        deadline = time.monotonic()+20
        while not socket.exists():
            drain()
            assert process.poll() is None and time.monotonic() < deadline

    def stop():
        cmd('quit')
        process.wait(timeout=15)
        assert process.returncode == 0
        os.close(master)

    def expect_count(count):
        deadline = time.monotonic()+120
        while time.monotonic() < deadline:
            lines = cmd('save -l -').splitlines()
            if lines == ["Error: can't save when tracks are being added"]:
                drain(.1)
                continue
            if len(lines) == count:
                assert len(set(lines)) == count
                return lines
            drain(.1)
        raise AssertionError(('library count', len(lines), count))

    try:
        start_player()
        start = time.monotonic()
        cmd('add -l ' + str(music))
        expect_count(args.count)
        results['import_seconds'] = time.monotonic()-start
        results['checks'].append('distinct-file directory import and full library export')
        # Real key input for view switching, search, navigation and terminal resize.
        os.write(master, ('2/track_%05d\r' % (args.count-1)).encode())
        drain(.2)
        os.write(master, b'\x1b[A\x1b[B')
        fcntl.ioctl(master, termios.TIOCSWINSZ, struct.pack('HHHH', 24, 80, 0, 0))
        drain(.2)
        start = time.monotonic()
        cmd('view 1', 'view 2', 'win-bottom', 'win-top')
        results['navigation_command_seconds'] = time.monotonic()-start
        results['checks'].append('search/navigation/view/resize interactions remain responsive')
        cmd('add -q ' + str(music / 'track_00000.wav'))
        assert len(cmd('save -q -').splitlines()) == 1
        results['checks'].append('queue operation with populated library')
        stop()
        start = time.monotonic()
        start_player()
        expect_count(args.count)
        results['restart_seconds'] = time.monotonic()-start
        results['checks'].append('full library persists across restart')
        (music / 'track_00000.wav').unlink()
        cmd('update')
        expect_count(args.count-1)
        results['checks'].append('update removes missing file')
        status = Path('/proc/%d/status' % process.pid).read_text()
        results['memory'] = [line for line in status.splitlines() if line.startswith(('VmRSS:', 'VmHWM:'))]
        stop()
        unexpected = screen.replace(b"Error: can't save when tracks are being added", b'')
        assert b'Error:' not in unexpected and b'Press <enter>' not in unexpected
        results['passed'] = True
    finally:
        if process and process.poll() is None:
            process.terminate()
            process.wait(timeout=5)
        socket.unlink(missing_ok=True)
        (out / 'screen.txt').write_bytes(screen)
        (out / 'results.json').write_text(json.dumps(results, indent=2)+'\n')
        print(out, flush=True)


if __name__ == '__main__':
    main()

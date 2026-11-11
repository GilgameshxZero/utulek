#!/usr/bin/env python3
"""Run the player and remote in an empty Bubblewrap root, with no host libraries."""
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
import uuid
import wave

ROOT = Path(__file__).resolve().parent.parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', type=Path, required=True)
    parser.add_argument('--remote', type=Path, required=True)
    parser.add_argument('--rootfs', type=Path, help='Optional distribution root filesystem')
    args = parser.parse_args()
    out = ROOT / '.obj/portable_runtime' / uuid.uuid4().hex[:12]
    (out / 'config').mkdir(parents=True)
    (out / 'tmp').mkdir()
    (out / 'config/rc').write_text('set mpris=false\nset output_plugin=alsa\n'
                                  'set dsp.alsa.device=null\nset softvol=true\n')
    with wave.open(str(out / 'track ü.wav'), 'wb') as f:
        f.setnchannels(2)
        f.setsampwidth(2)
        f.setframerate(8000)
        f.writeframes(b'\0' * 4 * 8000 * 60)
    sandbox = ['bwrap', '--unshare-user', '--unshare-pid', '--proc', '/proc',
               '--dev', '/dev', '--dir', '/etc', '--bind', str(out), '/state',
               '--setenv', 'HOME', '/state', '--setenv', 'XDG_CONFIG_HOME', '/state',
               '--setenv', 'CMUS_HOME', '/state/config', '--setenv', 'TMPDIR', '/state/tmp',
               '--setenv', 'LANG', 'C.UTF-8', '--setenv', 'TERM', 'xterm-256color',
               '--ro-bind', str(args.binary.resolve()), '/cmus',
               '--ro-bind', str(args.remote.resolve()), '/cmus-remote', '--chdir', '/state']
    if args.rootfs:
        sandbox[3:3] = ['--ro-bind', str(args.rootfs.resolve()), '/']
    master, slave = pty.openpty()
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 30, 120, 0, 0))
    screen, checks = bytearray(), []
    process = subprocess.Popen(sandbox + ['/cmus', '--listen', '/state/player.sock'],
                               stdin=slave, stdout=slave, stderr=slave)
    os.close(slave)

    def wait(seconds=.2):
        end = time.monotonic() + seconds
        while time.monotonic() < end:
            if select.select([master], [], [], .02)[0]:
                try:
                    screen.extend(os.read(master, 65536))
                except OSError:
                    break

    def remote(*commands, query=False):
        result = subprocess.check_output(sandbox + ['/cmus-remote', '--server',
            '/state/player.sock'] + (['-Q'] if query else ['-C', *commands]),
            text=True, timeout=10)
        wait()
        return result

    try:
        for _ in range(40):
            wait(.1)
            assert process.poll() is None, screen.decode(errors='replace')
            if (out / 'player.sock').exists():
                break
        assert (out / 'player.sock').exists(), screen.decode(errors='replace')
        checks.append('startup_with_no_host_libraries_or_terminfo')
        os.write(master, b':colorscheme drac\t\r')
        wait()
        remote('add /state/track ü.wav')
        remote('save /state/library.pl')
        assert '/state/track ü.wav' in (out / 'library.pl').read_text()
        checks.append('unicode_import_and_remote_without_host_libraries')
        remote('player-play')
        # ALSA null intentionally consumes as fast as possible; transport
        # timing belongs to the live-device suite, not this no-device check.
        for _ in range(30):
            if 'status stopped' in remote(query=True):
                break
            wait(.1)
        else:
            raise AssertionError('Null output did not decode to completion')
        checks.append('bundled_alsa_null_decodes_to_completion')
        remote('quit')
        process.wait(timeout=10)
        assert process.returncode == 0
        assert 'set color_titleline_bg=lightgreen\n' in (out / 'config/autosave').read_text()
        assert 'colorscheme dracula' in (out / 'config/command-history').read_text()
        checks.append('bundled_theme_completion_and_clean_exit')
        assert not list((out / 'tmp').iterdir()), 'Runtime left extracted files behind'
        checks.append('extracted_libraries_cleaned_on_exit')
        assert b'Error:' not in screen, screen.decode(errors='replace')
    finally:
        if process.poll() is None:
            process.terminate()
            process.wait(timeout=10)
        os.close(master)
        (out / 'screen.bin').write_bytes(screen)
        (out / 'results.json').write_text(json.dumps({'checks': checks,
            'passed': len(checks) == 5}, indent=2) + '\n')
    print(json.dumps({'results': str(out / 'results.json'), 'checks': checks}))


if __name__ == '__main__':
    main()

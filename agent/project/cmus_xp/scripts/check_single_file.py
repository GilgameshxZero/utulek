#!/usr/bin/env python3
"""Exercise a relocated Linux binary, embedded themes, keys, remote and restart."""
import argparse
import fcntl
import json
import os
from pathlib import Path
import pty
import select
import shutil
import struct
import subprocess
import termios
import time
import uuid

from checklib import write

ROOT = Path(__file__).resolve().parent.parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', type=Path, required=True)
    parser.add_argument('--remote', type=Path, required=True)
    args = parser.parse_args()
    run_id = uuid.uuid4().hex[:12]
    out = ROOT / '.obj/single_file' / ('run-' + run_id)
    app = out / 'relocated app ü'
    app.mkdir(parents=True)
    config = out / 'config'
    config.mkdir()
    binary, remote = app / 'cmus', app / 'cmus-remote'
    shutil.copy2(args.binary, binary)
    shutil.copy2(args.remote, remote)
    # Keep Unix sockets below sun_path's limit, regardless of checkout depth.
    socket = Path('/tmp') / ('cmus-single-' + run_id + '.sock')
    env = dict(os.environ, TERM='xterm-256color', CMUS_HOME=str(config),
               CMUS_LIB_DIR=str(out / 'absent_plugins'))
    env.pop('CMUS_DATA_DIR', None)
    checks = []
    process, master = None, None
    screen = bytearray()

    def wait(seconds=.25):
        end = time.monotonic() + seconds
        while time.monotonic() < end:
            if select.select([master], [], [], .02)[0]:
                try:
                    screen.extend(os.read(master, 65536))
                except OSError:
                    break

    def command(*commands):
        subprocess.run([str(remote), '--server', str(socket), '-C', *commands],
                       env=env, check=True, timeout=5)
        wait()

    def start():
        nonlocal process, master
        master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 30, 120, 0, 0))
        process = subprocess.Popen([str(binary), '--listen', str(socket)],
                                   env=env, cwd=out, stdin=slave, stdout=slave, stderr=slave)
        os.close(slave)
        for _ in range(30):
            wait(.1)
            if socket.exists():
                break
            assert process.poll() is None, 'Fresh startup failed'
        assert socket.exists(), 'Remote socket unavailable'

    def stop():
        command('quit')
        process.wait(timeout=5)
        assert process.returncode == 0
        os.close(master)

    try:
        plugins = subprocess.check_output([str(binary), '--plugins'], env=env, text=True)
        assert plugins.count('built into executable') == 2, plugins
        for name in ('stream:', 'mad:', 'cdio:', 'flac:', 'modplug:', 'mpc:', 'vorbis:', 'opus:',
                     'wavpack:', 'wav:', 'aac:', 'cue:', 'ffmpeg:', 'pulse', 'alsa', 'jack', 'ao'):
            assert '  ' + name in plugins, name
        checks.append('relocated_embedded_13_input_4_output_plugins')
        start()
        command('set mpris=false')
        os.write(master, b':colorscheme drac\t\r')
        wait()
        stop()
        saved = (config / 'autosave').read_text()
        assert 'set color_titleline_bg=lightgreen\n' in saved
        assert 'colorscheme dracula' in (config / 'command-history').read_text()
        checks.append('fresh_defaults_and_keyboard_theme_completion')
        start()
        for theme in sorted((ROOT / 'data').glob('*.theme')):
            command('colorscheme ' + theme.stem)
        command('colorscheme dracula')
        stop()
        assert 'set color_titleline_bg=lightgreen\n' in (config / 'autosave').read_text()
        checks.append('all_embedded_themes_via_remote')
        (config / 'dracula.theme').write_text('set color_titleline_bg=123\n')
        start()
        command('colorscheme dracula')
        stop()
        assert 'set color_titleline_bg=123\n' in (config / 'autosave').read_text()
        checks.append('user_theme_override')
        start()
        stop()
        assert 'set color_titleline_bg=123\n' in (config / 'autosave').read_text()
        checks.append('restart_persistence')
        assert not (app / 'cmus_xp-runtime').exists()
        assert b'Error:' not in screen, screen.decode(errors='replace')
    finally:
        if process and process.poll() is None:
            process.terminate()
            process.wait(timeout=5)
        socket.unlink(missing_ok=True)
        (out / 'screen.bin').write_bytes(screen)
        write(out / 'results.json', [{'check': name, 'passed': True} for name in checks],
              binary=str(args.binary))
    print(json.dumps({'results': str(out / 'results.json'), 'checks': checks}))


if __name__ == '__main__':
    main()

#!/usr/bin/env python3
"""Capture reference album rendering in an isolated Linux cmus PTY."""
import argparse
import fcntl
import json
import os
from pathlib import Path
import pty
import select
import signal
import struct
import subprocess
import termios
import time

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('binary', type=Path)
    parser.add_argument('fixtures', type=Path)
    parser.add_argument('output', type=Path)
    args = parser.parse_args()
    config = args.output.resolve().parent / ('linux_config_' + str(os.getpid()))
    config.mkdir(exist_ok=True)
    (config / 'rc').write_text('set mpris=false\nset softvol=true\nset show_all_tracks=true\n'
        'set color_separator=red\nset color_trackwin_album_fg=default\n'
        'set color_trackwin_album_bg=default\nset color_trackwin_album_attr=standout|reverse|bold\n')
    master, slave = pty.openpty()
    def resize(cols, rows):
        fcntl.ioctl(master, termios.TIOCSWINSZ, struct.pack('HHHH', rows, cols, 0, 0))
    resize(100, 30)
    process = subprocess.Popen([str(args.binary), '--listen', '/tmp/cmus-album-%d.sock' % os.getpid()],
        stdin=slave, stdout=slave, stderr=slave,
        env=dict(os.environ, TERM='xterm-256color', CMUS_HOME=str(config)))
    os.close(slave)
    data = bytearray()
    def wait(seconds):
        end = time.monotonic() + seconds
        while time.monotonic() < end:
            if select.select([master], [], [], .02)[0]:
                try: data.extend(os.read(master, 65536))
                except OSError: break
    frames = []
    def capture(phase, cols=100, rows=30):
        import base64
        wait(.6)
        frames.append(dict(surface='linux_upstream', stream='linux_upstream', phase=phase,
            cols=cols, rows=rows, bytes=base64.b64encode(data).decode()))
        data.clear()
    try:
        wait(1)
        os.write(master, (':add ' + str(args.fixtures.resolve()) + '\r1').encode())
        wait(1)
        for phase, text in [('albums', ''),
            ('standout', ':set color_trackwin_album_attr=standout\r'),
            ('reverse', ':set color_trackwin_album_attr=reverse\r'),
            ('bold', ':set color_trackwin_album_attr=bold\r'),
            ('palette', ':set color_trackwin_album_attr=default\r:set color_trackwin_album_fg=red\r:set color_trackwin_album_bg=blue\r'),
            ('restore_colors', ':set color_trackwin_album_fg=default\r:set color_trackwin_album_bg=default\r'),
            ('attributes', ':set color_trackwin_album_attr=bold|reverse|underline|italic\r'),
            ('short_heading', ':set format_trackwin_album= Short %= R \r'),
            ('restore_heading', ':set format_trackwin_album= %l %= %y %{duration} \r'),
            ('focus_tracks', '\t'), ('selection', 'jjjj'), ('view_switch', '21')]:
            if text: os.write(master, text.encode())
            capture(phase)
        for cols, rows in [(80,24),(120,40),(100,30)]:
            resize(cols,rows)
            process.send_signal(signal.SIGWINCH)
            capture('resize', cols,rows)
        os.write(master,b':quit\r')
        process.wait(timeout=5)
        assert process.returncode == 0
    finally:
        if process.poll() is None: process.terminate(); process.wait(timeout=5)
        os.close(master)
        Path('/tmp/cmus-album-%d.sock' % os.getpid()).unlink(missing_ok=True)
        args.output.write_text(''.join(json.dumps(f)+'\n' for f in frames))

if __name__ == '__main__': main()

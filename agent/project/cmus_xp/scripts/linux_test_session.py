"""Isolated real Linux player session used by persistence/release checks."""
import fcntl
import os
from pathlib import Path
import pty
import resource
import select
import signal
import struct
import subprocess
import termios
import threading
import time
import uuid


class Session:
    def __init__(self, binary, remote, config, *, address=None, password=None):
        self.binary, self.remote, self.config = map(Path, (binary, remote, config))
        self.socket = Path('/tmp') / ('cmus-check-' + uuid.uuid4().hex[:12])
        self.screen = bytearray()
        self.address, self.password = address, password

    def drain(self, seconds=.15):
        end = time.monotonic() + seconds
        while time.monotonic() < end:
            if select.select([self.master], [], [], .01)[0]:
                try:
                    self.screen.extend(os.read(self.master, 65536))
                except OSError:
                    break

    def start(self):
        self.master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 30, 120, 0, 0))
        self.process = subprocess.Popen([str(self.binary.resolve()), '--listen', str(self.address or self.socket)],
            stdin=slave, stdout=slave, stderr=slave,
            env=dict(os.environ, CMUS_HOME=str(self.config.resolve()), TERM='xterm-256color'),
            start_new_session=True,
            preexec_fn=lambda: signal.signal(signal.SIGXFSZ, signal.SIG_IGN))
        os.close(slave)
        for _ in range(100):
            self.drain(.05)
            if self.address:
                try:
                    self.command('status')
                    return self
                except AssertionError:
                    pass
            elif self.socket.exists():
                return self
            assert self.process.poll() is None, self.screen.decode(errors='replace')
        raise AssertionError('Player did not create its remote socket')

    def command(self, *commands):
        return self.client('-C', *commands)

    def client(self, *arguments):
        auth = ['--passwd', self.password] if self.password is not None else []
        client = subprocess.Popen([str(self.remote.resolve()), '--server', str(self.address or self.socket), *auth,
                                   *arguments], stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                  start_new_session=True)
        end = time.monotonic() + 10
        while True:
            try:
                output, errors = client.communicate(timeout=.03)
                break
            except subprocess.TimeoutExpired:
                self.drain(.01)
                if time.monotonic() >= end:
                    os.killpg(client.pid, signal.SIGKILL)
                    client.communicate()
                    raise AssertionError('Remote client stalled: ' + repr(arguments))
        assert client.returncode == 0, errors.decode(errors='replace')
        assert b'Error:' not in output, output.decode(errors='replace')
        self.drain()
        return output.decode(errors='replace')

    def keys(self, keys):
        os.write(self.master, keys.encode())
        self.drain()

    def limit_file_size(self, size):
        pending = [self.process.pid]
        while pending:
            pid = pending.pop()
            children = Path('/proc') / str(pid) / 'task' / str(pid) / 'children'
            pending.extend(map(int, children.read_text().split()))
            resource.prlimit(pid, resource.RLIMIT_FSIZE, (size, size))

    def stop(self):
        try:
            if self.process.poll() is None:
                # The upstream quit command asks for confirmation during an
                # import. Confirm only this test player's private terminal.
                confirmation = threading.Timer(.2, lambda: os.write(self.master, b'y'))
                confirmation.start()
                try:
                    self.command('quit')
                finally:
                    confirmation.cancel()
                    confirmation.join()
                self.process.wait(timeout=10)
            assert self.process.returncode == 0
        finally:
            if self.process.poll() is None:
                os.killpg(self.process.pid, signal.SIGTERM)
                try:
                    self.process.wait(timeout=3)
                except subprocess.TimeoutExpired:
                    os.killpg(self.process.pid, signal.SIGKILL)
                    self.process.wait(timeout=3)
            self.drain()
            os.close(self.master)
            self.socket.unlink(missing_ok=True)

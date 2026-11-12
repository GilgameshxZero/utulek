"""Shared helpers for the standalone check scripts.

Keeps session setup, bounded subprocess execution and result files in one
place so every check writes the same machine-readable schema.
"""
import fcntl
import json
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

ROOT = Path(__file__).resolve().parent.parent
RESULTS = ROOT / '.obj/checks'


def output(name):
    return RESULTS / name / 'results.json'


def write(path, checks, **extra):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    payload = dict(extra, passed=all(c['passed'] for c in checks), checks=checks)
    path.write_text(json.dumps(payload, indent=2) + '\n')
    return payload


def run(command, *, timeout=60, env=None, check=True, cwd=None):
    process = subprocess.run(command, capture_output=True, text=True, timeout=timeout,
                             env=env, cwd=cwd)
    if check and process.returncode:
        raise AssertionError(f'{command[0]} exited {process.returncode}: ' +
                             (process.stderr or process.stdout).strip())
    return process


def wait_status(session, needle, limit=100):
    text = ''
    for _ in range(limit):
        text = session.command('status')
        if needle in text:
            return True, text
        session.drain(.03)
    return False, text


def process_tree(pid):
    seen = []
    pending = [pid]
    while pending:
        current = pending.pop()
        seen.append(current)
        try:
            children = (Path('/proc') / str(current) / 'task' / str(current) /
                        'children').read_text().split()
        except OSError:
            continue
        pending.extend(int(child) for child in children)
    return seen


def memory(pid):
    rss = 0
    fds = 0
    for current in process_tree(pid):
        try:
            for line in (Path('/proc') / str(current) / 'status').read_text().splitlines():
                if line.startswith('VmRSS:'):
                    rss = max(rss, int(line.split()[1]) * 1024)
            fds = max(fds, len(list((Path('/proc') / str(current) / 'fd').iterdir())))
        except OSError:
            pass
    return rss, fds


class Session:
    def __init__(self, binary, remote, config, *, address=None, password=None, env=None):
        self.binary, self.remote, self.config = map(Path, (binary, remote, config))
        self.socket = Path('/tmp') / ('cmus-check-' + uuid.uuid4().hex[:12])
        self.screen = bytearray()
        self.address, self.password = address, password
        self.env = env

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
        env = dict(os.environ, CMUS_HOME=str(self.config.resolve()), TERM='xterm-256color')
        if self.env:
            env.update(self.env)
        self.process = subprocess.Popen([str(self.binary.resolve()), '--listen', str(self.address or self.socket)],
            stdin=slave, stdout=slave, stderr=slave,
            env=env,
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

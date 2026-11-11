#!/usr/bin/env python3
"""Rootless, loopback-only Dovecot fixture using extracted Debian packages.

Never installs packages, touches production mail, or creates a system service.
"""
import imaplib
import grp
import os
from pathlib import Path
import pwd
import signal
import socket
import ssl
import subprocess
import time

from imap_fixture_control import USER, PASSWORD


class DovecotFixture:
    def __init__(self, prefix, root):
        self.prefix, self.root = Path(prefix).resolve(), Path(root).resolve()
        self.root.mkdir(parents=True, exist_ok=True)
        self.process = None
        self.output = None
        with socket.socket() as sock:
            sock.bind(('127.0.0.1', 0))
            self.port = sock.getsockname()[1]
        self.cert, self.key = self.root / 'cert.pem', self.root / 'key.pem'
        subprocess.run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes',
                        '-days', '2', '-keyout', str(self.key), '-out', str(self.cert),
                        '-subj', '/CN=localhost', '-addext', 'subjectAltName=DNS:localhost,IP:127.0.0.1'],
                       check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        self.key.chmod(0o600)
        login = pwd.getpwuid(os.getuid()).pw_name
        group = grp.getgrgid(os.getgid()).gr_name
        lib = self.prefix / 'usr/lib/dovecot'
        for name in ['run', 'state', 'home', 'mail', 'empty', 'tmp']:
            (self.root / name).mkdir(exist_ok=True)
        (self.root / 'tmp/run').mkdir(exist_ok=True)
        self.config = self.root / 'dovecot.conf'
        self.config.write_text(f'''dovecot_config_version = 2.4.0
dovecot_storage_version = 2.4.0
base_dir = /tmp/run
state_dir = {self.root}/state
libexec_dir = {lib}
import_environment {{
  LD_LIBRARY_PATH = {lib}
}}
default_internal_user = {login}
default_internal_group = {group}
default_login_user = {login}
log_path = {self.root}/dovecot.log
protocols {{
  imap = yes
}}
listen = 127.0.0.1
ssl = required
ssl_server_cert_file = {self.cert}
ssl_server_key_file = {self.key}
auth_mechanisms = plain login
passdb static {{
  password = {PASSWORD}
}}
userdb static {{
  fields {{
    uid = {os.getuid()}
    gid = {os.getgid()}
    home = {self.root}/home
  }}
}}
mail_uid = {os.getuid()}
mail_gid = {os.getgid()}
mail_home = {self.root}/home
mail_driver = maildir
mail_path = {self.root}/mail
namespace inbox {{
  inbox = yes
  separator = /
}}
service anvil {{
  chroot =
}}
service imap-login {{
  chroot =
  inet_listener imap {{
    port = 0
  }}
  inet_listener imaps {{
    port = {self.port}
    ssl = yes
  }}
}}
''')
        self.env = dict(os.environ, LD_LIBRARY_PATH=str(lib))

    def __enter__(self):
        self.output = (self.root / 'master_output.txt').open('wb')
        # Debian binaries have compiled module paths. A private mount namespace
        # supplies those paths without creating files in the host's /usr.
        command = ['bwrap', '--die-with-parent', '--ro-bind', '/', '/',
                   '--dev-bind', '/dev', '/dev', '--proc', '/proc', '--tmpfs', '/usr',
                   '--ro-bind', '/usr/lib/x86_64-linux-gnu', '/usr/lib/x86_64-linux-gnu',
                   '--ro-bind', '/usr/lib64', '/usr/lib64', '--ro-bind', '/usr/share', '/usr/share',
                   '--ro-bind', str(self.prefix / 'usr/lib/dovecot'), '/usr/lib/dovecot',
                   '--ro-bind', str(self.prefix / 'usr/bin/doveconf'), '/usr/bin/doveconf',
                   '--bind', str(self.root), str(self.root), '--bind', str(self.root / 'tmp'), '/tmp',
                   str(self.prefix / 'usr/bin/doveconf'), '-c', str(self.config), '-F',
                   str(self.prefix / 'usr/sbin/dovecot'), '-F', '-c', str(self.config)]
        self.process = subprocess.Popen(command, env=self.env, stdout=self.output,
                                        stderr=subprocess.STDOUT, start_new_session=True)
        try:
            deadline = time.monotonic() + 10
            while time.monotonic() < deadline:
                if self.process.poll() is not None:
                    raise RuntimeError((self.root / 'master_output.txt').read_text())
                try:
                    with socket.create_connection(('127.0.0.1', self.port), timeout=.2):
                        return self
                except OSError:
                    time.sleep(.05)
            raise RuntimeError('Dovecot fixture did not start')
        except BaseException:
            self.__exit__(None, None, None)
            raise

    def connect(self):
        client = imaplib.IMAP4_SSL('localhost', self.port, timeout=10,
                                  ssl_context=ssl.create_default_context(cafile=str(self.cert)))
        client.login(USER, PASSWORD)
        return client

    def __exit__(self, *_):
        if self.process is not None and self.process.poll() is None:
            os.killpg(self.process.pid, signal.SIGTERM)
            try:
                self.process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                os.killpg(self.process.pid, signal.SIGKILL)
                self.process.wait()
        if self.output is not None:
            self.output.close()

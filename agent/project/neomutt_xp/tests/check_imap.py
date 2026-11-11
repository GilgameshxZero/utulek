#!/usr/bin/env python3
"""Exercise real NeoMutt IMAP UI and verify state with an independent client."""
import argparse
import errno
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

from dovecot_fixture import DovecotFixture
from imap_fixture_control import USER, PASSWORD, message, snapshot, by_number
from network_fixture import DisconnectProxy


class Terminal:
    def __init__(self, app, rc, mailbox, root, draft=None):
        self.master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 40, 120, 0, 0))
        def setup():
            os.setsid()
            fcntl.ioctl(0, termios.TIOCSCTTY, 0)
        arguments = ['-H', str(draft)] if draft else ['-f', mailbox]
        self.process = subprocess.Popen([str(app), '-n', '-F', str(rc), *arguments],
            stdin=slave, stdout=slave, stderr=slave, cwd=root, preexec_fn=setup,
            env=dict(os.environ, HOME=str(root), TMPDIR=str(root), TERM='xterm-256color', LANG='C.UTF-8'))
        os.close(slave)
        self.data = bytearray()

    def send(self, data):
        self.data.clear()
        os.write(self.master, data.encode())

    def pump(self, duration=.1):
        end = time.monotonic() + duration
        while time.monotonic() < end:
            if select.select([self.master], [], [], min(.05, end-time.monotonic()))[0]:
                try:
                    block = os.read(self.master, 65536)
                except OSError as error:
                    if error.errno == errno.EIO:
                        return
                    raise
                if not block:
                    return
                self.data.extend(block)
                if b'\x1b[6n' in block:
                    os.write(self.master, b'\x1b[1;1R')
                if b'\x1b[c' in block:
                    os.write(self.master, b'\x1b[?1;2c')

    def until(self, text, timeout=12):
        end = time.monotonic() + timeout
        while text.encode() not in self.data and self.process.poll() is None and time.monotonic() < end:
            self.pump(.05)
        if text.encode() not in self.data:
            raise AssertionError('Missing '+text+': '+repr(self.data[-1400:]))

    def quit(self):
        self.send('q')
        self.pump(.3)
        self.process.wait(timeout=5)
        assert self.process.returncode == 0

    def close(self):
        if self.process.poll() is None:
            self.process.kill()
            self.process.wait()
        os.close(self.master)


def write_config(root, server, port):
    url = f'imaps://localhost:{port}/'
    rc = root/'test.rc'
    rc.write_text(f'''set folder="{url}" imap_user="{USER}" imap_pass="{PASSWORD}"
set imap_authenticators="login" ssl_force_tls=yes
set certificate_file="{server.cert}" ssl_verify_host=yes ssl_verify_dates=yes socket_timeout=3
set charset="utf-8" sort=unsorted use_threads=no quit=yes delete=yes move=no mark_old=no
set wait_key=no sleep_time=0 confirm_create=no confirm_append=no timeout=1 mail_check=1
unset record
unset beep
''')
    return rc


def basic(app, prefix, root, concurrent_only=False, idle_delay=0):
    phases = []
    terminal = None
    with DovecotFixture(prefix, root/'server') as server:
        client = server.connect()
        for number in range(1, 4):
            assert client.append('INBOX', None, None, message(number))[0] == 'OK'
        assert client.create('Archive')[0] == 'OK'
        client.logout()
        url = f'imaps://localhost:{server.port}/'
        rc = write_config(root, server, server.port)
        if concurrent_only:
            with rc.open('a') as config:config.write('set imap_idle=no timeout=300 mail_check=300\n')
        try:
            terminal = Terminal(app, rc, url+'INBOX', root)
            terminal.until('IMAP fixture 1')
            terminal.send(':exec first-entry\r')
            terminal.pump(.1)
            terminal.send('\r')
            terminal.until('Owned IMAP body 1')
            terminal.send('q')
            terminal.pump(.1)
            terminal.send('F$')
            terminal.pump(.3)
            item = by_number(snapshot(server, 'INBOX'), 1)[0]
            assert '\\Flagged' in item['flags'] and '\\Seen' in item['flags'], item['flags']
            phases.append('read_and_flag_verified_on_server')

            # Keep a local per-flag change pending while another client changes
            # a different flag on the same message.
            terminal.send('/IMAP fixture 1\r')
            terminal.pump(.2)
            terminal.send('F')
            terminal.pump(.1)
            client = server.connect()
            client.select('INBOX')
            assert client.uid('store', item['uid'], '+FLAGS.SILENT', '(\\Answered)')[0]=='OK'
            client.logout()
            assert 'Answered' in by_number(snapshot(server,'INBOX'),1)[0]['flags']
            if idle_delay:terminal.pump(idle_delay)
            terminal.send('$')
            terminal.pump(.3)
            item = by_number(snapshot(server, 'INBOX'), 1)[0]
            assert '\\Flagged' not in item['flags'] and '\\Answered' in item['flags'], item['flags']
            terminal.send('/IMAP fixture 1\r')
            terminal.pump(.2)
            terminal.send('F$')
            terminal.pump(.3)
            item = by_number(snapshot(server, 'INBOX'), 1)[0]
            assert '\\Flagged' in item['flags'] and '\\Answered' in item['flags'], item['flags']
            phases.append('same_message_local_and_external_flag_deltas_preserved')
            if concurrent_only:
                terminal.quit()
                return {'passed':True,'phases':phases}

            terminal.send('/IMAP fixture 2\r')
            terminal.pump(.2)
            terminal.send('s')
            terminal.until('Save to mailbox')
            terminal.send('\x15'+url+'Archive\r')
            terminal.pump(.3)
            terminal.send('$')
            terminal.pump(.3)
            assert not by_number(snapshot(server, 'INBOX'), 2)
            assert by_number(snapshot(server, 'Archive'), 2)[0]['body'] == message(2)
            phases.append('move_verified_on_server')

            terminal.send('/IMAP fixture 3\r')
            terminal.pump(.2)
            terminal.send('d$')
            terminal.pump(.3)
            assert not by_number(snapshot(server, 'INBOX'), 3)
            phases.append('delete_expunge_verified_on_server')

            client = server.connect()
            client.append('INBOX', None, None, message(4))
            client.logout()
            terminal.send('$')
            terminal.until('IMAP fixture 4')
            terminal.send('/IMAP fixture 4\r')
            terminal.pump(.2)
            terminal.send('\r')
            terminal.until('Owned IMAP body 4')
            terminal.send('q')
            terminal.pump(.1)
            phases.append('new_delivery_visible_and_readable')

            terminal.quit()
            terminal.close()
            terminal = Terminal(app, rc, url+'INBOX', root)
            terminal.until('IMAP fixture 4')
            terminal.send('/IMAP fixture 1\r')
            terminal.pump(.2)
            terminal.send('\r')
            terminal.until('Owned IMAP body 1')
            terminal.send('q')
            terminal.pump(.1)
            terminal.quit()
            final = snapshot(server, 'INBOX')
            assert len(final) == 2
            assert by_number(final, 1)[0]['body'] == message(1)
            assert by_number(final, 4)[0]['body'] == message(4)
            assert '\\Flagged' in by_number(final, 1)[0]['flags']
            phases.append('restart_preserves_server_state_and_original_bytes')
            return {'passed': True, 'phases': phases,
                    'final': [{k:v for k,v in item.items() if k!='body'} for item in final]}
        except Exception as error:
            if terminal:
                (root/'failure.vt').write_bytes(terminal.data)
            return {'passed': False, 'phases': phases, 'error': str(error)}
        finally:
            if terminal:
                terminal.close()


def interrupt(app, prefix, root, kind='flag'):
    terminal = None
    with DovecotFixture(prefix, root/'server') as server, DisconnectProxy(server.port) as proxy:
        mailbox='Owned' if kind=='uidvalidity' else 'INBOX'
        client = server.connect()
        if mailbox!='INBOX':assert client.create(mailbox)[0]=='OK'
        client.append(mailbox, '(\\Seen)', None, message(1))
        if kind in ['delete','uid_shift']:
            client.append(mailbox, '(\\Seen)', None, message(2))
        client.logout()
        rc = write_config(root, server, proxy.port)
        try:
            terminal = Terminal(app, rc, f'imaps://localhost:{proxy.port}/{mailbox}', root)
            terminal.until('IMAP fixture 1')
            target=2 if kind=='uid_shift' else 1
            terminal.send('/IMAP fixture '+str(target)+'\r');terminal.pump(.2)
            terminal.send('\r');terminal.until('Owned IMAP body '+str(target))
            terminal.send('q');terminal.pump(.1)
            terminal.send('d' if kind=='delete' else 'F')
            terminal.pump(.2)
            proxy.disconnect(block=True)
            terminal.send('$')
            terminal.pump(.7)
            diagnostic = terminal.data.decode(errors='replace')
            if kind in ['uid_shift','uidvalidity']:
                client=server.connect()
                if kind=='uid_shift':
                    client.select(mailbox);uid=by_number(snapshot(server,mailbox),1)[0]['uid']
                    client.uid('store',uid,'+FLAGS.SILENT','(\\Deleted)');client.expunge()
                else:
                    assert client.rename(mailbox,'Original')[0]=='OK'
                    assert client.create(mailbox)[0]=='OK'
                    assert client.append(mailbox,'(\\Seen)',None,message(9))[0]=='OK'
                client.logout()
            proxy.resume()
            if 'reconnect' in diagnostic.lower() and '?' in diagnostic:
                terminal.send('y')
                terminal.pump(.5)
            terminal.send('$')
            terminal.pump(.5)
            def complete(items):
                if kind=='delete':return len(items)==1 and bool(by_number(items,2))
                if kind=='uidvalidity':return b'identity changed' in terminal.data
                target=by_number(items,2 if kind=='uid_shift' else 1)
                return bool(target) and '\\Flagged' in target[0]['flags']
            deadline=time.monotonic()+8
            items=snapshot(server,mailbox)
            while not complete(items) and time.monotonic()<deadline:
                terminal.pump(.1);items=snapshot(server,mailbox)
            assert complete(items), str([{k:v for k,v in item.items() if k!='body'} for item in items])+'; UI: '+diagnostic[-1300:]
            if kind=='uidvalidity':
                assert len(items)==1 and items[0]['body']==message(9) and '\\Flagged' not in items[0]['flags']
                assert snapshot(server,'Original')[0]['body']==message(1)
                return {'passed':True,'kind':kind,'new_identity_not_modified':True,'explicit_identity_error':True}
            expected=2 if kind in ['delete','uid_shift'] else 1
            assert len(items)==1 and items[0]['body']==message(expected)
            terminal.quit()
            return {'passed':True,'kind':kind,'pending_change_survived_interruption':True,
                    'original_bytes_preserved':True,'diagnostic':diagnostic[-1300:]}
        except Exception as error:
            if terminal:
                (root/'failure.vt').write_bytes(terminal.data)
            return {'passed': False, 'error': str(error)}
        finally:
            if terminal:
                terminal.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('app', type=Path)
    parser.add_argument('--dovecot-prefix', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--interrupt', action='store_true')
    parser.add_argument('--concurrent-only',action='store_true')
    parser.add_argument('--concurrent-idle',action='store_true')
    parser.add_argument('--interruption-kind',choices=['flag','delete','uid_shift','uidvalidity'],default='flag')
    args = parser.parse_args()
    root = args.output.resolve()
    root.mkdir(parents=True, exist_ok=True)
    result = interrupt(args.app.resolve(),args.dovecot_prefix,root,args.interruption_kind) if args.interrupt else basic(args.app.resolve(),args.dovecot_prefix,root,args.concurrent_only,1.6 if args.concurrent_idle else 0)
    (root/'results.json').write_text(json.dumps(result, indent=2)+'\n')
    print(json.dumps(result))
    return 0 if result['passed'] else 1


if __name__ == '__main__':
    raise SystemExit(main())

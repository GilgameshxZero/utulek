#!/usr/bin/env python3
"""Drive actual Linux NeoMutt through a PTY with synthetic Maildir fixtures."""
import argparse
import errno
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

p = argparse.ArgumentParser()
p.add_argument('app', type=Path)
p.add_argument('scratch', type=Path)
a = p.parse_args()
root = a.scratch.resolve()
root.mkdir(parents=True, exist_ok=True)
# Callers provide an isolated run directory; clear only our fixture Maildirs.
import shutil
for name in ('inbox.maildir','archive.maildir','postponed.maildir'):
    if (root/name).exists():shutil.rmtree(root/name)
mailbox = root / 'inbox.maildir'
for sub in ('new', 'cur', 'tmp'): (mailbox / sub).mkdir(parents=True, exist_ok=True)
for i in range(1,4):
    (mailbox / 'cur' / f'fixture{i};2,S').write_text(
        f'From: Sender <sender@example.invalid>\nTo: Reader <reader@example.invalid>\n'
        f'Date: Tue, 03 Nov 2026 00:00:00 +0000\nMessage-ID: <fixture{i}@example.invalid>\n'
        f'Subject: Windows fixture {i}\nMIME-Version: 1.0\nContent-Type: text/plain; charset=utf-8\n'
        f'Content-Transfer-Encoding: 8bit\n\nfixture body {i}\nUnicode: Grüße 日本語\n')
archive=root/'archive.maildir';postponed=root/'postponed.maildir'
for box in (archive,postponed):
    for sub in ('cur','new','tmp'):(box/sub).mkdir(parents=True,exist_ok=True)
rc = root / 'test.rc'
rc.write_text('set folder="."\nset maildir_field_delimiter=";"\nset mbox_type=Maildir\nset charset="utf-8"\n'
              'set realname="Fixture Reader"\nset from="reader@example.invalid"\nset sort=unsorted\n'
              'set quit=yes\nset delete=yes\nset move=no\nset mark_old=no\nset read_inc=1\n'
              'set write_inc=1\nset timeout=1\nset sleep_time=0\nunset record\nunset beep\nset arrow_cursor=yes\n'
              'bind editor <space> noop\nset postpone=no\nset confirm_append=no\n'
              f'set postponed="{postponed}"\nset header_cache="{root / "cache"}/"\n'
              f'set editor="{Path(__file__).resolve().parents[1] / ".obj/fixture_editor"}"\n')
master, slave = pty.openpty()
fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH',30,100,0,0))
def child_setup():
    os.setsid()
    fcntl.ioctl(0, termios.TIOCSCTTY, 0)
child = subprocess.Popen([str(a.app.resolve()),'-n','-F',str(rc),'-f',str(mailbox)],
    stdin=slave,stdout=slave,stderr=slave,cwd=root,
    env=dict(os.environ,HOME=str(root),TMPDIR=str(root),TERM='xterm-256color',LANG='C.UTF-8'),
    preexec_fn=child_setup)
os.close(slave)
data = bytearray()
phases=[]
def pump(duration):
    deadline=time.monotonic()+duration
    while time.monotonic()<deadline:
        ready,_,_=select.select([master],[],[],min(.05,max(0,deadline-time.monotonic())))
        if ready:
            try: block=os.read(master,65536)
            except OSError as e:
                if e.errno==errno.EIO: return
                raise
            if not block:return
            data.extend(block)
            if b'\x1b[6n' in block:os.write(master,b'\x1b[1;1R')
            if b'\x1b[c' in block:os.write(master,b'\x1b[?1;2c')
def until(token):
    deadline=time.monotonic()+10
    while token.encode() not in data and child.poll() is None and time.monotonic()<deadline:pump(.05)
    if token.encode() not in data:raise RuntimeError(f'missing {token}: {bytes(data[-1600:])!r}')
def capture(name):
    (root/(name+'.vt')).write_bytes(data)
    phases.append(name)
try:
    until('Windows fixture');capture('index')
    os.write(master,b'\r');until('fixture body');capture('pager')
    for rows,cols,label in [(24,80,'resize_small'),(40,120,'resize_large')]:
        fcntl.ioctl(master,termios.TIOCSWINSZ,struct.pack('HHHH',rows,cols,0,0))
        child.send_signal(signal.SIGWINCH);pump(.4);capture(label)
    os.write(master,b'q');pump(.2)
    os.write(master,b'/Windows fixture 3\r');pump(.3);capture('search')
    os.write(master,b'\r');until('fixture body 3');capture('search_pager')
    os.write(master,b'q');pump(.2)
    data.clear();os.write(master,b'm');until('To:')
    os.write(master,b'recipient@example.invalid\r');until('Subject:')
    os.write(master,'native fixture draft Grüße 日本語\r'.encode());until('Attachments');capture('compose_editor')
    if b'native fixture draft Gr' not in data:raise RuntimeError('Subject input changed')
    os.write(master,b'q');pump(.4);capture('discard')
    os.write(master,b'F');pump(.15);os.write(master,b'$');pump(.4)
    if not any(f.name.startswith('fixture3;') and 'F' in f.name.split(',')[1] for f in (mailbox/'cur').iterdir()):raise RuntimeError('Flag not persisted')
    capture('flag_persisted')
    os.write(master,b'/Windows fixture 3\r');pump(.25)
    os.write(master,b'd');pump(.15);os.write(master,b'$');pump(.4)
    if len(list((mailbox/'cur').iterdir()))!=2:raise RuntimeError('Delete not persisted')
    capture('delete_persisted')
    os.write(master,b':exec first-entry\r');pump(.15);os.write(master,b'tt');pump(.15)
    data.clear();os.write(master,b';s');until('Save tagged to mailbox')
    os.write(master,('\x15'+str(archive)+'\r').encode());pump(.4);os.write(master,b'$');pump(.4)
    messages=list((archive/'cur').iterdir())+list((archive/'new').iterdir())
    if len(messages)!=2 or list((mailbox/'cur').iterdir()):raise RuntimeError('Tagged move failed')
    contents=''.join(f.read_text() for f in messages)
    if 'fixture body 1' not in contents or 'fixture body 2' not in contents:raise RuntimeError('Moved body changed')
    capture('tagged_move')
    data.clear();os.write(master,b'c');until('Open mailbox')
    os.write(master,('\x15'+str(archive)+'\r').encode());until('Windows fixture');capture('archive_open')
    data.clear();os.write(master,b'm');until('To:')
    os.write(master,b'recipient@example.invalid\r');until('Subject:')
    os.write(master,b'postponed fixture draft\r');until('Attachments')
    os.write(master,b'P');pump(.6)
    if len(list((postponed/'cur').iterdir())+list((postponed/'new').iterdir()))!=1:raise RuntimeError('Postpone failed')
    capture('postponed');os.write(master,b'q');pump(.5)
    code=child.wait(timeout=5)
    if code:raise RuntimeError(f'exit {code}')
    os.close(master)
    master,slave=pty.openpty()
    fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack('HHHH',30,100,0,0))
    child=subprocess.Popen([str(a.app.resolve()),'-n','-F',str(rc),'-p'],
        stdin=slave,stdout=slave,stderr=slave,cwd=root,
        env=dict(os.environ,HOME=str(root),TMPDIR=str(root),TERM='xterm-256color',LANG='C.UTF-8'),preexec_fn=child_setup)
    os.close(slave);data.clear();until('Attachments')
    if b'postponed fixture draft' not in data or 'Editor fixture: Grüße 日本語'.encode() not in data:raise RuntimeError('Recalled payload changed')
    capture('restart_recall');os.write(master,b'q');pump(.5)
    recall=child.wait(timeout=5)
    if recall!=1:raise RuntimeError(f'Recall cancel exit {recall}')
    if list((postponed/'cur').iterdir())+list((postponed/'new').iterdir()):raise RuntimeError('Recalled draft remains')
    capture('restart_discard')
    result={'passed':True,'exit':code,'recall_exit':recall,'phases':phases}
except Exception as e:
    result={'passed':False,'error':str(e),'phases':phases}
finally:
    if child.poll() is None:child.kill();child.wait()
    capture('final');os.close(master)
(root/'results.json').write_text(json.dumps(result,indent=2)+'\n')
print(json.dumps(result))
raise SystemExit(0 if result['passed'] else 1)

#!/usr/bin/env python3
"""Read genuine encrypted synthetic PGP/MIME mail through the Linux PTY UI."""
import argparse
import base64
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

p=argparse.ArgumentParser()
p.add_argument('app',type=Path)
p.add_argument('cipher',type=Path)
p.add_argument('output',type=Path)
a=p.parse_args()
root=a.output.resolve();root.mkdir(parents=True,exist_ok=True)
results=[]
for state in ('success','wrong','cancel'):
 scratch=root/state;scratch.mkdir(exist_ok=True)
 box=scratch/'inbox.maildir'
 for sub in ('cur','new','tmp'):(box/sub).mkdir(parents=True,exist_ok=True)
 (scratch/'gpg_home').mkdir(exist_ok=True)
 (box/'cur/encrypted;2,S').write_text('From: sender@example.invalid\nTo: reader@example.invalid\nSubject: Encrypted MIME fixture\nMIME-Version: 1.0\nContent-Type: multipart/encrypted; protocol="application/pgp-encrypted"; boundary="fixture"\n\n--fixture\nContent-Type: application/pgp-encrypted\n\nVersion: 1\n\n--fixture\nContent-Type: application/octet-stream\nContent-Transfer-Encoding: base64\n\n'+base64.encodebytes(a.cipher.read_bytes()).decode()+'\n--fixture--\n')
 command='gpg --homedir ./gpg_home --batch --quiet --no-symkey-cache --pinentry-mode loopback --passphrase-fd 0 --status-fd 2 --decrypt %f'
 rc=scratch/'test.rc';rc.write_text('set folder="."\nset maildir_field_delimiter=";"\nset charset="utf-8"\nset quit=yes\nset move=no\nset wait_key=no\nset sleep_time=0\nunset beep\nunset pgp_use_gpg_agent\nset crypt_use_gpgme=no\nbind editor <space> noop\nset pgp_decrypt_command="'+command+'"\nset pgp_decode_command="'+command+'"\n')
 master,slave=pty.openpty();fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack('HHHH',40,120,0,0))
 def setup():
  os.setsid();fcntl.ioctl(0,termios.TIOCSCTTY,0)
 child=subprocess.Popen([str(a.app.resolve()),'-n','-F',str(rc),'-f',str(box)],cwd=scratch,stdin=slave,stdout=slave,stderr=slave,preexec_fn=setup,env=dict(os.environ,HOME=str(scratch),TMPDIR=str(scratch),TERM='xterm-256color',LANG='C.UTF-8'))
 os.close(slave);data=bytearray()
 def pump(duration):
  end=time.monotonic()+duration
  while time.monotonic()<end:
   if select.select([master],[],[],.05)[0]:
    try:block=os.read(master,65536)
    except OSError as e:
     if e.errno==errno.EIO:return
     raise
    if not block:return
    data.extend(block)
    if b'\x1b[6n' in block:os.write(master,b'\x1b[1;1R')
    if b'\x1b[c' in block:os.write(master,b'\x1b[?1;2c')
 def until(token):
  end=time.monotonic()+15
  while token.encode() not in data and child.poll() is None and time.monotonic()<end:pump(.05)
  if token.encode() not in data:raise RuntimeError('Missing '+token+'; '+repr(bytes(data[-1500:])))
 try:
  until('Encrypted MIME fixture');data.clear();os.write(master,b'\r');until('Enter PGP passphrase:');(scratch/'passphrase_prompt.vt').write_bytes(data)
  data.clear();os.write(master,b'\x07' if state=='cancel' else (b'fixture-only-passphrase' if state=='success' else b'incorrect-fixture-passphrase')+b'\r')
  if state=='success':
   until('Encrypted viewer fixture secret');until('DECRYPTION_OKAY')
   assert b'fixture-only-passphrase' not in data,'Passphrase echoed'
   (scratch/'decrypted_pager.vt').write_bytes(data);os.write(master,b'q');pump(.7)
  else:
   if state=='wrong':until('Decryption failed')
   pump(.7);assert b'Encrypted viewer fixture secret' not in data,'Plaintext exposed on rejected input'
  os.write(master,b'q');child.wait(timeout=5);assert child.returncode==0,child.returncode
  results.append({'state':state,'passed':True,'exit':child.returncode})
 except Exception as e:results.append({'state':state,'passed':False,'error':str(e)})
 finally:
  (scratch/'final.vt').write_bytes(data)
  if child.poll() is None:child.kill();child.wait()
  os.close(master)
 (root/'results.json').write_text(json.dumps(results,indent=2)+'\n')
print(json.dumps(results,indent=2))
raise SystemExit(0 if all(r['passed'] for r in results) else 1)

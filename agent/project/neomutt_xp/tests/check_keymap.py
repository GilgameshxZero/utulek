#!/usr/bin/env python3
"""Export effective bindings/macros from a real Linux NeoMutt session."""
import argparse,errno,fcntl,os,pty,select,struct,subprocess,termios,time
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('app');p.add_argument('config',type=Path);p.add_argument('output',type=Path);a=p.parse_args()
r=a.output.resolve();r.mkdir(parents=True,exist_ok=True);box=r/'fixture.maildir'
for d in ('cur','new','tmp'):(box/d).mkdir(parents=True,exist_ok=True)
(box/'cur/fixture;2,S').write_text('From: fixture@example.invalid\nSubject: Keymap fixture\n\nKeymap fixture body\n')
m,s=pty.openpty();fcntl.ioctl(s,termios.TIOCSWINSZ,struct.pack('HHHH',30,120,0,0))
def setup():os.setsid();fcntl.ioctl(0,termios.TIOCSCTTY,0)
child=subprocess.Popen([a.app,'-F',str(a.config.resolve()),'-f',str(box),'-e','set quit=yes'],stdin=s,stdout=s,stderr=s,preexec_fn=setup,env=dict(os.environ,TERM='xterm-256color'));os.close(s);data=bytearray()
def pump(t):
 end=time.monotonic()+t
 while time.monotonic()<end:
  if select.select([m],[],[],.05)[0]:
   try:b=os.read(m,65536)
   except OSError as e:
    if e.errno==errno.EIO:return
    raise
   if not b:return
   data.extend(b)
   if b'\x1b[6n' in b:os.write(m,b'\x1b[1;1R')
   if b'\x1b[c' in b:os.write(m,b'\x1b[?1;2c')
try:
 start=time.monotonic()
 while b'Keymap fixture' not in data and time.monotonic()-start<15:pump(.1)
 assert b'Keymap fixture' in data,bytes(data[-1000:])
 for kind in ('bind','macro'):
  output=r/(kind+'.rc');pager='python3 '+str(Path(__file__).with_name('capture_keymap.py').resolve())+' '+str(output)
  os.write(m,(':set pager="'+pager+'"\r').encode());pump(.25);os.write(m,(':'+kind+'\r').encode())
  start=time.monotonic()
  while not output.exists() and time.monotonic()-start<10:pump(.1)
  assert output.exists(),bytes(data[-1000:]);pump(.5)
 os.write(m,b'q');child.wait(timeout=5);assert child.returncode==0
 print('Effective bind and macro exports saved:',r)
finally:
 (r/'terminal.vt').write_bytes(data)
 if child.poll() is None:child.kill();child.wait()
 os.close(m)

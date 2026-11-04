#!/usr/bin/env python3
"""Measure upstream and port behavior for local/external Maildir flag conflict."""
import argparse,errno,fcntl,json,os,pty,select,struct,subprocess,termios,time
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('app',type=Path);p.add_argument('root',type=Path);a=p.parse_args();root=a.root.resolve();root.mkdir(parents=True,exist_ok=True);box=root/'inbox.maildir'
for d in ['cur','new','tmp']:(box/d).mkdir(parents=True,exist_ok=True)
message=b'From: sender@example.invalid\nSubject: Flag conflict fixture\n\nOwned body\n';source=box/'cur/fixture;2,S';source.write_bytes(message)
rc=root/'test.rc';rc.write_text('set folder="." mbox_type=Maildir charset="utf-8" maildir_field_delimiter=";" quit=yes move=no mark_old=no wait_key=no sleep_time=0 mail_check=0 timeout=1\nunset beep\nunset record\n')
master,slave=pty.openpty();fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack('HHHH',40,120,0,0))
def setup():os.setsid();fcntl.ioctl(0,termios.TIOCSCTTY,0)
proc=subprocess.Popen([str(a.app.resolve()),'-n','-F',str(rc),'-f',str(box)],cwd=root,stdin=slave,stdout=slave,stderr=slave,preexec_fn=setup,env=dict(os.environ,HOME=str(root),TMPDIR=str(root),TERM='xterm-256color',LANG='C.UTF-8'));os.close(slave);data=bytearray()
def pump(duration):
 end=time.monotonic()+duration
 while time.monotonic()<end:
  if select.select([master],[],[],.03)[0]:
   try:chunk=os.read(master,65536)
   except OSError as e:
    if e.errno==errno.EIO:return
    raise
   data.extend(chunk)
   if b'\x1b[6n' in chunk:os.write(master,b'\x1b[1;1R')
   if b'\x1b[c' in chunk:os.write(master,b'\x1b[?1;2c')
try:
 end=time.monotonic()+10
 while b'Flag conflict fixture' not in data and time.monotonic()<end:pump(.05)
 assert b'Flag conflict fixture' in data
 os.write(master,b'F');pump(.2);before=next((box/'cur').iterdir());flags=before.name.split(';2,')[1];external=before.with_name(before.name.split(';2,')[0]+';2,'+''.join(sorted(set(flags+'R'))));before.rename(external)
 os.write(master,b'$');pump(.4);os.write(master,b'q');pump(.6);proc.wait(timeout=5)
 files=list((box/'cur').iterdir());assert len(files)==1 and files[0].read_bytes()==message
 result={'exit':proc.returncode,'before_external':before.name,'final':files[0].name,'local_flag_preserved':'F' in files[0].name.split(';2,')[1],'external_reply_preserved':'R' in files[0].name.split(';2,')[1],'original_message_unchanged':True}
 (root/'results.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
finally:
 (root/'terminal.vt').write_bytes(data)
 if proc.poll() is None:proc.kill();proc.wait()
 os.close(master)

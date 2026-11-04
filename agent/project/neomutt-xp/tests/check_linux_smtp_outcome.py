#!/usr/bin/env python3
"""Compose/send keyboard checks against loopback-only SMTP outcomes."""
import argparse,errno,fcntl,json,os,pty,select,struct,subprocess,termios,time
from pathlib import Path
from mail_fixture import SmtpFixture
p=argparse.ArgumentParser();p.add_argument('app',type=Path);p.add_argument('root',type=Path);p.add_argument('--case');a=p.parse_args();root=a.root.resolve();root.mkdir(parents=True,exist_ok=True);results=[]
for state in ([a.case] if a.case else ['lost_ack','accepted_ack','rejected_data','record_failure_alternate']):
 d=root/state;d.mkdir(exist_ok=True);box=d/'source.maildir'
 for sub in ['cur','new','tmp']:(box/sub).mkdir(parents=True,exist_ok=True)
 source=box/'cur/source;2,S';original='From: outside@example.invalid\nSubject: SMTP UI fixture source\n\nOwned source\n';source.write_text(original);record=d/'record.maildir';blocked=d/'blocked.mbox';blocked_bytes=b'From fixture@example.invalid Tue Nov  3 00:00:00 2026\nFrom: fixture@example.invalid\nSubject: Existing archive\n\nSentinel\n\n'
 if state=='record_failure_alternate':blocked.write_bytes(blocked_bytes);blocked.chmod(0o444)
 initial_record=blocked if state=='record_failure_alternate' else record
 with SmtpFixture(drop_ack=state=='lost_ack',drop_after_ack=state in ['accepted_ack','record_failure_alternate'],reject_data=state=='rejected_data') as server:
  rc=d/'test.rc';rc.write_text('set folder="." mbox_type=Maildir maildir_field_delimiter=";" charset="utf-8" from="reader@example.invalid" hostname="fixture.example.invalid" quit=yes postpone=no recall=no move=no mark_old=no wait_key=no sleep_time=0 confirm_create=no confirm_append=no\nset editor="'+str(Path(__file__).resolve().parents[1]/'.obj/fixture_editor')+'"\nset smtp_url="smtp://fixture@127.0.0.1:'+str(server.server_address[1])+'" smtp_pass="fixture-pass" smtp_authenticators=plain ssl_force_tls=no ssl_starttls=no socket_timeout=5 record="'+str(initial_record)+'" copy=yes\n')
  master,slave=pty.openpty();fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack('HHHH',40,120,0,0))
  def setup():os.setsid();fcntl.ioctl(0,termios.TIOCSCTTY,0)
  child=subprocess.Popen([str(a.app.resolve()),'-n','-F',str(rc),'-f',str(box)],cwd=d,stdin=slave,stdout=slave,stderr=slave,preexec_fn=setup,env=dict(os.environ,HOME=str(d),TMPDIR=str(d),TERM='xterm-256color',LANG='C.UTF-8'));os.close(slave);data=bytearray()
  def send(s):os.write(master,s.encode())
  def pump(duration):
   end=time.monotonic()+duration
   while time.monotonic()<end:
    if select.select([master],[],[],.03)[0]:
     try:chunk=os.read(master,65536)
     except OSError as e:
      if e.errno==errno.EIO:return
      raise
     data.extend(chunk)
     if b'\x1b[6n' in chunk:send('\x1b[1;1R')
     if b'\x1b[c' in chunk:send('\x1b[?1;2c')
  def until(token):
   end=time.monotonic()+15
   while token.encode() not in data and child.poll() is None and time.monotonic()<end:pump(.05)
   if token.encode() not in data:raise RuntimeError('Missing '+token+' '+repr(data[-1200:]))
  try:
   until('SMTP UI fixture source');send('m');until('To:');send('recipient@example.invalid\r');until('Subject:');send('SMTP outcome '+state+'\r');until('Attachments');data.clear();send('y')
   if state in ['accepted_ack','record_failure_alternate']:
    if state=='record_failure_alternate':until('Fcc failed.');send('m');until('Fcc mailbox');send('\x15record.maildir\r')
    until('Mail sent');send('q')
   else:
    until('delivery status is unknown' if state=='lost_ack' else 'fixture DATA rejected');pump(.7)
    if state=='rejected_data':assert b'delivery status is unknown' not in data
    data.clear();send(':exec write-fcc\r');until('Write message to mailbox');send('\x15retained.maildir\r');pump(.5)
    retained=[f for sub in ['cur','new'] if (d/'retained.maildir'/sub).exists() for f in (d/'retained.maildir'/sub).iterdir()]
    assert len(retained)==1 and b'Editor fixture' in retained[0].read_bytes(), 'Failed submission lost draft body'
    data.clear();send('q');until('Mail not sent');send('q')
   pump(.6);child.wait(timeout=5);assert child.returncode==0
   records=[f for sub in ['cur','new'] if (record/sub).exists() for f in (record/sub).iterdir()];captured=sum(len(s['messages']) for s in server.sessions)
   assert len(server.sessions)==1 and captured==(0 if state=='rejected_data' else 1)
   assert len(records)==(1 if state in ['accepted_ack','record_failure_alternate'] else 0) and source.read_text()==original
   if state=='record_failure_alternate':assert blocked.read_bytes()==blocked_bytes
   results.append({'state':state,'passed':True,'connections':len(server.sessions),'captured_messages':captured,'archive_copies':len(records),'draft_preserved':state not in ['accepted_ack','record_failure_alternate']})
  except Exception as e:results.append({'state':state,'passed':False,'error':str(e)})
  finally:
   if state=='record_failure_alternate':blocked.chmod(0o600)
   (d/'terminal.vt').write_bytes(data)
   if child.poll() is None:child.kill();child.wait()
   os.close(master)
  print(json.dumps(results[-1]),flush=True)
(root/'results.json').write_text(json.dumps(results,indent=2)+'\n');raise SystemExit(0 if all(r['passed'] for r in results) else 1)

#!/usr/bin/env python3
"""Exercise the original calendar reply macro with default/alternate identities.

The test exports a draft only after the original macro completes, to compare
its From header with the generated attendee. No message is sent.
"""
import argparse,email,json,os,pty,select,fcntl,termios,struct,subprocess,time,errno
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('app',type=Path);p.add_argument('helpers',type=Path);p.add_argument('output',type=Path);a=p.parse_args()
root=a.output.resolve();root.mkdir(parents=True,exist_ok=True);results=[]
for name,recipient in [('default','default@example.invalid'),('alternate','alternate@example.invalid')]:
 d=root/name;d.mkdir(exist_ok=True);box=d/'inbox.maildir'
 for sub in ['cur','new','tmp']:(box/sub).mkdir(parents=True,exist_ok=True)
 ics='BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Fixture//EN\r\nMETHOD:REQUEST\r\nBEGIN:VEVENT\r\nUID:identity@example.invalid\r\nDTSTAMP:20261103T000000Z\r\nDTSTART:20300501T100000Z\r\nDTEND:20300501T110000Z\r\nSUMMARY:Identity fixture\r\nORGANIZER:mailto:organizer@example.invalid\r\nATTENDEE;RSVP=TRUE:mailto:'+recipient+'\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n'
 (box/'cur/fixture;2,S').write_text('From: organizer@example.invalid\nTo: '+recipient+'\nSubject: Calendar identity fixture\nMIME-Version: 1.0\nContent-Type: text/calendar; charset=utf-8; method=REQUEST\n\n'+ics)
 mailcap=d/'mailcap';mailcap.write_text('text/calendar; true; compose=true\n')
 post='<new-mime>.reply.tmp.ics<enter>text/calendar<enter><shell-escape>python3 -X utf8 \\"'+str(a.helpers.resolve()/'mutt-ical.py')+'\\" -i -g .reply.tmp.ics .event.tmp.ics<enter><shell-escape>rm .event.tmp.ics<enter><last-entry><update-encoding>'
 rc=d/'test.rc';rc.write_text('set folder="." crypt_use_gpgme=no maildir_field_delimiter=";" charset="utf-8" from="default@example.invalid" reverse_name=yes editor=true include=yes fast_reply=yes abort_unmodified=no quit=yes postpone=no recall=no wait_key=no sleep_time=0\nunset record\nalternates ^default@example\\.invalid$ ^alternate@example\\.invalid$\nset mailcap_path="'+str(mailcap)+'"\nsend2-hook "~f .*" "setenv NEOMUTT_FROM=\'$from\'"\nmacro attach r "<pipe-entry>iconv -c --to-code=UTF-8 > .event.tmp.ics<enter><reply>'+post+'"\nmacro compose Z "<write-fcc><kill-line>observed_draft.mbox<enter>"\nset confirm_create=no confirm_append=no mbox_type=mbox\n')
 master,slave=pty.openpty();fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack('HHHH',40,120,0,0))
 def setup():os.setsid();fcntl.ioctl(0,termios.TIOCSCTTY,0)
 proc=subprocess.Popen([str(a.app.resolve()),'-n','-F',str(rc),'-f',str(box)],cwd=d,stdin=slave,stdout=slave,stderr=slave,env=dict(os.environ,HOME=str(d),TMPDIR=str(d),TERM='xterm-256color',LANG='C.UTF-8'),preexec_fn=setup);os.close(slave);data=bytearray()
 def send(s):os.write(master,s.encode())
 def until(token):
  end=time.monotonic()+15
  while token.encode() not in data and proc.poll() is None and time.monotonic()<end:
   if select.select([master],[],[],.05)[0]:
    try:chunk=os.read(master,65536)
    except OSError as e:
     if e.errno==errno.EIO:break
     raise
    data.extend(chunk)
    if b'\x1b[6n' in chunk:send('\x1b[1;1R')
    if b'\x1b[c' in chunk:send('\x1b[?1;2c')
  if token.encode() not in data:raise RuntimeError('Missing '+token+' '+repr(data[-1000:]))
 try:
  until('Calendar identity fixture');send('v');until('Attachments');data.clear();send('r');until('Accept Invitation?');data.clear();send('y\r');until('Attachments');send('Z')
  end=time.monotonic()+8
  while not (d/'observed_draft.mbox').exists() and time.monotonic()<end:time.sleep(.05)
  msg=email.message_from_bytes((d/'observed_draft.mbox').read_bytes());actual=email.utils.parseaddr(msg['From'])[1]
  assert any(part.get_content_type()=='text/calendar' for part in msg.walk()), 'Reply macro did not attach calendar'
  answer=(d/'.reply.tmp.ics').read_text().replace('\n ','').replace('\n\t','');match='mailto:'+actual in answer
  results.append({'case':name,'draft_from':actual,'calendar_matches_draft':match,'expected_original_behavior':match==(name=='default'),'macro_used_compose_message':False})
  send('q');data.clear();until('Mail not sent');send('qq');proc.wait(timeout=5)
 except Exception as e:results.append({'case':name,'error':str(e),'expected_original_behavior':False})
 finally:
  (d/'terminal.vt').write_bytes(data)
  if proc.poll() is None:proc.kill();proc.wait()
  os.close(master)
(root/'results.json').write_text(json.dumps(results,indent=2)+'\n');print(json.dumps(results,indent=2));raise SystemExit(0 if all(r['expected_original_behavior'] for r in results) else 1)

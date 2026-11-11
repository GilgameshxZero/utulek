#!/usr/bin/env python3
"""Exercise shipped calendar reply macros and inspect the resulting MIME draft.

Owned synthetic invitations only; no messages are sent.
"""
import argparse,email,json,os,pty,select,fcntl,termios,struct,subprocess,time,errno
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('app',type=Path);p.add_argument('helpers',type=Path);p.add_argument('output',type=Path);p.add_argument('--config',type=Path,required=True);p.add_argument('--mime-type',default='text/calendar',choices=['text/calendar','application/ics']);a=p.parse_args()
root=a.output.resolve();root.mkdir(parents=True,exist_ok=True);results=[]
for name,key,choice,state in [(k+'_'+s,k,c,s) for k in ['r','R'] for c,s in [('y','ACCEPTED'),('n','DECLINED'),('t','TENTATIVE'),('c','CANCELLED')]] + [('request','R',None,None)]:
 recipient='default@example.invalid'
 d=root/name;d.mkdir(exist_ok=True);box=d/'inbox.maildir'
 for sub in ['cur','new','tmp']:(box/sub).mkdir(parents=True,exist_ok=True)
 ics='BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Fixture//EN\r\nMETHOD:REQUEST\r\nBEGIN:VEVENT\r\nUID:identity@example.invalid\r\nDTSTAMP:20261103T000000Z\r\nDTSTART:20300501T100000Z\r\nDTEND:20300501T110000Z\r\nSEQUENCE:7\r\nRECURRENCE-ID:20300501T100000Z\r\nSUMMARY:Identity fixture\r\nORGANIZER:mailto:organizer@example.invalid\r\nATTENDEE;RSVP=TRUE:mailto:'+recipient+'\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n'
 (box/'cur/fixture;2,S').write_text('From: organizer@example.invalid\nTo: '+recipient+'\nSubject: Calendar identity fixture\nMIME-Version: 1.0\nContent-Type: '+a.mime_type+'; charset=utf-8; method=REQUEST\n\n'+ics)
 mailcap=d/'mailcap';mailcap.write_text('text/calendar; true; compose=true\n')
 config=a.config.read_text();start=config.index('macro attach r ');end=config.index('# Save ICS request.',start)
 macros=(config[start:end]+config[config.index('macro compose R '):config.index('# Swap N')]).replace('${my_active_dir}',str(a.helpers.resolve())+'/')
 rc=d/'test.rc';rc.write_text('set folder="." crypt_use_gpgme=no maildir_field_delimiter=";" charset="utf-8" from="default@example.invalid" reverse_name=yes editor=true include=yes fast_reply=yes abort_unmodified=no quit=yes postpone=no recall=no wait_key=no sleep_time=0\nunset record\nalternates ^default@example\\.invalid$ ^alternate@example\\.invalid$\nset mailcap_path="'+str(mailcap)+'"\nsend2-hook "~f .*" "setenv NEOMUTT_FROM=\'$from\'"\n'+macros+'\nmacro compose K "<detach-file>"\nmacro compose Z "<write-fcc><kill-line>observed_draft.mbox<enter>"\nset confirm_create=no confirm_append=no mbox_type=mbox\n')
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
  until('Calendar identity fixture')
  if name=='request':
   send('m');until('To:');send('target@example.invalid\r');until('Subject:');send('Owned new invitation\r');until('Attachments');data.clear();send('R');until('Subject [');send('\r');until('Start (YYYY-MM-DD HH:MM):');send('2030-05-01 10:00\r');until('Duration (');data.clear();send('1h30m\r');until('Attachments')
  else:
   send('v');until('Attachments');data.clear();send(key);until('Accept Invitation?');data.clear();send(choice+'\r');
   if state=='CANCELLED':
    until('Press any key to continue');data.clear();send(' ');until('Attachments');send('K')
   until('Attachments')
  send('Z')
  end=time.monotonic()+8
  while not (d/'observed_draft.mbox').exists() and time.monotonic()<end:time.sleep(.05)
  msg=email.message_from_bytes((d/'observed_draft.mbox').read_bytes());actual=email.utils.parseaddr(msg['From'])[1]
  if state=='CANCELLED':
   assert any(part.get_content_type()=='text/plain' and b'organizer@example.invalid wrote:' in (part.get_payload(decode=True) or b'') for part in msg.walk()), 'Cancellation lost the original quoted body'
   assert not list((d/'neomutt-ical').glob('reply-*/reply.ics')), 'Cancelled reply remains'
   results.append({'case':name,'checks':{'cancel_preserves_body':True},'passed':True})
   send('q');data.clear();until('Mail not sent');send('qq');proc.wait(timeout=5)
   continue
  assert any(part.get_content_type()=='text/calendar' for part in msg.walk()), 'Reply macro did not attach calendar'
  answer_file=(d/'.request.tmp.ics') if name=='request' else next((d/'neomutt-ical').glob('reply-*/reply.ics'))
  answer=answer_file.read_text().replace('\n ','').replace('\n\t','')
  calendar=[part for part in msg.walk() if part.get_content_type()=='text/calendar'][0]
  checks={'mime_method_parameter':calendar.get_param('method')==('REQUEST' if name=='request' else 'REPLY'), 'body_matches_attachment':calendar.get_payload(decode=True).decode(calendar.get_content_charset() or 'utf-8').replace('\r\n','\n').rstrip('\n')==answer_file.read_text().rstrip('\n'), 'recipient':email.utils.parseaddr(msg['To'])[1]==('target@example.invalid' if name=='request' else 'organizer@example.invalid')}
  if name=='request':checks.update({'method':'METHOD:REQUEST' in answer,'organizer':'ORGANIZER:mailto:default@example.invalid' in answer,'attendee':'mailto:target@example.invalid' in answer,'duration':'DURATION:PT1H30M' in answer,'draft_cleaned':not (d/'.compose.tmp.mbox').exists()})
  else:checks.update({'calendar_primary':msg.get_content_type()=='text/calendar' and not msg.is_multipart(),'calendar_inline':calendar.get_content_disposition()=='inline','state':'PARTSTAT='+state in answer,'revision':'SEQUENCE:7' in answer,'instance':'RECURRENCE-ID:20300501T100000Z' in answer})
  if name=='r_ACCEPTED':
   (d/'extra.txt').write_text('Owned confirmation fixture\n')
   data.clear();send(':exec attach-file\r');until('Attach file');data.clear();send('extra.txt\r');until('Attachments')
   data.clear();send(':exec first-entry\r:exec detach-file\r');until('Really delete the main message?');send('n')
   checks['detach_confirmation_restored']=True
  results.append({'case':name,'checks':checks,'passed':all(checks.values())})
  send('q');data.clear();until('Mail not sent');send('qq');proc.wait(timeout=5)
 except Exception as e:results.append({'case':name,'error':str(e),'passed':False})
 finally:
  
  if any(not r['passed'] for r in results if r['case']==name):(d/'terminal.vt').write_bytes(data)
  if proc.poll() is None:proc.kill();proc.wait()
  os.close(master)
(root/'results.json').write_text(json.dumps(results,indent=2)+'\n');print(json.dumps(results,indent=2));raise SystemExit(0 if all(r['passed'] for r in results) else 1)

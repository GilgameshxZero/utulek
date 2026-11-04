#!/usr/bin/env python3
"""Mirror installed configuration with synthetic credentials/mailboxes/calendar.

No production encrypted file, mailbox, address book or calendar is copied.
The original configuration is read as data; its credential pipe is replaced.
"""
from pathlib import Path
import json,re,shutil,sys
source_module,root,system,terminal=sys.argv[1:]
source_module=Path(source_module);root=Path(root).resolve();home=root/'home';module=home/'main.syncthing/monochrome/module/email';module.mkdir(parents=True,exist_ok=True)
source=(source_module/'neomuttrc').read_text(encoding='utf-8')
lines=[l for l in source.splitlines() if l.lstrip().startswith('source ')]
if len(lines)!=1 or not re.search(r'gpg .*--decrypt',lines[0]):raise SystemExit('Unexpected credential source')
credentials='\n'.join('set '+n+'="fixture-only-password"' for n in sorted(set(re.findall(r'\$([a-z_]*smtp_pass[a-z_]*)',source))))
source=source.replace(lines[0],credentials)
for file in source_module.iterdir():
    if file.is_file() and (file.suffix=='.py' or file.name in ['neomutt-html-open','neomutt-html-open.bat','neomutt-ical-request','neomutt-ical-request.bat','neomutt-query','neomutt-query.bat','mailcap','timezones.zip']):shutil.copy2(file,module/file.name)
if (source_module/'tzdata').is_dir():shutil.copytree(source_module/'tzdata',module/'tzdata',dirs_exist_ok=True)
(module/'.signature').write_text('Synthetic fixture signature\n')
(home/'main.syncthing/snowfall').mkdir(parents=True,exist_ok=True)
for match in re.findall(r'~/([^\"\s]*\.maildir/?)',source):
    for sub in ['cur','new','tmp']:(home/match/sub).mkdir(parents=True,exist_ok=True)
normalized=re.sub(r'\\\n\s*','',source)
spool=re.search(r'set spool_file\s*=\s*"~/([^"]+)"',normalized)
if not spool:raise SystemExit('Expected a HOME-relative configured local inbox')
boxes={name:home/'main.syncthing'/relative for name,relative in dict(archive='monochrome/archive.syncthing/module/email/archive.maildir',postponed='monochrome/module/email/postponed.maildir',vault='monochrome/local.syncthing/module/email/vault.maildir').items()}
boxes['inbox']=home/spool[1]
for box in boxes.values():
    for sub in ['cur','new','tmp']:(box/sub).mkdir(parents=True,exist_ok=True)
calendar=home/'main.syncthing/monochrome/module/calendar/default';calendar.mkdir(parents=True,exist_ok=True)
uid='synthetic-config-calendar@example.invalid';subject='Configuration calendar fixture';summary='Calendar fixture 日本語'
ics='BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Fixture//EN\r\nMETHOD:REQUEST\r\nBEGIN:VEVENT\r\nUID:'+uid+'\r\nDTSTAMP:20261103T000000Z\r\nDTSTART:20300501T100000Z\r\nDTEND:20300501T110000Z\r\nSUMMARY:'+summary+'\r\nORGANIZER:mailto:sender@example.invalid\r\nATTENDEE;RSVP=TRUE:mailto:reader@example.invalid\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n'
msg='From: sender@example.invalid\nTo: reader@example.invalid\nDate: Tue, 03 Nov 2026 00:00:00 +0000\nMessage-ID: <'+uid+'>\nSubject: '+subject+'\nMIME-Version: 1.0\nContent-Type: text/calendar; charset=utf-8; method=REQUEST\nContent-Transfer-Encoding: 8bit\n\n'+ics
for box in boxes.values():(box/'cur/fixture;2,S').write_text(msg,encoding='utf-8')
(module/'neomuttrc').write_text(source+'\nset crypt_use_gpgme=no from="reader@example.invalid"\nalternates ^reader@example\\.invalid$\n',encoding='utf-8')
config=root/'test.rc';config.write_text('source "'+str(system).replace('\\','/')+'"\nsource "'+(module/'neomuttrc').as_posix()+'"\n',encoding='utf-8')
settings=dict(monochromeDirectory=str(home/'main.syncthing/monochrome'),calendarSummary=summary,terminalModule=terminal,sender='reader@example.invalid',replyIdentity='reader@example.invalid',inbox=str(boxes['inbox']),subject=subject,messageId=uid,uid=uid,calendarDirectory=str(calendar),mailboxes=[dict(name=n,search=n,status=n+'.maildir') for n in ['archive','postponed','vault','inbox']])
(root/'settings.json').write_text(json.dumps(settings,indent=2));print('Synthetic configuration fixture ready')

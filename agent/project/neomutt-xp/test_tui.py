#!/usr/bin/env python3
"""Exercise original and adapted attachment macros in disposable Linux TUIs."""
import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import pty
import re
import select
import shutil
import struct
import subprocess
import termios
import time
from stage_config import stage


class Terminal:
    def __init__(self, config, inbox, environment):
        self.master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 40, 140, 0, 0))
        def setup():
            os.setsid()
            fcntl.ioctl(slave, termios.TIOCSCTTY, 0)
        self.process = subprocess.Popen(['neomutt','-n','-d','5','-l',str(config.with_suffix('.debug')),'-F',str(config),'-f',str(inbox)],
            stdin=slave,stdout=slave,stderr=slave,env=environment,preexec_fn=setup)
        os.close(slave)
        self.output = bytearray()

    def read(self, seconds=0.2):
        end = time.monotonic() + seconds
        while time.monotonic() < end:
            ready,_,_ = select.select([self.master],[],[],min(0.05,max(0,end-time.monotonic())))
            if ready:
                try:
                    data=os.read(self.master,65536)
                except OSError:
                    return
                self.output.extend(data)

    def send(self, value):
        os.write(self.master,value.encode())
        self.read()

    def wait(self, marker, timeout=8):
        start = len(self.output)
        end=time.monotonic()+timeout
        while time.monotonic()<end:
            self.read(0.1)
            if marker.encode() in self.output[start:]:
                return
            if self.process.poll() is not None:
                break
        raise AssertionError('Terminal did not display ' + repr(marker))

    def close(self, log):
        self.process.terminate()
        try:
            self.process.wait(timeout=3)
        except subprocess.TimeoutExpired:
            self.process.kill()
            self.process.wait()
        self.read(0.1)
        log.write_bytes(self.output)
        os.close(self.master)


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('source',type=Path)
    parser.add_argument('output',type=Path)
    parser.add_argument('--normalize-macro-whitespace',action='store_true',
                        help='Diagnostic fixture only: replace indentation tabs in both configs')
    parser.add_argument('--populated-working-directory',action='store_true',
                        help='Add an ordinary filename to test empty-prefix completion')
    args=parser.parse_args()
    root=args.output.resolve()
    root.mkdir(parents=True,exist_ok=True)
    original=(args.source/'neomuttrc').read_text()
    results=[]
    def fixture(label):
        home=root/label/'home'
        email=home/'main.syncthing/monochrome/module/email'
        stage(args.source,email)
        work=home/'main.syncthing/snowfall'
        work.mkdir(parents=True,exist_ok=True)
        if args.populated_working_directory:
            (work/'ordinary_fixture.txt').write_text('Fixture only\n')
            (work/'another_fixture.txt').write_text('Fixture only\n')
        calendar=root/label/'saved_calendar'
        calendar.mkdir(parents=True,exist_ok=True)
        inbox=root/label/'inbox.maildir'
        archive=root/label/'archive'
        postponed=email/'postponed.maildir'
        vault=home/re.search(r'vault `ls -d "\$HOME/(.*?)"`',original)[1]
        for folder in (inbox,archive/'archive.maildir',postponed,vault):
            for part in ('cur','new','tmp'):
                (folder/part).mkdir(parents=True,exist_ok=True)
        invitation=(b'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nMETHOD:REQUEST\r\n'
            b'BEGIN:VEVENT\r\nUID:fixture-calendar\r\nSUMMARY:Fixture invitation\r\n'
            b'DTSTART:20300102T120000Z\r\nDURATION:PT1H\r\n'
            b'ORGANIZER:mailto:organizer@example.test\r\n'
            b'ATTENDEE:mailto:sender@example.test\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n')
        message=(b'From: Organizer <organizer@example.test>\nTo: sender@example.test\n'
            b'Date: Tue, 1 Jan 2030 00:00:00 +0000\nSubject: Fixture invitation\n'
            b'Message-ID: <fixture@example.test>\nMIME-Version: 1.0\n'
            b'Content-Type: multipart/mixed; boundary="fixture-boundary"\n\n'
            b'--fixture-boundary\nContent-Type: text/plain; charset=utf-8\n\nInvitation fixture.\n'
            b'--fixture-boundary\nContent-Type: application/ics; name="invite.ics"\n'
            b'Content-Disposition: attachment; filename="invite.ics"\n\n'+invitation+
            b'\n--fixture-boundary--\n')
        (inbox/'new/fixture').write_bytes(message)
        names=sorted(set(re.findall(r'my_smtp_pass_[a-z_]+',original)))
        dummy='\n'.join('set '+name+'=""' for name in names)
        tools=root/label/'tools'
        tools.mkdir(parents=True,exist_ok=True)
        for name,code in {'gpg':'cat <<\'FIXTURE\'\n'+dummy+'\nFIXTURE\n',
                          'fixture-editor':'exit 0\n'}.items():
            path=tools/name
            path.write_text('#!/usr/bin/env bash\n'+code)
            path.chmod(0o755)
        text=original if label=='original' else (email/'neomuttrc').read_text()
        text=re.sub(r'^set my_active_dir=.*$',lambda _:'set my_active_dir="'+str(email)+'/"',text,flags=re.M)
        text=re.sub(r'^set my_archive_dir=.*$',lambda _:'set my_archive_dir="'+str(archive)+'/"',text,flags=re.M)
        text=re.sub(r'^set spool_file=\\\n\s*"[^"]*"',lambda _:'set spool_file="'+str(inbox)+'"',text,flags=re.M)
        if label=='original':
            text=re.sub(r'^source "`gpg .*?(?:\n.*?)*?&`"$',lambda _:dummy,text,flags=re.M)
            text=re.sub(r'(mv \.event\.tmp\.ics )[^\s]*/\$\(uuidgen\)\.ics',lambda m:m[1]+str(calendar)+'/$(uuidgen).ics',text)
            mailcap=(args.source/'mailcap').read_text()
            mailcap=re.sub(r'[^;\s]*mutt-ical\.py',str(email/'mutt-ical.py'),mailcap)
            (email/'mailcap').write_text(mailcap)
        else:
            text=re.sub(r'(save \.event\.tmp\.ics \\")[^"\n]*?(\\")',lambda m:m[1]+str(calendar)+m[2],text)
        (email/'.signature').write_text('Fixture signature\n')
        text+='\nset from="sender@example.test"\nset real_name="Fixture Sender"\n'
        text+='set editor="'+str(tools/'fixture-editor')+'"\n'
        text+='unset smtp_url\nset sendmail="/bin/false"\n'
        if args.normalize_macro_whitespace:
            text=text.replace('\t',' ')
        config=root/label/'fixture.neomuttrc'
        config.write_text(text)
        env={**os.environ,'HOME':str(home),'TERM':'xterm-256color',
             'PATH':str(tools)+os.pathsep+os.environ['PATH'],'NEOMUTT_FROM':'sender@example.test'}
        return config,inbox,env,work,calendar
    for label in ('original','candidate'):
        config,inbox,env,work,calendar=fixture(label)
        terminal=Terminal(config,inbox,env)
        try:
            terminal.read(1)
            terminal.send('v')
            terminal.send('jj')
            terminal.send('S')
            end=time.monotonic()+5
            while not list(calendar.glob('*.ics')) and time.monotonic()<end:
                terminal.read(0.1)
            files=list(calendar.glob('*.ics'))
            result={'label':label,'save_macro_created_one_ics':len(files)==1,
                    'capture_scratch_removed':not (work/'.event.tmp.ics').exists()}
            if files:
                result['saved_sha256']=hashlib.sha256(files[0].read_bytes()).hexdigest()
            results.append(result)
        except Exception as error:
            results.append({'label':label,'error':str(error)})
        finally:
            terminal.close(root/(label+'_save.terminal'))
    parity=len(results)==2 and results[0].get('saved_sha256')==results[1].get('saved_sha256') and all(
        row.get('save_macro_created_one_ics') and row.get('capture_scratch_removed') for row in results)
    report={'normalized_fixture':args.normalize_macro_whitespace,
            'populated_working_directory':args.populated_working_directory,
            'save_macro_parity':bool(parity),'results':results}
    (root/'results.json').write_text(json.dumps(report,indent=2))
    print(json.dumps(report,indent=2))
    return 0 if parity else 1


if __name__=='__main__':
    raise SystemExit(main())

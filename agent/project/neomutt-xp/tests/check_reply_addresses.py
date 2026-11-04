#!/usr/bin/env python3
"""Compare actual interactive reply drafts, without invoking any transport."""
import email, email.utils, json, os, sys, time
from pathlib import Path
import pty, subprocess, select, fcntl, termios, struct, errno
class Session:
    def __init__(self, app, args, cwd):
        self.master, slave=pty.openpty()
        fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack('HHHH',40,120,0,0))
        def setup():
            os.setsid();fcntl.ioctl(0,termios.TIOCSCTTY,0)
        self.child=subprocess.Popen([app,*args],stdin=slave,stdout=slave,stderr=slave,cwd=cwd,env=dict(os.environ,TERM='xterm-256color'),preexec_fn=setup)
        os.close(slave);self.data=b'';self.before='';self.after=''
    def send(self,s):os.write(self.master,s.encode())
    def expect(self,token):
        deadline=time.monotonic()+15
        while token.encode() not in self.data and time.monotonic()<deadline:
            if select.select([self.master],[],[],.05)[0]:
                try:block=os.read(self.master,65536)
                except OSError as e:
                    if e.errno==errno.EIO:break
                    raise
                self.data+=block
        if token.encode() not in self.data:raise RuntimeError('Missing '+token+': '+repr(self.data[-1200:]))
        before,_,rest=self.data.partition(token.encode());self.before=before.decode(errors='replace');self.after=token;self.data=rest
    def close(self):
        if self.child.poll() is None:self.child.terminate()
        self.child.wait(timeout=5);os.close(self.master)
from reply_fixture import prepare
app, root = sys.argv[1:]
cases=prepare(root,'true',';'); results=[]
for case in cases:
    child=Session(str(Path(app).resolve()),['-n','-F',case['rc'],'-f',case['box']],case['directory'])
    output=[]
    try:
        child.expect('Reply address fixture');output.append(child.before+child.after)
        if case.get('pager'): child.send('\r');child.expect('Synthetic reply body')
        child.send(case['key']);child.expect('Attachments');child.send('Z')
        deadline=time.monotonic()+10
        while not Path(case['draft']).exists() and time.monotonic()<deadline: time.sleep(.05)
        msg=email.message_from_bytes(Path(case['draft']).read_bytes())
        addresses=sorted(a.lower() for _,a in email.utils.getaddresses(msg.get_all('To',[])+msg.get_all('Cc',[])))
        passed=addresses==sorted(case['expected']);results.append(dict(name=case['name'],passed=passed,addresses=addresses,expected=case['expected']))
        child.send('q');child.expect('Mail not sent');child.send('qq' if case.get('pager') else 'q');child.child.wait(timeout=10)
    except Exception as e:results.append(dict(name=case['name'],passed=False,error=str(e)))
    finally:
        Path(case['directory'],'terminal.vt').write_bytes(child.data);child.close()
    print(json.dumps(results[-1]),flush=True)
Path(root,'results.json').write_text(json.dumps(results,indent=2));print(json.dumps(results,indent=2))
sys.exit(not all(r['passed'] for r in results))

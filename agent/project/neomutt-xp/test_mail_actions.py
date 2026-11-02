#!/usr/bin/env python3
"""Compare mailbox macros using only disposable copies of the TUI fixtures."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import site
import shlex
import socketserver
import ssl
import subprocess
import threading
import time
from test_tui import Terminal


def imap_fixture(directory):
    certificate,key=directory/'fixture.pem',directory/'fixture.key'
    subprocess.run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-days','1',
        '-subj','/CN=localhost','-addext','subjectAltName=DNS:localhost,IP:127.0.0.1',
        '-keyout',str(key),'-out',str(certificate)],capture_output=True,check=True)
    context=ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    context.load_cert_chain(certificate,key)
    class Handler(socketserver.StreamRequestHandler):
        def setup(self):
            self.request=context.wrap_socket(self.request,server_side=True)
            super().setup()
        def handle(self):
            def send(line):
                self.wfile.write((line+'\r\n').encode());self.wfile.flush()
            send('* OK [CAPABILITY IMAP4rev1 AUTH=PLAIN SASL-IR] Synthetic fixture')
            while True:
                line=self.rfile.readline(65536)
                if not line:return
                parts=line.decode().strip().split(' ',2)
                if len(parts)<2:return
                tag,command=parts[:2]
                command=command.upper()
                arguments=parts[2] if len(parts)>2 else ''
                if command=='CAPABILITY':send('* CAPABILITY IMAP4rev1 AUTH=PLAIN SASL-IR')
                elif command=='AUTHENTICATE' and len(arguments.split())<2:
                    send('+ ');self.rfile.readline(65536)
                elif command in ('LIST','LSUB'):send('* '+command+' (\\HasNoChildren) "/" "INBOX"')
                elif command=='STATUS':
                    name=shlex.split(arguments)[0]
                    send('* STATUS "'+name+'" (MESSAGES 0 RECENT 0 UIDNEXT 1 UIDVALIDITY 1 UNSEEN 0)')
                elif command in ('SELECT','EXAMINE'):
                    send('* 0 EXISTS');send('* 0 RECENT');send('* OK [UIDVALIDITY 1] fixture')
                elif command=='LOGOUT':
                    send('* BYE fixture');send(tag+' OK logout');return
                send(tag+' OK fixture')
    class Server(socketserver.ThreadingTCPServer):
        allow_reuse_address=True
        daemon_threads=True
    server=Server(('127.0.0.1',0),Handler)
    thread=threading.Thread(target=server.serve_forever,kwargs={'poll_interval':.05},daemon=True)
    thread.start()
    return server,certificate


def snapshot(root):
    result={}
    for mailbox in root.rglob('*.maildir'):
        rows=[]
        for state in ('cur','new'):
            for path in (mailbox/state).glob('*'):
                if path.is_file():
                    flags=path.name.partition(';2,')[2]
                    rows.append([state,flags,hashlib.sha256(path.read_bytes()).hexdigest()])
        result[mailbox.relative_to(root).as_posix()]=sorted(rows)
    return result


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('fixtures',type=Path)
    parser.add_argument('output',type=Path)
    args=parser.parse_args()
    root=args.output.resolve()
    root.mkdir(parents=True,exist_ok=True)
    results=[]
    for action,keys in (('move_to_inbox','xi'),('purge','xp'),('mark_unread_old','xr'),
                        ('gmail_register_unregister','xgoxgc')):
        outcomes={}
        for label in ('original','candidate'):
            source=(args.fixtures/label).resolve()
            destination=root/action/label
            shutil.copytree(source,destination)
            config=destination/'fixture.neomuttrc'
            config.write_text(config.read_text().replace(str(source),str(destination)))
            home=destination/'home'
            mailcap=home/'main.syncthing/monochrome/module/email/mailcap'
            mailcap.write_text(mailcap.read_text().replace(str(source),str(destination)))
            inbox=destination/'inbox.maildir'
            messages=[path for state in ('new','cur') for path in (inbox/state).glob('*') if path.is_file()]
            message=messages[0].read_bytes()
            folders=list(destination.rglob('*.maildir'))
            for folder in folders:
                for state in ('cur','new'):
                    for path in (folder/state).glob('*'):
                        if path.is_file():path.unlink()
            selected=next(path for path in folders if path.name=='vault.maildir') if action=='move_to_inbox' else inbox
            (selected/'new/fixture').write_bytes(message)
            server=None
            with config.open('a') as stream:
                if action=='gmail_register_unregister':
                    server,certificate=imap_fixture(destination)
                    stream.write('\nset folder="imaps://127.0.0.1:'+str(server.server_address[1])+'"\n')
                    stream.write('set ssl_ca_certificates_file="'+str(certificate)+'"\n')
                    stream.write('set imap_user="fixture" imap_pass="fixture" imap_authenticators="plain"\n')
                else:
                    stream.write('\nset folder="imaps://127.0.0.1:9"\n')
            env={**os.environ,'HOME':str(home),'TERM':'xterm-256color',
                 'PATH':str(destination/'tools')+os.pathsep+os.environ['PATH']}
            if label=='original':env['PYTHONPATH']=site.getusersitepackages()+os.pathsep+env.get('PYTHONPATH','')
            terminal=Terminal(config,selected,env)
            try:
                terminal.read(0.7)
                terminal.send(keys)
                terminal.read(0.5)
                terminal.send('q')
                terminal.read(0.5)
                if terminal.process.poll() is None:
                    terminal.send('\n')
                    terminal.read(0.4)
                exit_code=terminal.process.poll()
                outcomes[label]={'mailboxes':snapshot(destination),'exited':exit_code==0}
            finally:
                terminal.close(root/(action+'_'+label+'.terminal'))
                if server:
                    server.shutdown();server.server_close()
        results.append({'name':action,'passed':outcomes['original']==outcomes['candidate'] and
                        outcomes['original']['exited'],'outcomes':outcomes})
    print(json.dumps({'results':results},indent=2))
    return 0 if all(row['passed'] for row in results) else 1


if __name__=='__main__':
    raise SystemExit(main())

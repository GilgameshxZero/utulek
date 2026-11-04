#!/usr/bin/env python3
"""Exercise actual NeoMutt SMTP against an isolated loopback fixture server.

Optional Windows execution uses an attached SSH reverse forward. It creates only
neomutt-xp-network fixture files and a task-owned executable under the test home.
"""
import argparse
import base64
from email import policy
from email.parser import BytesParser
import json
import os
import shutil
import tarfile
from pathlib import Path
import socket
import ssl
import subprocess
import time
from mail_fixture import SmtpFixture, PASSWORD
from gpg_fixture import encrypt_fixture, decrypt_command

ROOT=Path(__file__).resolve().parents[1]
p=argparse.ArgumentParser()
p.add_argument('app',type=Path)
p.add_argument('--ssh-host')
p.add_argument('--cases', nargs='+')
p.add_argument('--output',type=Path,required=True)
a=p.parse_args()
output=a.output.resolve();output.mkdir(parents=True,exist_ok=True)
app=a.app.resolve()
ssh=['ssh','-F','/dev/null','-o','BatchMode=yes','-o','ConnectTimeout=10']
remote=None
if a.ssh_host:
    home=subprocess.check_output([*ssh,a.ssh_host,'echo %USERPROFILE%'],text=True).strip()
    remote=home.replace('\\','/')+'/neomutt-xp-network'
    subprocess.run([*ssh,a.ssh_host,f'if not exist "{remote}" mkdir "{remote}"'],check=True)
    subprocess.run(['scp','-F','/dev/null',str(app),a.ssh_host+':neomutt-xp-network/neomutt.exe'],check=True)
cert=output/'fixture.pem';key=output/'fixture-key.pem'
if not cert.exists():
    subprocess.run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-days','2',
                    '-keyout',str(key),'-out',str(cert),'-subj','/CN=localhost',
                    '-addext','subjectAltName=DNS:localhost'],
                   stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,check=True)
    key.chmod(0o600)
context=ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER);context.load_cert_chain(cert,key)
if remote:subprocess.run(['scp','-F','/dev/null',str(cert),a.ssh_host+':neomutt-xp-network/fixture.pem'],check=True)
# Genuine encrypted settings and OAuth tokens consumed by the actual helpers.
fixtures=output/'gpg_fixture'
encrypt_fixture(fixtures,'smtp-settings.gpg',b'set smtp_pass="fixture-pass"\n')
from datetime import datetime, timedelta
payload={'registration':'google','authflow':'authcode','email':'fixture@example.invalid',
         'client_id':'fixture-client','client_secret':'fixture-secret','refresh_token':'fixture-refresh',
         'access_token':'fixture-oauth-token','access_token_expiration':(datetime.now()+timedelta(days=1)).isoformat()}
encrypt_fixture(fixtures,'oauth-token.gpg',json.dumps(payload).encode())
for name in ('mutt_oauth2.py','command_args.py','private_storage.py'):
 shutil.copy2(ROOT/'python'/name,fixtures/name)
shutil.copy2(ROOT/'tests/protect_fixture.py',fixtures/'protect_fixture.py')
shutil.copy2(ROOT/'tests/record_fixture.py',fixtures/'record_fixture.py')
(fixtures/'calendar 日本語.ics').write_bytes(b'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Fixture//EN\r\nBEGIN:VEVENT\r\nUID:record-fixture@example.invalid\r\nDTSTAMP:20261103T000000Z\r\nDTSTART:20300501T100000Z\r\nDTEND:20300501T110000Z\r\nSUMMARY:Record fixture\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n')
(fixtures/'pixel.png').write_bytes(base64.b64decode('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+j2YQAAAAASUVORK5CYII='))
fixture_path=str(fixtures)
if remote:
 archive=output/'gpg_fixture.tar.gz'
 with tarfile.open(archive,'w:gz') as tar:
  tar.add(fixtures,arcname='gpg_fixture')
  resources=app.parent/'neomutt'
  if resources.is_dir():tar.add(resources,arcname='neomutt')
 subprocess.run(['scp','-F','/dev/null',str(archive),a.ssh_host+':neomutt-xp-network/gpg_fixture.tar.gz'],check=True)
 extract_code=f"import tarfile; tarfile.open('{remote}/gpg_fixture.tar.gz').extractall('{remote}',filter='data')"
 subprocess.run([*ssh,a.ssh_host,'python3 -X utf8 -c "'+extract_code+'"'],check=True)
 fixture_path=remote+'/gpg_fixture'
 subprocess.run([*ssh,a.ssh_host,f'python3 -X utf8 "{fixture_path}/protect_fixture.py" "{fixture_path}/oauth-token.gpg"'],check=True)
decryption=decrypt_command(fixture_path)
oauth_command='python3 -X utf8 "'+fixture_path+'/mutt_oauth2.py" "'+fixture_path+'/oauth-token.gpg" --decryption-pipe '+"'"+decryption+"'"
if remote:
 oauth_command='python3 -X utf8 "'+fixture_path+'/mutt_oauth2.py" "'+fixture_path+'/oauth-token.gpg" --decryption-pipe "'+decryption.replace('"','""')+'"'
def rc_quote(value):return value.replace('\\','\\\\').replace('"','\\"')
def mime_signature(message):
    return [(part.get_content_type(),part.get_filename(),part.get_payload(decode=True).replace(b'\r\n',b'\n') if part.get_content_maintype()=='text' else part.get_payload(decode=True)) for part in message.walk() if not part.is_multipart()]
run_id=str(time.time_ns())
results=json.loads((output/'results.json').read_text()) if a.cases and (output/'results.json').exists() else []
for name,method,tls,wrong,drop in [
 ('plain','plain',None,False,False),('login','login',None,False,False),
 ('default_login','',None,False,False),
 ('login_no_colon','login',None,False,False),('login_empty_challenge','login',None,False,False),('default_plain','',None,False,False),
 ('oauth','xoauth2',None,False,False),('starttls','plain','starttls',False,False),
 ('implicit_tls','plain','implicit',False,False),('rejected_password','plain',None,True,False),
 ('disconnect_during_data','plain',None,False,True),
 ('untrusted_tls','plain','starttls',False,False),
 ('hostname_mismatch','plain','starttls',False,False),
 ('encrypted_password','plain','starttls',False,False),
 ('encrypted_oauth','xoauth2','starttls',False,False),
 ('record_plain','plain',None,False,False),
 ('record_attachments','plain',None,False,False),
 ('record_rejected','plain',None,True,False),
 ('record_disconnect','plain',None,False,True),
 ('record_lost_ack','plain',None,False,False),
 ('record_rejected_data','plain',None,False,False),
 ('record_disconnect_after_ack','plain',None,False,False)]:
    if a.cases and name not in a.cases: continue
    results=[r for r in results if r['case']!=name]
    with SmtpFixture(context if tls else None,implicit_tls=tls=='implicit',drop_data=drop,drop_ack=name=='record_lost_ack',reject_data=name=='record_rejected_data',drop_after_ack=name=='record_disconnect_after_ack',auth_methods=[name.removeprefix('default_').upper()] if name.startswith('default_') else None,login_prompts=(b'Username',b'Password') if name=='login_no_colon' else (b'',b'') if name=='login_empty_challenge' else None) as server:
        local_port=server.server_address[1]
        port=48127 if remote else local_port
        forwarding=None
        try:
            if remote:
                forwarding=subprocess.Popen([*ssh,'-o','ExitOnForwardFailure=yes','-N','-R',
                     f'127.0.0.1:{port}:127.0.0.1:{local_port}',a.ssh_host],stdout=subprocess.PIPE,stderr=subprocess.PIPE)
                time.sleep(.35)
                if forwarding.poll() is not None:raise RuntimeError(forwarding.stderr.read().decode())
            scheme='smtps' if tls=='implicit' else 'smtp'
            trust=(remote+'/fixture.pem') if remote else str(cert)
            if name=='untrusted_tls':trust=(remote+'/not_trusted.pem') if remote else str(output/'not_trusted.pem')
            hostname='127.0.0.1' if name=='hostname_mismatch' else 'localhost'
            rc=output/(name+'.rc')
            rc.write_text(f'set folder="."\nset realname="Fixture Reader"\nset from="reader@example.invalid"\n'
                 f'set smtp_url="{scheme}://fixture@{hostname}:{port}"\n'
                 f'set smtp_pass="{"wrong" if wrong else PASSWORD}"\nset smtp_authenticators="{method}"\n'
                 f'set smtp_oauth_refresh_command="echo fixture-oauth-token"\n'
                 f'set ssl_force_tls={"yes" if tls else "no"}\nset ssl_starttls={"yes" if tls else "no"}\n'
                 f'set ssl_use_system_certs=no\nset certificate_file="{trust}"\n'
                 'set socket_timeout=5\nset ssl_verify_host=yes\nset ssl_verify_dates=yes\n'
                 'set hostname="fixture.example.invalid"\nset charset="utf-8"\nunset record\nset copy=no\n')
            record_path=None
            if name.startswith('record_'):
                record_path=(remote+'/record_fixture_'+run_id+'_'+name+'.maildir') if remote else str(output/('record_fixture_'+run_id+'_'+name+'.maildir'))
                rc.write_text(rc.read_text()+f'set mbox_type=Maildir maildir_field_delimiter=";" confirm_create=no confirm_append=no record="{record_path}" copy=yes\n')
            if name=='encrypted_password':
                text=rc.read_text().replace('set smtp_pass="fixture-pass"', 'source "'+rc_quote(decryption+' "'+fixture_path+'/smtp-settings.gpg"')+'|"')
                rc.write_text(text)
            if name=='encrypted_oauth':
                text=rc.read_text().replace('set smtp_oauth_refresh_command="echo fixture-oauth-token"',
                                           'set smtp_oauth_refresh_command="'+rc_quote(oauth_command)+'"')
                rc.write_text(text)
            attachment_names=['calendar 日本語.ics','pixel.png'] if name=='record_attachments' else []
            if remote:
                subprocess.run(['scp','-F','/dev/null',str(rc),a.ssh_host+':neomutt-xp-network/'+rc.name],check=True)
                attachments=(' -a '+ ' '.join('"'+fixture_path+'/'+filename+'"' for filename in attachment_names)+' --') if attachment_names else ''
                command=[*ssh,a.ssh_host,f'"{remote}/neomutt.exe" -n -F "{remote}/{rc.name}" -s "fixture {name}"{attachments} recipient@example.invalid']
                env=None
            else:
                command=[str(app),'-n','-F',str(rc),'-s','fixture '+name]+(['-a',*[str(fixtures/n) for n in attachment_names],'--'] if attachment_names else [])+['recipient@example.invalid']
                env=dict(os.environ,HOME=str(output),TMPDIR=str(output))
            sent=subprocess.run(command,input='fixture body Grüße 日本語\n.leading dot\n'.encode(),
                                capture_output=True,env=env,timeout=25)
            (output/(name+'.stdout')).write_bytes(sent.stdout)
            (output/(name+'.stderr')).write_bytes(sent.stderr)
            messages=[m for s in server.sessions for m in s['messages']]
            success=not wrong and not drop and name not in ('untrusted_tls','hostname_mismatch','record_lost_ack','record_rejected_data')
            passed=(sent.returncode==0 and len(messages)==1) if success else (sent.returncode!=0 and not messages)
            if name=='record_lost_ack':passed=sent.returncode!=0 and len(messages)==1 and len(server.sessions)==1 and b'delivery status is unknown' in sent.stdout+sent.stderr
            if name=='record_rejected_data':passed=passed and b'delivery status is unknown' not in sent.stdout+sent.stderr
            if name.startswith('record_'):passed=passed and len(server.sessions)==1
            if name.startswith('default_'):
                expected='AUTH '+name.removeprefix('default_').upper()
                passed=passed and all(c==expected for s in server.sessions for c in s['commands'] if c.startswith('AUTH '))
            if messages:
                data=messages[0]['data'];(output/(name+'.eml')).write_bytes(data)
                parsed=BytesParser(policy=policy.default).parsebytes(data)
                body=(parsed.get_body(preferencelist=('plain',)) if parsed.is_multipart() else parsed).get_content()
                if attachment_names:
                    attachments=list(parsed.iter_attachments())
                    passed=passed and [(part.get_filename(),part.get_content_type()) for part in attachments]==[('calendar 日本語.ics','text/calendar'),('pixel.png','image/png')]
                    passed=passed and all(part.get_payload(decode=True)==(fixtures/filename).read_bytes() for part,filename in zip(attachments,attachment_names))
                passed=passed and parsed['Subject']=='fixture '+name and 'Grüße 日本語' in body and '.leading dot' in body
            if tls and success:passed=passed and any(s['tls'] and s['authenticated'] for s in server.sessions)
            record_summary=None
            if record_path:
                if remote:
                    record_run=subprocess.run([*ssh,a.ssh_host,f'python3 -X utf8 "{fixture_path}/record_fixture.py" "{record_path}"'],capture_output=True,check=True,timeout=15)
                else:
                    record_run=subprocess.run(['python3',str(ROOT/'tests/record_fixture.py'),record_path],capture_output=True,check=True,timeout=15)
                records=json.loads(record_run.stdout)
                record_ok=len(records)==(1 if success else 0)
                if records:
                    saved=BytesParser(policy=policy.default).parsebytes(base64.b64decode(records[0]['data']))
                    submitted=BytesParser(policy=policy.default).parsebytes(messages[0]['data']) if messages else None
                    record_ok=record_ok and submitted is not None and saved['Message-ID']==submitted['Message-ID'] and mime_signature(saved)==mime_signature(submitted)
                    (output/(name+'.record.eml')).write_bytes(base64.b64decode(records[0]['data']))
                passed=passed and record_ok
                record_summary={'passed':bool(record_ok),'copies':len(records),'successful_submission':success}
            for session in server.sessions:
                session['messages']=[{'sender':m['sender'],'recipients':m['recipients'],'bytes':len(m['data'])} for m in session['messages']]
            results.append({'case':name,'passed':bool(passed),'exit':sent.returncode,'sessions':server.sessions,'record':record_summary})
        except Exception as error:results.append({'case':name,'passed':False,'error':str(error)})
        finally:
            if forwarding:
                forwarding.terminate()
                try:forwarding.communicate(timeout=5)
                except subprocess.TimeoutExpired:forwarding.kill();forwarding.communicate()
        (output/'results.json').write_text(json.dumps(results,indent=2)+'\n')
        print(json.dumps(results[-1]),flush=True)
raise SystemExit(0 if all(r['passed'] for r in results) else 1)

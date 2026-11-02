#!/usr/bin/env python3
"""Exercise real GPG fixture storage and mock provider requests; no accounts."""
import argparse
import base64
import json
import os
from pathlib import Path
import shlex
import shutil
import subprocess
import sys
import tempfile

ROOT=Path(__file__).resolve().parent
sys.path.insert(0,str(ROOT/'python'))
from private_storage import check_private, protect_private, windows_acl
from command_args import split_command

DRIVER='''import json,os,runpy,sys,urllib.parse
from unittest.mock import patch
sys.path.insert(0,os.environ['TEST_HELPERS'])
sys.argv=[os.environ['TEST_SCRIPT'],*json.loads(os.environ['TEST_ARGS'])]
calls=[]
class Response:
 def __init__(self,data):self.data=data
 def read(self):return json.dumps(self.data).encode()
def request(url,data=None,*args,**kwargs):
 params=urllib.parse.parse_qs(data.decode())
 calls.append(params)
 if os.environ.get('TEST_REJECT')=='1':return Response({'error':'invalid_grant','error_description':'Fixture rejection'})
 if url.endswith('/device/code'):
  return Response({'device_code':'fixture-device','message':'Fixture device authorization','interval':0})
 return Response({'access_token':'refreshed-fixture-access','refresh_token':'refreshed-fixture-refresh','expires_in':3600})
try:
 with patch('urllib.request.urlopen',side_effect=request),patch('time.sleep'):
  runpy.run_path(os.environ['TEST_SCRIPT'],run_name='__main__')
finally:
 from pathlib import Path
 Path(os.environ['TEST_CALLS']).write_text(json.dumps(calls))
'''


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--script',type=Path,default=ROOT/'python/mutt_oauth2.py')
    args=parser.parse_args()
    results=[]
    with tempfile.TemporaryDirectory(prefix='neomutt_oauth_') as temporary:
        root=Path(temporary)
        gpg_home=root/'gpg home'
        gpg_home.mkdir(mode=0o700)
        protect_private(gpg_home)
        gpg=shutil.which('gpg')
        if not gpg:
            raise RuntimeError('Existing GPG executable not found')
        home_argument=str(gpg_home)
        if os.name=='nt' and '/git/usr/bin/' in gpg.replace('\\','/').lower():
            # Git's GPG is an MSYS executable: its agent sockets require MSYS
            # paths, not drive-colon paths. Native GnuPG uses native paths.
            home_argument=subprocess.run(['cygpath','-u',str(gpg_home)],capture_output=True,text=True,check=True).stdout.strip()
        base=[gpg,'--homedir',home_argument,'--batch','--pinentry-mode','loopback',
              '--passphrase','synthetic-fixture-passphrase','--no-symkey-cache']
        decrypt=base+['--decrypt']
        encrypt=base+['--compress-algo','none','--symmetric']
        join=subprocess.list2cmdline if os.name=='nt' else shlex.join
        driver=root/'driver.py'
        driver.write_text(DRIVER)
        token_path=root/'token with spaces.gpg'
        valid={'registration':'google','authflow':'authcode','email':'person@example.test',
               'client_id':'fixture-client','client_secret':'fixture-secret',
               'access_token':'fixture-access','access_token_expiration':'2099-01-01T00:00:00',
               'refresh_token':'fixture-refresh'}
        def put(token):
            result=subprocess.run(encrypt,input=json.dumps(token).encode(),capture_output=True,check=True)
            token_path.write_bytes(result.stdout)
            protect_private(token_path)
        def get():
            return json.loads(subprocess.run(decrypt,input=token_path.read_bytes(),capture_output=True,check=True).stdout)
        def run(flags=(),reject=False):
            cli=[str(token_path),'--decryption-pipe',join(decrypt),'--encryption-pipe',join(encrypt),*flags]
            env={**os.environ,'TEST_HELPERS':str(args.script.resolve().parent),
                 'TEST_SCRIPT':str(args.script.resolve()),'TEST_ARGS':json.dumps(cli),
                 'TEST_CALLS':str(root/'calls.json'),'TEST_REJECT':'1' if reject else '0'}
            response=subprocess.run([sys.executable,'-S',str(driver)],input=b'fixture-code\n',capture_output=True,env=env,timeout=20)
            calls=json.loads((root/'calls.json').read_text())
            return response,calls
        def check(name,passed,detail=''):
            results.append({'name':name,'passed':bool(passed),'detail':detail})
        try:
            check('quoted_command_roundtrip',split_command(join(decrypt))==decrypt)
            for provider in ('google','microsoft'):
                token={**valid,'registration':provider}
                for protocol in ('imap','pop','smtp'):
                    put(token)
                    for output in ('token','sasl','msasl'):
                        response,calls=run(['--protocol',protocol,'--format',output])
                        if output=='token':
                            expected='fixture-access'
                        else:
                            if provider=='google':
                                host={'imap':'imap.gmail.com','pop':'pop.gmail.com','smtp':'smtp.gmail.com'}[protocol]
                                port={'imap':993,'pop':995,'smtp':587}[protocol]
                                raw=f'n,a=person@example.test,\1host={host}\1port={port}\1auth=Bearer fixture-access\1\1'
                                method='OAUTHBEARER'
                            else:
                                raw='user=person@example.test\1auth=Bearer fixture-access\1\1'
                                method='XOAUTH2'
                            expected=base64.b64encode(raw.encode()).decode()
                            if output=='msasl':expected=method+' '+expected
                        check(provider+'/'+protocol+'/'+output,response.returncode==0 and
                              response.stdout.decode().strip()==expected and not calls,
                              response.stderr.decode(errors='replace'))
            put({**valid,'access_token_expiration':'1990-01-01T00:00:00'})
            response,calls=run()
            refreshed=get()
            check('refresh_updates_encrypted_state',response.returncode==0 and response.stdout.strip()==b'refreshed-fixture-access'
                  and len(calls)==1 and calls[0]['grant_type']==['refresh_token'] and
                  refreshed['refresh_token']=='refreshed-fixture-refresh',response.stderr.decode(errors='replace'))
            put({**valid,'access_token_expiration':'1990-01-01T00:00:00'})
            before=token_path.read_bytes()
            response,calls=run(reject=True)
            check('provider_rejection_preserves_state',response.returncode!=0 and token_path.read_bytes()==before)
            for flow in ('authcode','devicecode'):
                token_path.unlink()
                response,calls=run(['--authorize','--provider','google','--authflow',flow,
                                   '--email','person@example.test','--client-id','fixture-client','--client-secret','fixture-secret'])
                token=get() if response.returncode==0 else {}
                check('authorize_'+flow,response.returncode==0 and token.get('access_token')=='refreshed-fixture-access'
                      and bool(calls),response.stderr.decode(errors='replace'))
            check_private(token_path)
            check('private_storage_accepts_protected_fixture',True)
            if os.name!='nt':
                token_path.chmod(0o644)
                try:
                    check_private(token_path)
                except ValueError:
                    check('private_storage_rejects_broad_permissions',True)
                else:
                    check('private_storage_rejects_broad_permissions',False)
                token_path.chmod(0o600)
            else:
                import base64 as b64
                encoded=b64.b64encode(str(token_path).encode()).decode()
                script="$p=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('"+encoded+"'));$a=Get-Acl -LiteralPath $p;$a.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new([Security.Principal.SecurityIdentifier]::new('S-1-1-0'),'Read','Allow'));Set-Acl -LiteralPath $p -AclObject $a"
                powershell=Path(os.environ['SystemRoot'])/'System32/WindowsPowerShell/v1.0/powershell.exe'
                subprocess.run([str(powershell),'-NoProfile','-NonInteractive','-EncodedCommand',b64.b64encode(script.encode('utf-16le')).decode()],check=True,capture_output=True)
                try:
                    check_private(token_path)
                except subprocess.CalledProcessError:
                    check('private_storage_rejects_everyone_read_acl',True)
                else:
                    check('private_storage_rejects_everyone_read_acl',False)
                protect_private(token_path)
        finally:
            conf=Path(gpg).with_name('gpgconf.exe' if os.name=='nt' else 'gpgconf')
            if conf.exists():
                subprocess.run([str(conf),'--homedir',home_argument,'--kill','gpg-agent'],capture_output=True,timeout=10)
    print(json.dumps({'results':results},indent=2,ensure_ascii=True))
    return 0 if all(row['passed'] for row in results) else 1


if __name__=='__main__':
    raise SystemExit(main())

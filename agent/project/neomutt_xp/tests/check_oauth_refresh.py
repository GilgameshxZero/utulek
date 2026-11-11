#!/usr/bin/env python3
"""Genuine encrypted token refresh/persistence with a mocked provider; no network."""
import argparse
from datetime import datetime,timedelta
import hashlib
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys

p=argparse.ArgumentParser();p.add_argument('helper',type=Path);p.add_argument('output',type=Path);a=p.parse_args()
root=a.output.resolve();root.mkdir(parents=True,exist_ok=True)
sys.path.insert(0,str(a.helper.resolve().parent))
from private_storage import protect_private,check_private
home=root/'gpg_home';home.mkdir(exist_ok=True)
if os.name!='nt':home.chmod(0o700)
password=root/'fixture_passphrase.txt';password.write_bytes(b'fixture-only-passphrase\n');protect_private(password)
def gp(p):
 s=str(p).replace('\\','/')
 return '/'+s[0].lower()+s[2:] if os.name=='nt' and len(s)>1 and s[1]==':' else s
base=['gpg','--homedir',gp(home),'--batch','--yes','--quiet','--no-symkey-cache','--pinentry-mode','loopback','--passphrase-file',gp(password)]
# Command strings use native argv quoting on Windows, POSIX quoting on Linux.
import shlex
join=subprocess.list2cmdline if os.name=='nt' else shlex.join
dec=join(base+['--decrypt']);enc=join(base+['--symmetric'])
runner=root/'mock_provider.py';runner.write_text("import io,json,runpy,sys,urllib.request\n"+"urllib.request.urlopen=lambda *a,**kw: io.BytesIO(json.dumps({'access_token':'new-fixture-access','refresh_token':'new-fixture-refresh','expires_in':3600}).encode())\n"+"helper=sys.argv.pop(1); sys.path.insert(0,str(__import__('pathlib').Path(helper).parent)); runpy.run_path(helper,run_name='__main__')\n")
results=[]
for case in ['unexpired','force_refresh','expired_refresh','encryption_failure','empty_encryption','replace_failure']:
 token={'registration':'google','authflow':'localhostauthcode','email':'fixture@example.invalid','client_id':'fixture-id','client_secret':'fixture-secret','access_token':'old-fixture-access','refresh_token':'old-fixture-refresh','access_token_expiration':(datetime.now()+timedelta(hours=1) if case in ('unexpired','force_refresh') else datetime.now()-timedelta(hours=1)).isoformat()}
 path=root/(case+'.gpg');run=subprocess.run(base+['--symmetric'],input=json.dumps(token).encode(),capture_output=True,check=True);path.write_bytes(run.stdout);protect_private(path);before=path.read_bytes()
 encryption=enc
 if case in ('encryption_failure','empty_encryption'):
  fail=root/'encrypt_failure.py';fail.write_text('import sys\nsys.exit('+('37' if case=='encryption_failure' else '0')+')\n');encryption=join([sys.executable,str(fail)])
 chosen_runner=runner
 if case=='replace_failure':
  chosen_runner=root/'mock_replace.py';chosen_runner.write_text(runner.read_text().replace("helper=sys.argv.pop(1);", "import os\ndef fail(*a,**kw): raise OSError('fixture replace failure')\nos.replace=fail\nhelper=sys.argv.pop(1);"))
 run=subprocess.run([sys.executable,'-X','utf8',str(chosen_runner),str(a.helper.resolve()),str(path),*(['--force-refresh'] if case=='force_refresh' else []),'--decryption-pipe',dec,'--encryption-pipe',encryption],capture_output=True)
 successful=case in ('unexpired','force_refresh','expired_refresh')
 decoded=json.loads(subprocess.run(base+['--decrypt',gp(path)],capture_output=True,check=True).stdout)
 expected_access='new-fixture-access' if case in ('expired_refresh','force_refresh') else 'old-fixture-access'
 passed=(run.returncode==0 if successful else run.returncode!=0) and decoded['access_token']==expected_access
 if case not in ('expired_refresh','force_refresh'):passed=passed and path.read_bytes()==before
 else:passed=passed and decoded['refresh_token']=='new-fixture-refresh' and run.stdout.strip()==b'new-fixture-access'
 check_private(path);passed=passed and not list(root.glob('.'+path.name+'.*'))
 results.append({'case':case,'passed':passed,'exit':run.returncode,'same_ciphertext':path.read_bytes()==before,'decryptable_with_original_password':True})
 (root/(case+'.stderr')).write_bytes(run.stderr)
(root/'results.json').write_text(json.dumps(results,indent=2)+'\n');print(json.dumps(results,indent=2));raise SystemExit(0 if all(r['passed'] for r in results) else 1)

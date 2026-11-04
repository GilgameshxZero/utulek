#!/usr/bin/env python3
"""Run the production OAuth helper with real GPG prompts and a fake provider."""
from pathlib import Path
from datetime import datetime,timedelta
import contextlib,io,json,os,runpy,subprocess,sys,urllib.request
helper=Path(sys.argv[1]);root=Path(sys.argv[2]);root.mkdir(parents=True,exist_ok=True)
sys.path.insert(0,str(helper.parent));from private_storage import protect_private
home=root/'gpg_home';home.mkdir(exist_ok=True)
if os.name!='nt':home.chmod(0o700)
def gp(path):
 s=str(path).replace('\\','/')
 return '/'+s[0].lower()+s[2:] if os.name=='nt' and len(s)>1 and s[1]==':' else s
base=['gpg','--homedir',gp(home),'--quiet','--no-symkey-cache','--pinentry-mode','loopback']
token=dict(registration='google',authflow='localhostauthcode',email='fixture@example.invalid',client_id='fixture-id',client_secret='fixture-secret',access_token='old-fixture-access',refresh_token='old-fixture-refresh',access_token_expiration=(datetime.now()-timedelta(hours=1)).isoformat())
cipher=root/'token.gpg'
run=subprocess.run(base+['--batch','--yes','--passphrase','fixture-only-passphrase','--symmetric'],input=json.dumps(token).encode(),capture_output=True,check=True);cipher.write_bytes(run.stdout);protect_private(cipher);before=cipher.read_bytes()
import shlex
join=subprocess.list2cmdline if os.name=='nt' else shlex.join
urllib.request.urlopen=lambda *a,**kw:io.BytesIO(json.dumps(dict(access_token='new-fixture-access',refresh_token='new-fixture-refresh',expires_in=3600)).encode())
sys.argv=[str(helper),str(cipher),'--decryption-pipe',join(base+['--decrypt']),'--encryption-pipe',join(base+['--compress-algo','none','--symmetric'])]
output=io.StringIO()
try:
 with contextlib.redirect_stdout(output):
  try:runpy.run_path(str(helper),run_name='__main__')
  except SystemExit as e:
   if e.code not in (None,0):raise
 if output.getvalue().strip()!='new-fixture-access':raise RuntimeError('Wrong fixture access token')
 decoded=json.loads(subprocess.run(base+['--batch','--passphrase','fixture-only-passphrase','--decrypt',gp(cipher)],capture_output=True,check=True).stdout)
 if decoded['refresh_token']!='new-fixture-refresh' or cipher.read_bytes()==before:raise RuntimeError('Refreshed ciphertext not persisted')
 print('set my_fixture_decryption_marker="console token refresh verified"')
finally:
 subprocess.run(['gpgconf','--homedir',gp(home),'--kill','gpg-agent'],stdin=subprocess.DEVNULL,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,check=True)
 (root/'cipher_status.json').write_text(json.dumps(dict(ciphertext_changed=cipher.read_bytes()!=before)))

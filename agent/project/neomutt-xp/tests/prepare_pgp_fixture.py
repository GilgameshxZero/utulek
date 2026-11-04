#!/usr/bin/env python3
"""Create an isolated disposable signed/encrypted MIME fixture; never use real keys."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess

p=argparse.ArgumentParser()
p.add_argument('root',type=Path)
p.add_argument('--gpg',default='gpg')
p.add_argument('--cleanup',action='store_true')
a=p.parse_args()
root=a.root.resolve();home=root/'gpg_home'
gpg=Path(a.gpg)
gpgconf=str(gpg.with_name('gpgconf.exe')) if gpg.suffix=='.exe' else 'gpgconf'
if a.cleanup:
 subprocess.run([gpgconf,'--homedir',str(home),'--kill','all'],check=True)
 raise SystemExit(0)
root.mkdir(parents=True,exist_ok=True)
home.mkdir(mode=0o700,exist_ok=True)
if (home/'private-keys-v1.d').exists():raise SystemExit('Refusing to reuse an existing keyring')
env=dict(os.environ,GNUPGHOME=str(home));env.pop('GPG_AGENT_INFO',None)
base=[a.gpg,'--homedir',str(home),'--batch','--yes','--pinentry-mode','loopback','--passphrase','']
def run(args,data=None):
 r=subprocess.run(base+args,input=data,capture_output=True,env=env,timeout=45)
 if r.returncode:raise RuntimeError(r.stderr.decode(errors='replace'))
 return r.stdout
run(['--quick-generate-key','Fixture Reader <reader@example.invalid>','ed25519','sign','1d'])
keys=run(['--with-colons','--list-secret-keys']).decode()
fingerprint=next(line.split(':')[9] for line in keys.splitlines() if line.startswith('fpr:'))
run(['--quick-add-key',fingerprint,'cv25519','encr','1d'])
plaintext='Content-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\nGPGME encrypted fixture secret Grüße 日本語\r\n'.encode()
cipher=run(['--trust-model','always','--local-user',fingerprint,'--recipient',fingerprint,'--sign','--encrypt'],plaintext)
(root/'signed_encrypted.gpg').write_bytes(cipher)
(root/'settings.json').write_text(json.dumps({'fingerprint':fingerprint,'gnupg_home':str(home),'cipher':str(root/'signed_encrypted.gpg'),'cipher_sha256':hashlib.sha256(cipher).hexdigest()},indent=2)+'\n')
print(json.dumps({'prepared':True,'cipher_bytes':len(cipher),'isolated_home':str(home)}))

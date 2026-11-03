#!/usr/bin/env python3
"""Parse the installed configuration with synthetic HOME and password source.

The original file is read as data. Its one credential source is replaced before
NeoMutt sees it. Only synthetic encrypted credentials are decrypted; no production key, mailbox connection or sending is used.
"""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import tarfile
ROOT=Path(__file__).resolve().parents[1]
p=argparse.ArgumentParser()
p.add_argument('app',type=Path)
p.add_argument('--config',type=Path)
p.add_argument('--synthetic',action='store_true')
p.add_argument('--ssh-host')
p.add_argument('--wrong-passphrase',action='store_true')
p.add_argument('--output',type=Path,required=True)
a=p.parse_args()
out=a.output.resolve();out.mkdir(parents=True,exist_ok=True)
home=out/'home'
module=home/'main.syncthing/monochrome/module/email'
module.mkdir(parents=True,exist_ok=True)
(home/'main.syncthing/snowfall').mkdir(parents=True,exist_ok=True)
(home/'main.syncthing/monochrome/local.syncthing/module/email/vault.maildir').mkdir(parents=True,exist_ok=True)
if a.synthetic:
 source=(ROOT/'tests/fixture_config.rc').read_text()
else:
 if not a.config:p.error('--config or --synthetic is required')
 source=a.config.read_text()
 # Create only fake mailboxes under the disposable HOME for literal configured paths.
 for match in re.findall(r'~/([^\"\s]*\.maildir/?)',source):
  for sub in ('cur','new','tmp'):(home/match/sub).mkdir(parents=True,exist_ok=True)
for relative in ('main.syncthing/monochrome/module/email/postponed.maildir',
                 'main.syncthing/monochrome/archive.syncthing/module/email/archive.maildir',
                 'main.syncthing/monochrome/local.syncthing/module/email/vault.maildir'):
 for sub in ('cur','new','tmp'):(home/relative/sub).mkdir(parents=True,exist_ok=True)
credential_lines=[line for line in source.splitlines() if line.lstrip().startswith('source ')]
if len(credential_lines)!=1 or ('gpg --decrypt' not in credential_lines[0] and not a.synthetic):
 raise SystemExit('Refusing unexpected configuration source commands')
password_names=sorted(set(re.findall(r'\$([a-z_]*smtp_pass[a-z_]*)',source)))
plaintext='\n'.join('set '+name+'="fixture-password"' for name in password_names)
plaintext+='\nset my_fixture_decryption_marker="decrypted Grüße 日本語"\n'
gpg_home=module/'fixture_gpg';gpg_home.mkdir(exist_ok=True);gpg_home.chmod(0o700)
passphrase=module/'fixture_passphrase.txt';passphrase.write_text('fixture-only-passphrase\n');passphrase.chmod(0o600)
ciphertext=module/'neomuttrc.password.gpg'
encrypt=['gpg','--homedir',str(gpg_home),'--batch','--yes','--quiet','--no-symkey-cache',
         '--pinentry-mode','loopback','--passphrase-file',str(passphrase),
         '--compress-algo','none','--symmetric','--output',str(ciphertext)]
subprocess.run(encrypt,input=plaintext.encode(),capture_output=True,check=True)
ciphertext.chmod(0o600)
if a.wrong_passphrase:passphrase.write_text('incorrect-fixture-passphrase\n')
gpg_command='gpg --homedir "${my_active_dir}fixture_gpg" --batch --quiet --no-symkey-cache --pinentry-mode loopback --passphrase-file "${my_active_dir}fixture_passphrase.txt" --status-fd 2 --decrypt "${my_active_dir}neomuttrc.password.gpg"'
replacement='source "'+gpg_command.replace('"','\\"')+'|"'
source=source.replace(credential_lines[0],replacement)
if (module/'fixture-passwords.py').exists():(module/'fixture-passwords.py').unlink()
config=home/'fixture.rc';config.write_text(source)
app=a.app.resolve()
ssh=['ssh','-F','/dev/null','-o','BatchMode=yes','-o','ConnectTimeout=10']
if a.ssh_host:
 remote_home=subprocess.check_output([*ssh,a.ssh_host,'echo %USERPROFILE%'],text=True).strip().replace('\\','/')
 remote=remote_home+'/neomutt-xp-config'
 subprocess.run([*ssh,a.ssh_host,f'if not exist "{remote}" mkdir "{remote}"'],check=True)
 archive=out/'fixture.tar.gz'
 with tarfile.open(archive,'w:gz') as tar:tar.add(home,arcname='home')
 subprocess.run(['scp','-F','/dev/null',str(archive),a.ssh_host+':neomutt-xp-config/fixture.tar.gz'],check=True)
 subprocess.run(['scp','-F','/dev/null',str(app),a.ssh_host+':neomutt-xp-test.exe'],check=True)
 subprocess.run([*ssh,a.ssh_host,f'tar -xzf "{remote}/fixture.tar.gz" -C "{remote}"'],check=True)
 command=[*ssh,a.ssh_host,f'set "HOME={remote}/home"&& set "TMPDIR={remote}/home"&& set "EDITOR=fixture-editor"&& neomutt-xp-test.exe -n -F "{remote}/home/fixture.rc" -D']
 env=None
else:
 command=[str(app),'-n','-F',str(config),'-D']
 env=dict(os.environ,HOME=str(home),TMPDIR=str(home),EDITOR="fixture-editor")
run=subprocess.run(command,capture_output=True,env=env,timeout=30)
(out/'dump.txt').write_bytes(run.stdout);(out/'stderr.txt').write_bytes(run.stderr)
settings={}
for line in run.stdout.decode('utf-8',errors='replace').splitlines():
 match=re.match(r'(?:set )?([a-zA-Z_][a-zA-Z_0-9]*)\s*=\s*(.*)',line)
 if match:settings[match[1]]=match[2]
(out/'settings.json').write_text(json.dumps(settings,indent=2)+'\n')
decrypted=b'[GNUPG:] DECRYPTION_OKAY' in run.stderr
marker='decrypted Grüße 日本語' in settings.get('my_fixture_decryption_marker','')
parser_errors=[line for line in run.stderr.decode(errors='replace').splitlines() if line and not line.startswith(('[GNUPG:]','gpg:'))]
expected=(not decrypted and not marker) if a.wrong_passphrase else (decrypted and marker and not parser_errors and run.returncode==0)
result={'passed':expected,'exit':run.returncode,'settings':len(settings),
        'credential_source_replaced':True,'synthetic_password_variables':password_names,
        'gpg_decryption_ok':decrypted,'decrypted_marker':marker,'intentional_wrong_passphrase':a.wrong_passphrase,
        'parser_errors':parser_errors}
(out/'results.json').write_text(json.dumps(result,indent=2)+'\n')
print(json.dumps(result))
raise SystemExit(0 if result['passed'] else 1)

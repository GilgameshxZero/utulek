#!/usr/bin/env python3
"""Check NeoMutt's sendmail process path with a capture-only executable."""
import argparse
from email import policy
from email.parser import BytesParser
import json
import os
from pathlib import Path
import subprocess
ROOT=Path(__file__).resolve().parents[1]
p=argparse.ArgumentParser();p.add_argument('app',type=Path);p.add_argument('helper',type=Path)
p.add_argument('--ssh-host');p.add_argument('--output',type=Path,required=True)
a=p.parse_args();output=a.output.resolve();output.mkdir(parents=True,exist_ok=True)
ssh=['ssh','-F','/dev/null','-o','BatchMode=yes'];remote=None
if a.ssh_host:
 home=subprocess.check_output([*ssh,a.ssh_host,'echo %USERPROFILE%'],text=True).strip().replace('\\','/')
 remote=home+'/neomutt-xp-sendmail-fixture'
 subprocess.run([*ssh,a.ssh_host,f'if not exist "{remote}" mkdir "{remote}"'],check=True)
 for source,name in [(a.app,'neomutt-xp-test.exe'),(a.helper,'neomutt-xp-sendmail.exe')]:
  subprocess.run(['scp','-F','/dev/null',str(source.resolve()),a.ssh_host+':'+name],check=True)
 helper=home+'/neomutt-xp-sendmail.exe'
else:helper=str(a.helper.resolve())
results=[]
for name,code,delay,wait in [('success',0,0,0),('exit37',37,0,0),('background',0,2000,1)]:
 capture=(remote if remote else str(output))+'/'+name+' Grüße 日本語.eml'
 rc=output/(name+'.rc')
 rc.write_text(f'set folder="."\nset realname="Fixture Reader"\nset from="reader@example.invalid"\n'
     f'set sendmail="{helper} -oi"\nset sendmail_wait={wait}\n'
     f'setenv NEOMUTT_XP_CAPTURE="{capture}"\nsetenv NEOMUTT_XP_SENDMAIL_EXIT="{code}"\n'
     f'setenv NEOMUTT_XP_SENDMAIL_DELAY_MS="{delay}"\n'
     'set charset="utf-8"\nset hostname="fixture.example.invalid"\nunset record\nset copy=no\n')
 if remote:
  subprocess.run(['scp','-F','/dev/null',str(rc),a.ssh_host+':neomutt-xp-sendmail-fixture/'+rc.name],check=True)
  command=[*ssh,a.ssh_host,f'neomutt-xp-test.exe -n -F "{remote}/{rc.name}" -s "fixture {name}" recipient@example.invalid']
  env=None
 else:
  command=[str(a.app.resolve()),'-n','-F',str(rc),'-s','fixture '+name,'recipient@example.invalid']
  env=dict(os.environ,HOME=str(output),TMPDIR=str(output))
 sent=subprocess.run(command,input='capture fixture Grüße 日本語\n'.encode(),capture_output=True,env=env,timeout=15)
 (output/(name+'.stdout')).write_bytes(sent.stdout);(output/(name+'.stderr')).write_bytes(sent.stderr)
 local=Path(capture) if not remote else output/(name+'.eml')
 if remote:
  for suffix in ('','.args'):
   subprocess.run(['scp','-F','/dev/null',a.ssh_host+':'+capture+suffix,str(local)+suffix],check=True)
 parsed=BytesParser(policy=policy.default).parsebytes(local.read_bytes())
 expected=sent.returncode!=0 if code else sent.returncode==0
 args=Path(str(local)+'.args').read_text().splitlines()
 result={'case':name,'passed':expected and 'Grüße 日本語' in parsed.get_content() and '-oi' in args and 'recipient@example.invalid' in args,'exit':sent.returncode,'arguments':args}
 results.append(result);print(json.dumps(result),flush=True)
 (output/'results.json').write_text(json.dumps(results,indent=2)+'\n')
raise SystemExit(0 if all(r['passed'] for r in results) else 1)

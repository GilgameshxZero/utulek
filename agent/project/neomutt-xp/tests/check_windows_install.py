#!/usr/bin/env python3
"""Check the installed Windows launcher against synthetic configuration only."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile

p=argparse.ArgumentParser()
p.add_argument('--write-config-loader',action='store_true')
p.add_argument('--output',type=Path,required=True)
a=p.parse_args()
root=Path.home()/'main.syncthing/monochrome'
app=root/'app/neomutt.exe';launcher=root/'bin/neomutt.bat'
loader=Path.home()/'.config/neomutt/neomuttrc'
source='source ~/main.syncthing/monochrome/module/email/neomuttrc\n'
if a.write_config_loader:
 loader.parent.mkdir(parents=True,exist_ok=True)
 try:
  with loader.open('x',encoding='utf-8',newline='\n') as out:out.write(source)
 except FileExistsError:
  if loader.read_text(encoding='utf-8')!=source:raise RuntimeError('Existing config left untouched; unexpected contents')
assert loader.read_text(encoding='utf-8')==source
results=[]
with tempfile.TemporaryDirectory(prefix='neomutt install space 日本語 ') as scratch:
 rc=Path(scratch)/'fixture space.rc'
 rc.write_text('set my_install_marker="Grüße 日本語 with spaces"\n',encoding='utf-8')
 cases=[('version','-v',0,'NeoMutt 20260616'),('quoted_config_arguments','-n -F "'+str(rc)+'" -Q my_install_marker',0,'Grüße 日本語 with spaces'),('failure_exit_status','-n -F \"'+str(Path(scratch)/'missing.rc')+'\" -D',1,'')]
 for name,args,code,token in cases:
  command='cmd.exe /d /s /c ""'+str(launcher)+'" '+args+'"'
  run=subprocess.run(command,cwd=scratch,capture_output=True,timeout=20)
  output=(run.stdout+run.stderr).decode('utf-8',errors='replace')
  results.append({'name':name,'passed':run.returncode==code and token in output,'exit':run.returncode,'output':output})
report={'passed':all(r['passed'] for r in results),'app':str(app),'launcher':str(launcher),'sha256':{str(f.relative_to(root)):hashlib.sha256(f.read_bytes()).hexdigest() for f in (app,launcher)},'default_config_loader':str(loader),'results':results}
a.output.write_text(json.dumps(report,indent=2)+'\n',encoding='utf-8')
print(json.dumps({k:v for k,v in report.items() if k!='results'},indent=2))
print(json.dumps([{k:v for k,v in r.items() if k!='output'} for r in results],indent=2))
raise SystemExit(0 if report['passed'] else 1)

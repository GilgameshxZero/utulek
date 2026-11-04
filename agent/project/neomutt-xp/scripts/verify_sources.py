#!/usr/bin/env python3
"""Reconstruct pinned upstream sources and verify all archive files against work."""
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import tarfile
ROOT=Path(__file__).resolve().parents[1]
manifest=json.loads((ROOT/'dependencies.json').read_text())
verification=ROOT/'.obj/source-verification';verification.mkdir(exist_ok=True)
results=[]
for name in ('neomutt','ncurses','lmdb','libgpg-error','libassuan','gpgme','sqlite'):
 item=manifest[name];archive=ROOT/'.obj/downloads'/item['archive']
 if hashlib.sha256(archive.read_bytes()).hexdigest()!=item['sha256']:raise SystemExit('Archive hash mismatch: '+name)
 target=verification/name
 if target.exists():shutil.rmtree(target)
 target.mkdir()
 originals=[]
 with tarfile.open(archive) as tar:
  for member in tar.getmembers():
   member.name='/'.join(member.name.split('/')[1:])
   if not member.name:continue
   if member.isfile():originals.append(member.name)
   tar.extract(member,target,filter='data')
 patches=sorted((ROOT/'patches').glob(name+'-*.patch'))
 for patch in patches:subprocess.run(['patch','--batch','-p1','-i',str(patch)],cwd=target,check=True,stdout=subprocess.DEVNULL)
 work=ROOT/('.obj/upstream' if name=='neomutt' else '.obj/deps/'+name+'-'+item['version'])
 differences=[file for file in originals if (work/file).read_bytes()!=(target/file).read_bytes()]
 results.append({'source':name,'files':len(originals),'patches':[p.name for p in patches],'passed':not differences,'differences':differences})
(verification/'results.json').write_text(json.dumps(results,indent=2)+'\n')
print(json.dumps(results,indent=2))
raise SystemExit(0 if all(r['passed'] for r in results) else 1)

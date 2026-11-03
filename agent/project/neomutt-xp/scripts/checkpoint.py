#!/usr/bin/env python3
"""Save a reviewable source/binary/evidence checkpoint without Git commits."""
import argparse
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import shutil
import subprocess
import tarfile
ROOT=Path(__file__).resolve().parents[1]
p=argparse.ArgumentParser()
p.add_argument('name')
p.add_argument('--note',required=True)
a=p.parse_args()
if not a.name or any(c not in 'abcdefghijklmnopqrstuvwxyz0123456789_-' for c in a.name):
 p.error('name must contain only lowercase letters, digits, underscores and hyphens')
subprocess.run(['python3',ROOT/'scripts/export_patches.py'],check=True)
destination=ROOT/'.obj/checkpoints'/a.name
destination.mkdir(parents=True,exist_ok=False)
files=[f for f in ROOT.rglob('*') if f.is_file() and not any(part in ('.obj','.bin','__pycache__') for part in f.relative_to(ROOT).parts)]
with tarfile.open(destination/'source.tar.gz','w:gz') as archive:
 for f in sorted(files):archive.add(f,arcname='neomutt-xp/'+str(f.relative_to(ROOT)))
for directory in ('.bin','.obj/verification'):
 source=ROOT/directory
 if source.exists():shutil.copytree(source,destination/('binaries' if directory=='.bin' else 'verification'))
hashes={str(f.relative_to(destination)):hashlib.sha256(f.read_bytes()).hexdigest() for f in destination.rglob('*') if f.is_file()}
metadata={'name':a.name,'created_utc':datetime.now(timezone.utc).isoformat(),'note':a.note,'sha256':hashes}
(destination/'checkpoint.json').write_text(json.dumps(metadata,indent=2)+'\n')
print(destination)

#!/usr/bin/env python3
"""Collect small synthetic UI artifacts, excluding database/cache contents."""
import os
from pathlib import Path
import sys
import zipfile
root=Path(sys.argv[1]).resolve()
if root.name!='neomutt-xp-fixture':raise SystemExit('Expected the isolated fixture root')
with zipfile.ZipFile(sys.argv[2],'w',compression=zipfile.ZIP_DEFLATED) as archive:
 for directory,dirs,files in os.walk(root):
  dirs[:]=[d for d in dirs if d not in ('cache','gpg_home','fixture_gpg')]
  for name in files:
   path=Path(directory)/name
   if path.stat().st_size>10*1024*1024:continue
   archive.write(path,path.relative_to(root))
print('evidence saved')

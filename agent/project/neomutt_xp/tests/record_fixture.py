"""Inspect only a test-owned Maildir for SMTP/record consistency checks."""
import argparse,base64,json
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('maildir',type=Path);a=p.parse_args()
# Prevent accidental use as a general mailbox collector.
if not a.maildir.name.startswith('record_fixture_'):raise SystemExit('Expected owned record fixture')
files=[]
for sub in ['cur','new']:
 directory=a.maildir/sub
 if directory.exists():
  for path in directory.iterdir():
   if path.is_file():files.append({'name':sub+'/'+path.name,'data':base64.b64encode(path.read_bytes()).decode()})
print(json.dumps(files))

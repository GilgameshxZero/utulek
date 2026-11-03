#!/usr/bin/env python3
"""Stage native GPGME engines/resources alongside the native Windows binary."""
import hashlib,json,shutil,subprocess,urllib.request
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
item=json.loads((ROOT/'dependencies.json').read_text())['gnupg_windows']
archive=ROOT/'.obj/downloads'/item['archive'];archive.parent.mkdir(parents=True,exist_ok=True)
if not archive.exists():
 with urllib.request.urlopen(item['url'],timeout=60) as response:archive.write_bytes(response.read())
if hashlib.sha256(archive.read_bytes()).hexdigest()!=item['sha256']:raise SystemExit('GnuPG installer hash mismatch')
source=ROOT/'.obj/gnupg-windows'
if not (source/'bin/gpg.exe').exists():
 source.mkdir(exist_ok=True)
 with (source/'extract.log').open('w') as out:subprocess.run(['7z','x','-y','-o'+str(source),str(archive)],stdout=out,stderr=subprocess.STDOUT,check=True)
destination=ROOT/'.bin/windows/neomutt';engines=destination/'gnupg/bin';engines.mkdir(parents=True,exist_ok=True)
for file in (source/'bin').iterdir():
 if file.suffix.lower() in ('.exe','.dll'):shutil.copy2(file,engines/file.name)
shutil.copytree(source/'share/gnupg',destination/'gnupg/share/gnupg',dirs_exist_ok=True)
shutil.copy2(source/'README.txt',destination/'gnupg/README.txt')
shutil.copy2(ROOT/'runtime/neomutt-system.rc',destination/'system.rc')
shutil.copy2(ROOT/'runtime/mime.types',destination/'mime.types')
manual=Path('/usr/share/doc/neomutt/manual.txt.gz')
if manual.exists():shutil.copy2(manual,destination/'manual.txt.gz')
print(destination)

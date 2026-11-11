#!/usr/bin/env python3
"""Use real local paths/settings without decrypting production credentials.

Only for the read-only browser harness: source commands are replaced by fake
password settings; outgoing transport must never be invoked by that harness.
"""
from pathlib import Path
import re,sys
module,root,system=map(Path,sys.argv[1:]);root.mkdir(parents=True,exist_ok=True)
source=(module/'neomuttrc').read_text(encoding='utf-8')
lines=[l for l in source.splitlines() if l.lstrip().startswith('source ')]
if len(lines)!=1 or not re.search(r'gpg .*--decrypt',lines[0]):raise SystemExit('Unexpected source command')
replacement='\n'.join('set '+n+'="fixture-unused-password"' for n in sorted(set(re.findall(r'\$([a-z_]*smtp_pass[a-z_]*)',source))))
source=source.replace(lines[0],replacement)
source='source "'+system.as_posix()+'"\n'+source+'\nset crypt_use_gpgme=no header_cache="'+(root/'cache').as_posix()+'/"\n'
(root/'readonly.rc').write_text(source,encoding='utf-8');print('Read-only config prepared; no production credentials read')

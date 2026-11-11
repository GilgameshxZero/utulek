#!/usr/bin/env python3
"""Compare complete email query stdout/exit status with upstream, unnormalized."""
import json,os,subprocess,sys
from pathlib import Path
native,upstream,root=map(Path,sys.argv[1:]);root=root.resolve();root.mkdir(parents=True,exist_ok=True);books=root/'books';books.mkdir(exist_ok=True)
for uid,name,address in [('alice','Alice Zoë','alice@example.invalid'),('bob','Bob Zoë','bob@example.invalid'),('empty','NoEmail','')]:
 data='BEGIN:VCARD\nVERSION:3.0\nUID:'+uid+'\nFN:'+name+'\n'+('EMAIL;TYPE=work:'+address+'\n' if address else '')+'END:VCARD\n'
 (books/(uid+'.vcf')).write_text(data,encoding='utf-8')
config=root/'khard.conf';config.write_text('[addressbooks]\n[[fixture]]\npath = '+books.as_posix()+'\n')
results=[];env=dict(os.environ,HOME=str(root),USERPROFILE=str(root),LANG='C.UTF-8')
for terms in [[],['ALICE'],['example.invalid'],['missing'],['NoEmail'],['Zoë'],['name:alice'],['alice','Zoë']]:
 for remove in [False,True]:
  args=['-c',str(config),'email','-d','formatted_name','--parsable',*(['--remove-first-line'] if remove else []),'--',*terms]
  a=subprocess.run([str(native.resolve()),*args],capture_output=True,env=env);b=subprocess.run([str(upstream.resolve()),*args],capture_output=True,env=env)
  passed=a.returncode==b.returncode and a.stdout==b.stdout
  results.append(dict(terms=terms,remove_first_line=remove,passed=passed,native_exit=a.returncode,upstream_exit=b.returncode,**({} if passed else dict(native=a.stdout.decode(errors='replace'),upstream=b.stdout.decode(errors='replace')))))
(root/'results.json').write_text(json.dumps(results,indent=2));print(json.dumps(results,ensure_ascii=True,indent=2));sys.exit(not all(r['passed'] for r in results))

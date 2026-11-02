#!/usr/bin/env python3
"""Compare saved edit/new semantics with current upstream on disposable books."""
import argparse
import json
import os
from pathlib import Path
import subprocess
import tempfile
from audit_current import FIXTURES, normalize_yaml
from compare_operations import invoke

CASES={
 'replacement omissions':('edit','First name: Alice\nLast name: Example\nNote: Replacement\n',False),
 'blank clears':('edit','First name: Alice\nLast name: Example\nPhone:\nEmail:\nPrivate:\n',False),
 'full round trip':('edit',None,False),
 'tagged private round trip':('edit',None,False),
 'tagged private update':('edit','First name: Alice\nLast name: Example\nPrivate:\n SECRET:\n  account: private update\n',False),
 'tagged structured values':('edit','First name: Alice\nLast name: Example\nRole:\n - team: Maintainer\nNote:\n - source: tagged note\nOrganisation:\n - division: [Company, Research]\nCategories:\n - [One, Two]\n - [Three]\nPrivate:\n SECRET: private update\nImpp:\n xmpp, pref=2: handle@example.invalid\n',False),
 'dates v4':('edit','First name: Alice\nLast name: Example\nBirthday: --04-03\nAnniversary: text= Spring day\n',False),
 'invalid date':('edit','First name: Alice\nLast name: Example\nBirthday: 2000-02-30\n',True),
 'duplicate YAML':('edit','First name: Alice\nLast name: Example\nNote: One\nNote: Two\n',True),
 'new organisation v3':('new','Organisation:\n - [Acme, Research]\n',False),
 'new reduced leap day':('new','First name: Leap\nLast name: Person\nBirthday: --02-29\n',False),
 'new name v4':('new','Prefix: Dr.\nFirst name: New\nLast name: Person\nSuffix: Jr.\nImpp:\n xmpp: person@example.invalid\nBirthday: 2000-02-29\n',False),
}

def command(exe,args):
 p=subprocess.run([exe,*args],capture_output=True,text=True,timeout=10)
 if p.returncode:raise RuntimeError(p.stderr+p.stdout)
 return p.stdout

def snapshot(exe,config,book):
 cards=[]
 for path in book.glob('*.vcf'):
  text=path.read_text();uid=next(line[4:] for line in text.splitlines() if line.startswith('UID:'))
  model=normalize_yaml(command(exe,['-c',str(config),'show','--format','yaml','uid:'+uid]))
  # Prefixing VALUE=text exports with text= is needed for lossless re-import.
  for key in ['Anniversary','Birthday']:
   if isinstance(model.get(key),str) and model[key].startswith('text= '):model[key]=model[key][6:]
  if isinstance(model.get('Private'),dict):
   for key,value in model['Private'].items():
    if isinstance(value,list) and len(value)==1 and isinstance(value[0],dict):model['Private'][key]=value[0]
  cards.append({'uid':uid if uid=='alpha-one' else '<generated>','model':model,'unknown_preserved':'X-UNKNOWN:preserve' in text})
 return sorted(cards,key=lambda c:c['uid'])

def run_case(exe,label,case):
 action,data,invalid=case
 with tempfile.TemporaryDirectory(prefix='khard edit audit ') as temp:
  root=Path(temp);book=root/'contacts';book.mkdir();config=root/'khard.conf'
  config.write_text(f"[addressbooks]\n[[test]]\npath = '{book}'\n[contact table]\nlocalize_dates = no\n[vcard]\nprivate_objects = SECRET,\npreferred_version = 3.0\nsearch_in_source_files = no\n")
  if action=='edit':
   fixture=FIXTURES['alice'][1]
   if label=='full round trip':fixture=fixture.replace('itemprivate1.X-SECRET:private value\nitemprivate1.X-ABLabel:account\n','X-SECRET:private value\n')
   (book/'original.vcf').write_text('BEGIN:VCARD\n'+fixture+'END:VCARD\n')
  before={p.name:p.read_bytes().hex() for p in book.glob('*.vcf')}
  if data is None:data=command(exe,['-c',str(config),'show','--format','yaml','uid:alpha-one'])
  inputfile=root/'contact input.yaml';inputfile.write_text(data)
  args=['-c',str(config),action,'-i',str(inputfile)]
  if action=='edit':args.append('uid:alpha-one')
  if label in ['new name v4','new reduced leap day']:args+=['--vcard-version','4.0']
  result=invoke(exe,args,'y\n');result['cards']=snapshot(exe,config,book)
  result['unchanged']={p.name:p.read_bytes().hex() for p in book.glob('*.vcf')}==before
  return result

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--reference',default='.obj/upstream-current/bin/khard');p.add_argument('--candidate',default='.bin/khard');p.add_argument('--report',default='.obj/current_edit_audit.json');args=p.parse_args();os.environ['LC_ALL']='C';cases=[]
 for label,case in CASES.items():
  reference=run_case(args.reference,label,case);candidate=run_case(args.candidate,label,case)
  expected_invalid=case[2]
  if expected_invalid:passed=reference['exit_code']!=0 and candidate['exit_code']!=0 and reference['unchanged'] and candidate['unchanged']
  else:passed=reference['exit_code']==candidate['exit_code']==0 and reference['cards']==candidate['cards']
  if label=='full round trip':passed=passed and reference['unchanged'] and candidate['unchanged']
  exception=None
  if label=='new reduced leap day':
   exception='preserve valid vCard reduced leap day instead of rejecting it with a non-leap placeholder year'
   passed=reference['exit_code']!=0 and reference['unchanged'] and candidate['exit_code']==0 and candidate['cards'][0]['model'].get('Birthday')=='--02-29'
  if label.startswith('tagged private'):
   exception='upstream 0.22.0 cannot re-import tagged private values'
   passed=reference['exit_code']!=0 and reference['unchanged'] and 'private object SECRET' in reference['stderr'] and candidate['exit_code']==0
   if label.endswith('round trip'):passed=passed and candidate['unchanged'] and reference['cards']==candidate['cards']
   else:passed=passed and candidate['cards'][0]['model']['Private']=={'SECRET':{'account':'private update'}} and candidate['cards'][0]['unknown_preserved']
  cases.append({'case':label,'passed':passed,'expected_difference':exception,'reference':reference,'candidate':candidate})
 report={'reference_version':command(args.reference,['--version']).strip(),'synthetic_only':True,'passed':all(c['passed'] for c in cases),'cases':cases};Path(args.report).write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
 for c in cases:
  if not c['passed']:print(json.dumps(c,ensure_ascii=False))
 print(f"{sum(c['passed'] for c in cases)}/{len(cases)} edit cases passed; report: {args.report}");raise SystemExit(0 if report['passed'] else 1)
if __name__=='__main__':main()

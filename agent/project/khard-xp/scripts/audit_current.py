#!/usr/bin/env python3
"""Current upstream audit: disposable books, native locales and semantic YAML."""
import argparse
import json
import os
import re
from pathlib import Path
import subprocess
import tempfile
from ruamel.yaml import YAML

FIXTURES = {
 'alice': ('family','VERSION:4.0\nUID:alpha-one\nN:Example;Alice;Marie;Dr.;Jr.\nFN:Alice Example\nNICKNAME:Ally\nKIND:individual\nTEL;TYPE=work:+15550100002\nTEL;TYPE=home;PREF=1:+15550100003\nTEL;TYPE=work:+15550100001\nEMAIL;TYPE=work:z@example.invalid\nEMAIL;TYPE=home:a@example.invalid\nIMPP;PREF=1:xmpp:alice@example.invalid\nADR;TYPE=home:Box;Extended;First Street,Second Street;City;Region;10000;Country\nORG:Example;Engineering\nTITLE:Engineer\nitemrole1.ROLE:Developer\nitemrole1.X-ABLabel:job\nBDAY:19900403\nANNIVERSARY:20000102\nCATEGORIES:Friends,Test\\,One\nCATEGORIES:Other\nURL:https://example.invalid\nNOTE:Old\\nsecond line\nitemprivate1.X-SECRET:private value\nitemprivate1.X-ABLabel:account\nX-UNKNOWN:preserve\n'),
 'zoe': ('family','VERSION:4.0\nUID:alpha-two\nN:Zoë;Zoë;;;\nFN:Zoë Example\nEMAIL;TYPE=work:zoe@example.invalid\nBDAY:--04-02\n'),
 'ake': ('work','VERSION:4.0\nUID:beta-one\nN:Åke;Åke;;;\nFN:Åke Example\nKIND:org\nTEL;TYPE=home:+15550100009\nBDAY;VALUE=text:Spring equinox\n'),
 'anne': ('work','VERSION:4.0\nUID:beta-two\nN:Änne;Änne;;;\nFN:Änne Example\nBDAY:20000102T120000\n'),
 'li': ('family','VERSION:4.0\nUID:chinese-li\nN:李;明;;;\nFN:李明\nBDAY:19900506\nADR;TYPE=home:;;中山路一号;上海;;200000;中国\nitem1.EMAIL:li@example.invalid\nitem1.X-ABLabel:工作\n'),
 'wang': ('work','VERSION:4.0\nUID:chinese-wang\nN:王;小明;;;\nFN:王小明\nBDAY:19900607\n'),
 'zhang': ('work','VERSION:4.0\nUID:chinese-zhang\nN:張;美華;;;\nFN:張美華\nBDAY:19900708\n'),
 'osten': ('work','VERSION:4.0\nUID:gamma\nN:Östen;Östen;;;\nFN:Östen Example\nBDAY:20000201\n'),
}

def normalize_yaml(text):
 def clean(value):
  if hasattr(value,"isoformat"):return value.isoformat()
  if isinstance(value,dict):
   out={k:x for k,v in value.items() if (x:=clean(v)) not in (None,'',[],{})}
   if isinstance(out.get('Private'),dict):
    for key,item in out['Private'].items():
     if isinstance(item,list) and len(item)==1 and isinstance(item[0],dict):out['Private'][key]=item[0]
   return out
  if isinstance(value,list):return [clean(v) for v in value]
  return value
 return clean(YAML(typ='safe').load(text))

def normalize_pretty(text):
 lines=[];kind=False
 for line in text.splitlines(keepends=True):
  if line.startswith('Kind:'):
   if kind:continue
   kind=True
  lines.append(line)
 return ''.join(lines)

def run(exe,config,args,env):
 p=subprocess.run([exe,'-c',str(config),*args],capture_output=True,text=True,env=env,timeout=15)
 return {'exit_code':p.returncode,'stdout':p.stdout,'stderr':p.stderr}

def main():
 parser=argparse.ArgumentParser(description=__doc__)
 parser.add_argument('--reference',default='.obj/upstream-current/bin/khard')
 parser.add_argument('--candidate',default='.bin/khard')
 parser.add_argument('--report',default='.obj/current_upstream_audit.json')
 parser.add_argument('--locale-path',default='.obj/locales')
 args=parser.parse_args();cases=[]
 with tempfile.TemporaryDirectory(prefix='khard final audit ') as directory:
  root=Path(directory)
  for book in ['family','work']:(root/book).mkdir()
  for name,(book,data) in FIXTURES.items():(root/book/(name+'.vcf')).write_text('BEGIN:VCARD\n'+data+'END:VCARD\n')
  config=root/'khard.conf'
  base=f"[addressbooks]\n[[family]]\npath = '{root/'family'}'\n[[work]]\npath = '{root/'work'}'\n[general]\ndefault_action = list\n[contact table]\ndisplay = formatted_name\nsort = formatted_name\nlocalize_dates = LOCALIZE\nshow_uids = yes\npreferred_phone_number_type = work, pref\npreferred_email_address_type = work,\n[vcard]\nprivate_objects = SECRET,\nsearch_in_source_files = no\n"
  commands=[('list',[]),('list parsable',['list','-p']),('list grouped',['list','-g']),('list reversed',['list','-r']),('list uid scope',['list','-a','family','name:Alice']),('list names',['list','-d','last_name','-s','last_name']),('list fields',['list','-F','uid,name,phone,email,kind,impp']),('list collection',['list','-p','-F','uid,categories,roles,post_addresses,emails,impp']),('list dates',['list','-p','-F','uid,birthday,anniversary']),('birthdays',['birthdays']),('birthdays parsable',['birthdays','-p']),('phone',['phone']),('phone parsable',['phone','-p']),('phone row refinement',['phone','-p','phone_numbers:100002']),('email',['email']),('email parsable',['email','-p','--remove-first-line']),('email row refinement',['email','-p','--remove-first-line','emails:home']),('postaddress',['postaddress']),('postaddress parsable',['postaddress','-p']),('addressbooks',['addressbooks']),('show yaml',['show','--format','yaml','uid:alpha-one']),('show vcard',['show','--format','vcard','uid:alpha-one']),('show pretty',['show','uid:alpha-one']),('filename',['filename','uid:alpha-one']),('template',['template']),('no birthdays',['birthdays','-p','name:missing']),('no email',['email','-p','--remove-first-line','name:missing']),('no phone',['phone','-p','name:missing']),('no address',['postaddress','-p','name:missing']),('kind abbreviation',['list','-p','kind:o']),('impp query',['list','-p','impp:xmpp'])]
  env=os.environ.copy();env['LOCPATH']=str(Path(args.locale_path).resolve())
  for locale in ['C','en_US.utf8','de_DE.UTF-8','sv_SE.UTF-8','is_IS.UTF-8','zh_CN.UTF-8','zh_TW.UTF-8']:
   env['LC_ALL']=locale
   for localize in [False,True]:
    config.write_text(base.replace('LOCALIZE','yes' if localize else 'no'))
    chosen=commands if locale in ['C','en_US.utf8'] else [c for c in commands if c[0] in ['list parsable','list grouped','list reversed','birthdays','show pretty','postaddress','email']]
    for label,command in chosen:
     reference=run(args.reference,config,command,env);candidate=run(args.candidate,config,command,env)
     expected=reference['stdout'];actual=candidate['stdout'];exception=None
     if label=='show pretty':expected=normalize_pretty(expected);exception='upstream duplicate Kind heading omitted'
     if label in ['show yaml','template']:
      try:expected=normalize_yaml(expected);actual=normalize_yaml(actual)
      except Exception as e:actual={'parse_error':str(e),'text':actual}
     if label in ['list','list grouped','list reversed','list names','list fields','list uid scope','birthdays','phone','email','postaddress']:
      padded_expected='\n'.join(re.sub(r' {2,}','\t',line.rstrip()) for line in expected.splitlines())
      padded_actual='\n'.join(re.sub(r' {2,}','\t',line.rstrip()) for line in actual.splitlines())
      if padded_expected==padded_actual and expected!=actual:
       exception='CJK display-cell padding corrected';expected=padded_expected;actual=padded_actual
     passed=reference['exit_code']==candidate['exit_code'] and expected==actual
     cases.append({'case':label,'locale':locale,'localize_dates':localize,'passed':passed,'expected_difference':exception,'reference':reference,'candidate':candidate,'normalized_reference':expected if not passed else None,'normalized_candidate':actual if not passed else None})
  nested=root/'discovery';(nested/'a'/'shared').mkdir(parents=True);(nested/'b'/'shared').mkdir(parents=True)
  env['LC_ALL']='C'
  for label,extra in [('discover',f"[[discovered]]\npath = '{nested}/**'\ntype = discover\n"),('literal interpolation',f"[[literal]]\npath = '{root}/%(unexpanded)s'\n")]:
   if label=='literal interpolation':(root/'%(unexpanded)s').mkdir()
   config.write_text(base.replace('[general]',extra+'[general]').replace('LOCALIZE','no'))
   reference=run(args.reference,config,['addressbooks'],env);candidate=run(args.candidate,config,['addressbooks'],env)
   def names(text):return sorted(text.splitlines())
   cases.append({'case':label,'passed':reference['exit_code']==candidate['exit_code']==0 and names(reference['stdout'])==names(candidate['stdout']),'reference':reference,'candidate':candidate})
 # Introspect the pinned reference parser to inventory every command/alias
 # and option. --help exercises candidate argument parsing without mutations.
 from khard.cli import create_parsers
 from khard.actions import Actions
 _,reference_parser=create_parsers()
 children=next(a for a in reference_parser._actions if isinstance(a,argparse._SubParsersAction))
 inventory={}
 for name in Actions.get_all():
  result=subprocess.run([args.candidate,name,'--help'],capture_output=True,text=True,timeout=10)
  cases.append({'case':'command alias '+name,'passed':result.returncode==0,'candidate':{'exit_code':result.returncode,'stderr':result.stderr}})
 for name in Actions.get_actions():
  options={}
  for action in children.choices[name]._actions:
   for flag in action.option_strings:
    values=[]
    if action.nargs!=0:
     value=next(iter(action.choices)) if action.choices else {'-F':'uid','--fields':'uid'}.get(flag,'test')
     values=[str(value)]
    result=subprocess.run([args.candidate,name,flag,*values,'--help'],capture_output=True,text=True,timeout=10)
    options[flag]={'accepted':result.returncode==0,'exit_code':result.returncode,'stderr':result.stderr}
  inventory[name]=options
  cases.append({'case':'option inventory '+name,'passed':all(v['accepted'] for v in options.values())})
 report={'interface_inventory':inventory,'reference_version':subprocess.run([args.reference,'--version'],capture_output=True,text=True,check=True).stdout.strip(),'synthetic_only':True,'passed':all(c['passed'] for c in cases),'cases':cases}
 Path(args.report).write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
 for case in cases:
  if not case['passed']:print(json.dumps({k:v for k,v in case.items() if k not in ['reference','candidate']},ensure_ascii=False))
 print(f"{sum(c['passed'] for c in cases)}/{len(cases)} current-upstream cases passed; report: {args.report}")
 raise SystemExit(0 if report['passed'] else 1)
if __name__=='__main__':main()

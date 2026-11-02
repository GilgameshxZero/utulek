#!/usr/bin/env python3
"""Upload only synthetic adapter tests into a temporary Windows directory.

Requires explicit permission for host writes. Deletes the directory afterward.
"""
import argparse
import base64
import io
from pathlib import Path
import subprocess
import zipfile


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('host', help='SSH user@host; no implicit host authorization')
    parser.add_argument('originals', type=Path)
    parser.add_argument('--full',action='store_true',help='Include real calendar, OAuth and Lynx helper suites')
    parser.add_argument('--lynx-package',type=Path,default=Path(__file__).resolve().parent/'.obj/lynx_windows/app')
    parser.add_argument('--only',choices=('adapters','calendar','html','oauth','recovery'))
    args = parser.parse_args()
    root = Path(__file__).resolve().parent
    archive = io.BytesIO()
    with zipfile.ZipFile(archive, 'w', zipfile.ZIP_DEFLATED) as bundle:
        bundle.write(root / 'test_adapters.py', 'test_adapters.py')
        bundle.write(root / '.obj/argv_observer.exe', 'argv_observer.exe')
        for source in (root / 'runtime').iterdir():
            bundle.write(source, 'runtime/' + source.name)
        for name in ('neomutt-html-open.sh', 'neomutt-ical-request.sh'):
            bundle.write(args.originals / name, 'originals/' + name)
        if args.full:
            for name in ('test_calendar.py','test_html.py','test_oauth.py','test_deployment.py','deployment.py','lynx.bat'):
                bundle.write(root/name,name)
            for name in ('calendar_oracle.json','html_oracle.json'):
                bundle.write(root/'.obj'/name,name)
            for source in (root/'python').iterdir():
                if source.is_file():
                    bundle.write(source,'python/'+source.name)
            for source in args.lynx_package.rglob('*'):
                if source.is_file():
                    bundle.write(source,'lynx_windows/'+source.relative_to(args.lynx_package).as_posix())
    payload = base64.b64encode(archive.getvalue()).decode('ascii')
    remote = '''import base64,io,json,pathlib,subprocess,sys,tempfile,zipfile
with tempfile.TemporaryDirectory(prefix='neomutt_test_driver_') as temporary:
 root=pathlib.Path(temporary)
 with zipfile.ZipFile(io.BytesIO(base64.b64decode(PAYLOAD))) as bundle:
  bundle.extractall(root)
 suites={'adapters':['test_adapters.py',str(root/'originals'),'--observer',str(root/'argv_observer.exe')]}
 if (root/'test_calendar.py').exists():
  suites['calendar']=['test_calendar.py','--oracle',str(root/'calendar_oracle.json')]
  suites['html']=['test_html.py','--oracle',str(root/'html_oracle.json'),'--windows-package',str(root/'lynx_windows')]
  suites['oauth']=['test_oauth.py']
  suites['recovery']=['test_deployment.py']
 selection=SELECTION
 if selection:suites={name:arguments for name,arguments in suites.items() if name==selection}
 reports={}
 code=0
 for name,arguments in suites.items():
  result=subprocess.run([sys.executable,str(root/arguments[0]),*arguments[1:]],capture_output=True,cwd=root)
  try: reports[name]=json.loads(result.stdout.decode('utf-8'))
  except Exception: reports[name]={'error':result.stdout.decode('utf-8',errors='replace'),'stderr':result.stderr.decode('utf-8',errors='replace')}
  if result.returncode: code=1
 print(json.dumps(reports,ensure_ascii=True,indent=2))
raise SystemExit(code)
'''.replace('PAYLOAD', repr(payload)).replace('SELECTION',repr(args.only))
    result = subprocess.run(['ssh', '-F', '/dev/null', '-o', 'BatchMode=yes', '-o',
                             'ConnectTimeout=10', args.host, 'python -'],
                            input=remote.encode(), stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    import sys
    sys.stdout.buffer.write(result.stdout)
    sys.stderr.buffer.write(result.stderr)
    raise SystemExit(result.returncode)


if __name__ == '__main__':
    main()

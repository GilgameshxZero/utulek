#!/usr/bin/env python3
"""Read a helper-produced synthetic attachment in an isolated real browser."""
import json,os,subprocess,sys
from pathlib import Path
helper,root,browser=map(Path,sys.argv[1:]);helper=helper.resolve();browser=browser.resolve();root=root.resolve();root.mkdir(parents=True,exist_ok=True)
source=root/'source 日本語.html';source.write_text('<html><body><h1>HTML browser fixture 日本語</h1></body></html>',encoding='utf-8')
marker=root/'opened.json';capture=root/'capture.py';capture.write_text('import json,os,pathlib,sys\npathlib.Path(os.environ["FIXTURE_OPEN_MARKER"]).write_text(json.dumps(sys.argv[1:]),encoding="utf-8")\n')
if os.name=='nt':
 opener=root/'capture.bat';opener.write_text('@echo off\npython3 -X utf8 "%~dp0capture.py" %*\n')
else:
 opener=root/'capture';opener.write_text('#!/usr/bin/env python3\n'+capture.read_text());opener.chmod(0o700)
env=dict(os.environ,LANG="C.UTF-8",BROWSER=opener.as_posix(),FIXTURE_OPEN_MARKER=str(marker))
run=subprocess.run(['bash',helper.as_posix(),source.as_posix()],cwd=root,env=env,capture_output=True,timeout=15)
(root/'helper_stderr.txt').write_bytes(run.stderr)
if run.returncode:print(run.stderr.decode('utf-8',errors='replace'));run.check_returncode()
args=json.loads(marker.read_text());page=Path(args[0]);assert page.is_absolute() and page.is_file()
uri=page.as_uri();command=[str(browser),'--headless=new','--disable-gpu','--no-first-run','--no-default-browser-check','--user-data-dir='+str(root/'browser_profile'),'--dump-dom',uri]
run=subprocess.run(command,capture_output=True,timeout=45)
(root/'browser_dom.html').write_bytes(run.stdout);(root/'browser_stderr.txt').write_bytes(run.stderr)
text=run.stdout.decode('utf-8',errors='replace');results=dict(exit=run.returncode,absolute_file_uri=uri,readable=page.is_file(),content_preserved='HTML browser fixture 日本語' in text,passed=run.returncode==0 and 'HTML browser fixture 日本語' in text)
(root/'results.json').write_text(json.dumps(results,indent=2));print(json.dumps(results,ensure_ascii=True));sys.exit(not results['passed'])

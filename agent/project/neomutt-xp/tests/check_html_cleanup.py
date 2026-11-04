#!/usr/bin/env python3
"""Check failed input, opener failure and overlapping cleanup timer safety."""
import json,os,subprocess,sys,time
from pathlib import Path
helper=Path(sys.argv[1]).resolve();root=Path(sys.argv[2]).resolve();root.mkdir(parents=True,exist_ok=True)
marker=root/'opened.json';capture=root/'capture.py';capture.write_text('import json,os,pathlib,sys\npathlib.Path(os.environ["FIXTURE_OPEN_MARKER"]).write_text(json.dumps(sys.argv[1:]))\nsys.exit(int(os.environ.get("FIXTURE_BROWSER_EXIT","0")))\n')
if os.name=='nt':
 opener=root/'capture.bat';opener.write_text('@echo off\npython3 -X utf8 "%~dp0capture.py" %*\nexit /b %errorlevel%\n')
else:
 opener=root/'capture';opener.write_text('#!/usr/bin/env python3\n'+capture.read_text());opener.chmod(0o700)
env=dict(os.environ,LANG='C.UTF-8',BROWSER=opener.as_posix(),FIXTURE_OPEN_MARKER=str(marker))
source=root/'source.html';source.write_text('<p>first fixture 日本語</p>')
def launch(path,extra=None):
 return subprocess.run(['bash',helper.as_posix(),path.as_posix()],cwd=root,env=dict(env,**(extra or {})),capture_output=True,timeout=15)
results=[]
first_started=time.monotonic();run=launch(source,dict(FIXTURE_BROWSER_EXIT='37'));assert run.returncode==37,(run.returncode,run.stderr)
page=Path(json.loads(marker.read_text())[0]);old=page.read_bytes();results.append(dict(case='browser_failure_propagated',passed=True))
run=launch(root/'does_not_exist.html');assert run.returncode!=0 and page.read_bytes()==old
results.append(dict(case='missing_input_preserves_page',passed=True));print(json.dumps(results),flush=True)
while time.monotonic()<first_started+8:time.sleep(.1)
source.write_text('<p>newer fixture 日本語</p>');second_started=time.monotonic();run=launch(source);assert run.returncode==0,(run.returncode,run.stderr)
while time.monotonic()<first_started+66:time.sleep(.1)
assert page.is_file() and b'newer fixture' in page.read_bytes(),'Older timer removed newer page'
results.append(dict(case='old_timer_preserves_new_page',passed=True));print(json.dumps(results[-1]),flush=True)
while time.monotonic()<second_started+67:time.sleep(.1)
assert not page.exists(),'Current page timer failed to clean up'
results.append(dict(case='current_timer_removes_own_page',passed=True));assert not list(root.glob('.neomutt-html-open.??????'))
(root/'results.json').write_text(json.dumps(results,indent=2));print(json.dumps(results),flush=True)

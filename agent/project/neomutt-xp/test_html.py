#!/usr/bin/env python3
"""Compare native Lynx HTML dumps against the existing Linux renderer."""
import argparse
import base64
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile


CASES = {
    'plain': '<html><body><p>Hello world.</p><p>Second paragraph.</p></body></html>',
    'heading_ascii': '<h1>Plain title</h1><p>Body</p>',
    'heading_accents': '<meta charset="utf-8"><h1>éééééé</h1><p>Body</p>',
    'unicode': '<meta charset="utf-8"><h1>Café 世界</h1><p>Greek αβγ and emoji ☀.</p>',
    'links': '<p><a href="https://example.test/a">Visible link</a></p><a href="https://hidden.example.test/"></a>',
    'lists': '<h2>List</h2><ol><li>First</li><li>Second<ul><li>Nested</li></ul></li></ol>',
    'table': '<table><tr><th>Name</th><th>Value</th></tr><tr><td>A</td><td>42</td></tr></table>',
    'images': '<p>Before <img src="https://example.test/a.png" alt="Picture label"> after.</p>',
    'whitespace': '<p>lots     of\n whitespace &amp; &lt;entities&gt;&nbsp;here.</p><pre>a  b\n c</pre>',
    'broken': '<html><body><p>Unclosed <b>bold<p>another paragraph &nbsp; end',
    'long': '<p>' + 'long wrapped sentence with words ' * 30 + '</p>',
}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--oracle',type=Path,required=True)
    parser.add_argument('--write-oracle',action='store_true')
    parser.add_argument('--windows-package',type=Path)
    args = parser.parse_args()
    oracle = {} if args.write_oracle else json.loads(args.oracle.read_text())
    results=[]
    with tempfile.TemporaryDirectory(prefix='neomutt html ü ') as temporary:
        root=Path(temporary)
        if os.name=='nt':
            app=root/'app/lynx'
            shutil.copytree(args.windows_package,app)
            (root/'bin').mkdir()
            shutil.copy2(Path(__file__).with_name('lynx.bat'),root/'bin/lynx.bat')
        for name,html in CASES.items():
            path=root/(name+' & ü.html')
            path.write_text(html,encoding='utf-8')
            if os.name=='nt':
                command='"'+str(root/'bin/lynx')+'" -dump -hiddenlinks=ignore "'+str(path)+'"'
            else:
                command=['lynx','-dump','-hiddenlinks=ignore',str(path)]
            result=subprocess.run(command,shell=os.name=='nt',capture_output=True,timeout=15)
            actual={'exit':result.returncode,'stdout_b64':base64.b64encode(result.stdout.replace(b'\r\n',b'\n')).decode()}
            if args.write_oracle:
                oracle[name]=actual
            else:
                expected_bytes=base64.b64decode(oracle[name]['stdout_b64'])
                actual_bytes=base64.b64decode(actual['stdout_b64'])
                # User accepts small native-Lynx layout differences. Keep exact
                # comparison in the report; tolerate only line-edge whitespace.
                trim=lambda value:b'\n'.join(line.strip(b' \t') for line in value.split(b'\n'))
                equivalent=actual['exit']==oracle[name]['exit'] and trim(actual_bytes)==trim(expected_bytes)
                results.append({'name':name,'passed':equivalent,'byte_identical':actual==oracle[name],
                    'detail':'' if actual==oracle[name] else {'expected':oracle[name],'actual':actual,
                        'stderr':result.stderr.decode('utf-8',errors='replace')}})
    if args.write_oracle:
        args.oracle.write_text(json.dumps(oracle,indent=2,ensure_ascii=True))
        print('Recorded',len(oracle),'Linux Lynx fixtures')
    else:
        print(json.dumps({'results':results},indent=2,ensure_ascii=True))
        return 0 if all(row['passed'] for row in results) else 1
    return 0


if __name__=='__main__':
    raise SystemExit(main())

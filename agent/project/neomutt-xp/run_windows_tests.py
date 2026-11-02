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
    payload = base64.b64encode(archive.getvalue()).decode('ascii')
    remote = '''import base64,io,pathlib,subprocess,sys,tempfile,zipfile
with tempfile.TemporaryDirectory(prefix='neomutt_test_driver_') as temporary:
 root=pathlib.Path(temporary)
 with zipfile.ZipFile(io.BytesIO(base64.b64decode(PAYLOAD))) as bundle:
  bundle.extractall(root)
 result=subprocess.run([sys.executable,str(root/'test_adapters.py'),str(root/'originals'),'--observer',str(root/'argv_observer.exe')],capture_output=True)
 sys.stdout.buffer.write(result.stdout)
 sys.stderr.buffer.write(result.stderr)
 code=result.returncode
raise SystemExit(code)
'''.replace('PAYLOAD', repr(payload))
    result = subprocess.run(['ssh', '-F', '/dev/null', '-o', 'BatchMode=yes', '-o',
                             'ConnectTimeout=10', args.host, 'python -'],
                            input=remote.encode(), stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    import sys
    sys.stdout.buffer.write(result.stdout)
    sys.stderr.buffer.write(result.stderr)
    raise SystemExit(result.returncode)


if __name__ == '__main__':
    main()

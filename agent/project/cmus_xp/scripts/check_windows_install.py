#!/usr/bin/env python3
"""Read-only verification of synced Windows binaries and actual launchers."""
import argparse
import base64
import json
from pathlib import Path
import subprocess


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--host', required=True)
    parser.add_argument('--checkpoint', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    expected = json.loads(args.checkpoint.read_text())
    code = r'''$p=Join-Path $env:USERPROFILE 'main.syncthing/monochrome';$a=Join-Path $p 'app';$b=Join-Path $p 'bin';$result=@{};
foreach($name in @('cmus.exe','cmus-remote.exe')) {$result[$name]=(Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $a $name)).Hash.ToLower()};
$result.version=(& (Join-Path $b 'cmus.bat') --version | Out-String);
$result.remote_version=(& (Join-Path $b 'cmus-remote.bat') --version | Out-String);
$result.runtime=(& (Join-Path $b 'cmus.bat') --check-runtime | Out-String);$result.runtime_exit=$LASTEXITCODE;
$result|ConvertTo-Json -Depth 5'''
    encoded = base64.b64encode(('$ErrorActionPreference="Stop";\n' + code).encode('utf-16le')).decode()
    run = subprocess.run(['ssh', '-F', '/dev/null', '-o', 'BatchMode=yes', '-o',
        'ConnectTimeout=8', args.host, 'powershell.exe', '-NoProfile', '-NonInteractive',
        '-EncodedCommand', encoded], capture_output=True, text=True, timeout=45)
    run.check_returncode()
    report = json.loads(run.stdout)
    version = expected['version'].split()[-1]
    report['hashes_match'] = all(report[name] == expected['files'][name] for name in ('cmus.exe', 'cmus-remote.exe'))
    report['passed'] = (report['hashes_match'] and report['runtime_exit'] == 0 and
        version in report['version'] and version in report['remote_version'])
    args.output.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report, indent=2))
    if not report['passed']:
        raise SystemExit(1)


if __name__ == '__main__':
    main()

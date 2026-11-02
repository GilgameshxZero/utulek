#!/usr/bin/env python3
"""Assemble a verified-candidate layout and portable recovery pointer; no install."""
import argparse
from datetime import datetime,timezone
import hashlib
import json
import os
from pathlib import Path
import shutil
from stage_config import stage


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('monochrome',type=Path)
    parser.add_argument('output',type=Path)
    parser.add_argument('--lynx',type=Path,required=True)
    parser.add_argument('--baseline',type=Path,required=True)
    args=parser.parse_args()
    project=Path(__file__).resolve().parent
    source=args.monochrome.resolve()
    output=args.output.resolve()
    if output.exists():
        raise FileExistsError('Choose a fresh release directory')
    baseline=json.loads((args.baseline/'manifest.json').read_text())
    for name,record in baseline.items():
        path=source/'module/email'/name
        if hashlib.sha256(path.read_bytes()).hexdigest()!=record['sha256']:
            raise RuntimeError('Live source changed since parity tests: '+name)
    email=output/'module/email'
    stage(source/'module/email',email)
    # Original .sh helpers and mailcap remain in place, including for already
    # running Linux sessions. Only the new template is deployed.
    for name in ('mailcap','neomutt-html-open.sh','neomutt-ical-request.sh'):
        (email/name).unlink()
    stamp=datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ')
    backup=source/'app/neomutt-xp-rollback'/stamp
    pointer={'manifest_relative_to_recovery':stamp+'/manifest.json'}
    recovery=output/'app/neomutt-xp-rollback'
    recovery.mkdir(parents=True)
    (recovery/'current.json').write_text(json.dumps(pointer,indent=2))
    app=output/'app/lynx'
    app.mkdir(parents=True)
    for name in ('lynx.exe','lynx.cfg','lynx.lss'):
        shutil.copy2(args.lynx/name,app/name)
    for name in ('COPYING','COPYHEADER','README'):
        shutil.copy2(args.lynx/'doc'/name,app/name)
    (output/'bin').mkdir()
    shutil.copy2(project/'lynx.bat',output/'bin/lynx.bat')
    # xdg-open accepts one file/URL. Its old %* wrapper double-quotes an
    # already quoted path; %~1 removes that extra quote layer safely.
    opener=(source/'bin/xdg-open.bat').read_text()
    if '"%*"' not in opener:
        raise ValueError('Review changed xdg-open launcher before preparing release')
    (output/'bin/xdg-open.bat').write_text(opener.replace('"%*"','"%~1"'),newline='')
    persistent=['module/email/'+name for name in ('neomutt-revert','neomutt-revert.bat',
                'neomutt-rollback.py','deployment.py')]+['app/neomutt-xp-rollback/current.json']
    plan={'package':str(output),'installation_root':str(source),'backup':str(backup),'persistent':persistent}
    (output.parent/(output.name+'_plan.json')).write_text(json.dumps(plan,indent=2))
    print(json.dumps({'files':sum(path.is_file() for path in output.rglob('*')),'backup':str(backup)},indent=2))


if __name__=='__main__':
    main()

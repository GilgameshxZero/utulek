#!/usr/bin/env python3
"""Exercise install/read-back, guarded rollback and repeat rollback in a fixture."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
from deployment import install,restore


def main():
    results=[]
    with tempfile.TemporaryDirectory(prefix='neomutt_recovery_') as temporary:
        root=Path(temporary)/'root'
        package=Path(temporary)/'package'
        target=root/'module/email/neomuttrc'
        target.parent.mkdir(parents=True)
        target.write_bytes(b'old config\n')
        source=package/'module/email/neomuttrc'
        source.parent.mkdir(parents=True)
        source.write_bytes(b'new config\n')
        (source.parent/'added.py').write_bytes(b'fixture\n')
        project=Path(__file__).resolve().parent
        for path in ('neomutt-revert','neomutt-revert.bat'):
            shutil.copy2(project/'runtime'/path,source.parent/path)
        shutil.copy2(project/'python/neomutt-rollback.py',source.parent/'neomutt-rollback.py')
        shutil.copy2(project/'deployment.py',source.parent/'deployment.py')
        pointer=package/'app/neomutt-xp-rollback/current.json'
        pointer.parent.mkdir(parents=True)
        pointer.write_text(json.dumps({'manifest_relative_to_recovery':'test/manifest.json'}))
        persistent=['module/email/'+name for name in ('neomutt-revert','neomutt-revert.bat',
            'neomutt-rollback.py','deployment.py')]+['app/neomutt-xp-rollback/current.json']
        sentinel=root/'module/email/token.gpg'
        sentinel.write_bytes(b'untouched fixture\n')
        manifest=install(root,package,root/'app/neomutt-xp-rollback/test',persistent=persistent)
        results.append({'name':'install_readback','passed':target.read_bytes()==b'new config\n'})
        expected=restore(manifest)
        results.append({'name':'rollback_dry_run','passed':len(expected)==2 and target.read_bytes()==b'new config\n'})
        if os.name=='nt':
            command='"'+str(target.parent/'neomutt-revert')+'" --check'
        else:
            command=[str(target.parent/'neomutt-revert'),'--check']
        response=subprocess.run(command,shell=os.name=='nt',capture_output=True)
        results.append({'name':'paired_recovery_launcher_arguments','passed':response.returncode==0,
                        'detail':response.stderr.decode(errors='replace')})
        target.write_bytes(b'user edit\n')
        try:
            restore(manifest,apply=True)
        except RuntimeError:
            results.append({'name':'protect_post_install_edits','passed':True})
        else:
            results.append({'name':'protect_post_install_edits','passed':False})
        target.write_bytes(b'new config\n')
        restore(manifest,apply=True)
        results.append({'name':'rollback_readback','passed':target.read_bytes()==b'old config\n' and
                        not (target.parent/'added.py').exists()})
        results.append({'name':'unrelated_token_untouched','passed':sentinel.read_bytes()==b'untouched fixture\n'})
        results.append({'name':'repeat_rollback','passed':restore(manifest,apply=True)==[]})
    print(json.dumps({'results':results},indent=2))
    return 0 if all(row['passed'] for row in results) else 1


if __name__=='__main__':
    raise SystemExit(main())

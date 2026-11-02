"""Atomic file deployment and guarded rollback, using only the standard library."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import tempfile


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def inside(root, relative):
    path=(root/relative).resolve()
    if not path.is_relative_to(root.resolve()):
        raise ValueError('Deployment path escaped its root')
    return path


def replace_file(source, destination, mode=None):
    destination.parent.mkdir(parents=True,exist_ok=True)
    descriptor,temporary=tempfile.mkstemp(prefix='.neomutt_xp_',dir=destination.parent)
    os.close(descriptor)
    try:
        shutil.copyfile(source,temporary)
        os.chmod(temporary,mode if mode is not None else Path(source).stat().st_mode & 0o777)
        os.replace(temporary,destination)
    finally:
        Path(temporary).unlink(missing_ok=True)


def install(root, package, backup, persistent=()):
    root,package,backup=Path(root).resolve(),Path(package).resolve(),Path(backup).resolve()
    if not backup.is_relative_to(root):
        raise ValueError('Recovery snapshot must be inside the synchronized installation root')
    if backup.exists():
        raise FileExistsError('Refusing to overwrite an existing rollback snapshot')
    files=sorted(path for path in package.rglob('*') if path.is_file())
    plan=[]
    backup.mkdir(parents=True)
    for source in files:
        relative=source.relative_to(package).as_posix()
        target=inside(root,relative)
        if target.is_symlink():
            raise ValueError('Refusing to replace a symlink: '+relative)
        record={'path':relative,'installed_sha256':digest(source),
                'before_sha256':digest(target) if target.exists() else None,
                'before_mode':target.stat().st_mode&0o777 if target.exists() else None,
                'persistent':relative in persistent}
        if target.exists():
            original=backup/'original'/relative
            original.parent.mkdir(parents=True,exist_ok=True)
            shutil.copy2(target,original)
        plan.append(record)
    manifest={'version':1,'root_relative_to_backup':os.path.relpath(root,backup),
              'files':plan}
    # Configuration switches last, after all helpers and resources are present.
    ordered=sorted(plan,key=lambda item:item['path'].endswith('/neomuttrc'))
    changed=[]
    try:
        for record in ordered:
            target=inside(root,record['path'])
            current=digest(target) if target.exists() else None
            if current!=record['before_sha256']:
                raise RuntimeError('File changed while staging installation: '+record['path'])
            replace_file(package/record['path'],target)
            changed.append(record)
            if digest(target)!=record['installed_sha256']:
                raise RuntimeError('Installed-file verification failed: '+record['path'])
        (backup/'manifest.json').write_text(json.dumps(manifest,indent=2),encoding='utf-8')
    except BaseException:
        for record in reversed(changed):
            target=inside(root,record['path'])
            if record['before_sha256'] is None:
                target.unlink(missing_ok=True)
            else:
                replace_file(backup/'original'/record['path'],target,record['before_mode'])
        raise
    return backup/'manifest.json'


def restore(manifest_path, apply=False, force=False):
    manifest_path=Path(manifest_path).resolve()
    backup=manifest_path.parent
    manifest=json.loads(manifest_path.read_text(encoding='utf-8'))
    root=(backup/manifest['root_relative_to_backup']).resolve()
    changes=[]
    for record in manifest['files']:
        if record.get('persistent'):
            continue
        target=inside(root,record['path'])
        current=digest(target) if target.exists() else None
        # A previously completed rollback is harmless to invoke again.
        if current==record['before_sha256']:
            continue
        if not force and current!=record['installed_sha256']:
            raise RuntimeError('File edited after installation; refusing to overwrite it: '+record['path'])
        if record['before_sha256'] is not None:
            original=inside(backup/'original',record['path'])
            if digest(original)!=record['before_sha256']:
                raise RuntimeError('Recovery copy failed its checksum: '+record['path'])
        changes.append(record)
    if apply:
        # Restore the config first so it ceases to reference added helpers.
        for record in sorted(changes,key=lambda item:not item['path'].endswith('/neomuttrc')):
            target=inside(root,record['path'])
            if record['before_sha256'] is None:
                target.unlink(missing_ok=True)
            else:
                replace_file(backup/'original'/record['path'],target,record['before_mode'])
        for record in changes:
            target=inside(root,record['path'])
            current=digest(target) if target.exists() else None
            if current!=record['before_sha256']:
                raise RuntimeError('Rollback read-back failed: '+record['path'])
    return [record['path'] for record in changes]

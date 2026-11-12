#!/usr/bin/env python3
"""Stage an out-of-source build under .obj; keep finished programs under .bin.

The source tree is never modified. Only files whose size or mtime changed are
copied, and files removed from the source are removed from the stage, so
incremental builds stay correct. ``clean`` removes build outputs only; pinned
dependencies, checkpoints and evidence under .obj are left alone.
"""
import os
from pathlib import Path
import shutil
import subprocess
import sys

ROOT = Path(__file__).resolve().parent.parent
STAGE = ROOT / '.obj/build'
MANIFEST = ROOT / '.obj/build.manifest'
IGNORE = {'.obj', '.bin', '.git', 'out', 'build', 'dist', '__pycache__', 'config'}
GENERATED = (('scripts/build.mk', 'Makefile'), ('scripts/configure_project.sh', 'configure'))
OUTPUTS = {'.version', 'config.mk', 'embedded_plugins.c', 'embedded_data.c',
           'cmus', 'cmus-remote', '.install.log'}


def sources():
    for base, dirs, files in os.walk(ROOT):
        dirs[:] = sorted(d for d in dirs if d not in IGNORE)
        base = Path(base)
        for name in files:
            if name.endswith(('.pyc', '.pyo', '.o', '.lo', '.so', '.a', '.d')):
                continue
            if base == ROOT and name in OUTPUTS:
                continue
            yield base / name


def sync():
    STAGE.mkdir(parents=True, exist_ok=True)
    previous = set(MANIFEST.read_text().splitlines()) if MANIFEST.is_file() else set()
    current = set()
    for source in sources():
        relative = source.relative_to(ROOT)
        if relative in (Path('Makefile'), Path('configure')):
            continue
        current.add(relative.as_posix())
        destination = STAGE / relative
        try:
            stat = source.stat()
            old = destination.stat()
            if old.st_size == stat.st_size and int(old.st_mtime) == int(stat.st_mtime):
                continue
        except OSError:
            pass
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(source, destination)
    for relative in previous - current:
        stale = STAGE / relative
        if stale.is_file() or stale.is_symlink():
            stale.unlink()
    for source, destination in GENERATED:
        target = STAGE / destination
        shutil.copy2(ROOT / source, target)
        if destination == 'configure':
            target.chmod(0o755)
    MANIFEST.parent.mkdir(parents=True, exist_ok=True)
    MANIFEST.write_text(''.join(line + '\n' for line in sorted(current)))
    return STAGE


def clean():
    for path in (STAGE, ROOT / '.obj/linux_unbundled', ROOT / '.obj/native'):
        if path.is_dir():
            for base, _, _ in os.walk(path):
                os.chmod(base, 0o700)
            shutil.rmtree(path)
    MANIFEST.unlink(missing_ok=True)


def main():
    action, *args = sys.argv[1:]
    if action == 'clean':
        clean()
        return
    stage = sync()
    command = ['sh', './configure', *args] if action == 'configure' else ['make', *args]
    raise SystemExit(subprocess.call(command, cwd=stage))


if __name__ == '__main__':
    main()

#!/usr/bin/env python3
"""Stage the upstream build under .obj; keep finished programs under .bin."""
import os
from pathlib import Path
import shutil
import subprocess
import sys

ROOT = Path(__file__).resolve().parent.parent
STAGE = ROOT / '.obj/build'


def main():
    action, *args = sys.argv[1:]
    if action == 'clean':
        for path in (ROOT / '.obj', ROOT / '.bin'):
            if path.exists():
                for base, _, _ in os.walk(path):
                    os.chmod(base, 0o700)
                shutil.rmtree(path)
        return
    shutil.copytree(ROOT, STAGE, dirs_exist_ok=True,
                    ignore=shutil.ignore_patterns('.obj', '.bin', 'out', 'build', 'dist',
                                                 '__pycache__', '.git'))
    shutil.copy2(ROOT / 'scripts/build.mk', STAGE / 'Makefile')
    shutil.copy2(ROOT / 'scripts/configure_project.sh', STAGE / 'configure')
    (ROOT / '.bin').mkdir(exist_ok=True)
    if action == 'configure':
        command = ['sh', './configure', *args]
    else:
        command = ['make', *args]
    raise SystemExit(subprocess.call(command, cwd=STAGE))


if __name__ == '__main__':
    main()

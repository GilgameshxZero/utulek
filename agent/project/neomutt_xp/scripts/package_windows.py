#!/usr/bin/env python3
"""Stage the single native executable; libraries and baseline data are embedded.

Existing GPG and user-configured calendar/OAuth helpers remain external by design.
"""
from pathlib import Path
import shutil
ROOT = Path(__file__).resolve().parents[1]
legacy = ROOT / '.bin/windows/neomutt'
if legacy.exists():
    shutil.rmtree(legacy)
print(ROOT / '.bin/windows/neomutt.exe')

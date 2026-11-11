#!/usr/bin/env python3
"""Export modified upstream files as reviewable patches against pinned archives."""
import difflib
import json
from pathlib import Path
import tarfile
ROOT = Path(__file__).resolve().parents[1]
manifest = json.loads((ROOT / 'dependencies.json').read_text())
for name in ('neomutt', 'ncurses', 'lmdb'):
    item = manifest[name]
    source = ROOT / ('.obj/upstream' if name == 'neomutt' else f'.obj/deps/{name}-{item["version"]}')
    changes = []
    with tarfile.open(ROOT / '.obj/downloads' / item['archive']) as archive:
        for entry in archive.getmembers():
            if not entry.isfile(): continue
            relative = '/'.join(entry.name.split('/')[1:])
            target = source / relative
            if not target.is_file(): continue
            original = archive.extractfile(entry).read()
            current = target.read_bytes()
            if original == current: continue
            changes.extend(difflib.unified_diff(original.decode().splitlines(True),
                           current.decode().splitlines(True), fromfile='a/' + relative,
                           tofile='b/' + relative))
    destination = ROOT / 'patches' / (name + '-native_windows.patch')
    destination.write_text(''.join(changes))
    print(f'{destination}: {len(changes)} lines')

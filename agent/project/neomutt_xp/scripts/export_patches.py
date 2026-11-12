#!/usr/bin/env python3
"""Export modified upstream files as reviewable patches against pinned archives."""
import argparse
import difflib
import tarfile

from deps import MANIFEST, ROOT, source


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('names', nargs='*', default=['neomutt', 'ncurses', 'lmdb'])
    args = parser.parse_args()
    for name in args.names:
        item = MANIFEST[name]
        work = source(name)
        changes = []
        with tarfile.open(ROOT / '.obj/downloads' / item['archive']) as archive:
            for entry in archive.getmembers():
                if not entry.isfile():
                    continue
                relative = '/'.join(entry.name.split('/')[1:])
                target = work / relative
                if not target.is_file():
                    continue
                original = archive.extractfile(entry).read()
                current = target.read_bytes()
                if original == current:
                    continue
                changes.extend(difflib.unified_diff(
                    original.decode(errors='surrogateescape').splitlines(True),
                    current.decode(errors='surrogateescape').splitlines(True),
                    fromfile='a/' + relative, tofile='b/' + relative))
        destination = ROOT / 'patches' / (name + '-native_windows.patch')
        destination.write_text(''.join(changes))
        print(f'{destination}: {len(changes)} lines')


if __name__ == '__main__':
    main()

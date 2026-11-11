#!/usr/bin/env python3
"""Read an Arch/Manjaro build host's bundle owners, versions and notices.

Print JSON to stdout; do not modify the host. Run on the original build host
with its portable manifests as arguments. This captures installed-package
provenance, not a complete corresponding-source archive or distribution approval.
"""
import argparse
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path


def fields(path):
    sections = path.read_text().strip().split('\n\n')
    return {lines[0].strip('%'): lines[1:] for section in sections
            if (lines := section.splitlines()) and lines[0].startswith('%')}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('manifests', type=Path, nargs='+')
    args = parser.parse_args()
    names = set()
    for path in args.manifests:
        names.update(json.loads(path.read_text())['libraries'])
    # Resolve the same locations traversed by the Linux packager.
    roots = [Path('/usr/lib'), Path('/usr/lib64')]
    roots += sorted(Path('/usr/lib').glob('*-linux-gnu'))
    resolved = {name: set() for name in sorted(names)}
    roots = {root.resolve() for root in roots if root.is_dir()}
    roots = {root for root in roots if not any(root != other and root.is_relative_to(other)
                                              for other in roots)}
    for root in sorted(roots):
        for path in root.rglob('*'):
            if path.name in names and path.is_file():
                resolved[path.name].add(str(path.resolve()))
    resolved = {name: sorted(paths) for name, paths in resolved.items()}
    targets = {p.lstrip('/') for paths in resolved.values() for p in paths}
    targets.update(('usr/share/alsa/alsa.conf', 'usr/share/pipewire/pipewire.conf',
                    'usr/share/terminfo/x/xterm-256color'))
    owners, packages = {}, {}
    for directory in sorted(Path('/var/lib/pacman/local').iterdir()):
        if not (directory / 'files').is_file():
            continue
        owned = targets.intersection(fields(directory / 'files').get('FILES', []))
        if not owned:
            continue
        metadata = fields(directory / 'desc')
        name = metadata['NAME'][0]
        packages[name] = {'metadata': metadata, 'notices': {}}
        for path in owned:
            owners['/' + path] = name
        notice_dir = Path('/usr/share/licenses') / name
        if notice_dir.is_dir():
            for path in sorted(notice_dir.rglob('*')):
                if path.is_file():
                    packages[name]['notices'][str(path)] = path.read_text(errors='replace')
    common = Path('/usr/share/licenses/common')
    common_notices = {str(p): p.read_text(errors='replace') for p in sorted(common.rglob('*'))
                      if p.is_file()}
    identifiers = {'GPL-2.0-or-later', 'GPL-3.0-or-later', 'LGPL-2.1-or-later'}
    for package in packages.values():
        for license_name in package['metadata'].get('LICENSE', []):
            identifiers.update(license_name.split())
    for path in sorted(Path('/usr/share/licenses/spdx').rglob('*.txt')):
        if path.stem in identifiers:
            common_notices[str(path)] = path.read_text()
    libraries = {}
    for name, paths in resolved.items():
        libraries[name] = [{'path': p, 'package': owners.get(p),
                            'sha256': hashlib.file_digest(Path(p).open('rb'), 'sha256').hexdigest()}
                           for p in paths]
    print(json.dumps({'utc': datetime.now(timezone.utc).isoformat(),
        'os_release': Path('/etc/os-release').read_text(), 'libraries': libraries,
        'data_owners': {p: owners.get('/' + p) for p in sorted(targets) if p.startswith('usr/share/')},
        'packages': packages, 'common_notices': common_notices,
        'scope': 'Post-build installed-package inventory; bundled ELF files may be patched by StaticX. '
                 'Exact distribution source packages and build recipes must accompany publication.'}, indent=2))


if __name__ == '__main__':
    main()

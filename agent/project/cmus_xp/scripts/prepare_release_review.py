#!/usr/bin/env python3
"""Prepare an offline local source/provenance review kit, never publish it.

Includes checked source archives and recovered notices. Distribution blockers
are explicit: this kit does not pretend to contain all distribution-library
sources or the precise MinGW runtime source used by the binary toolchain.
"""
import argparse
from datetime import datetime, timezone
import json
from pathlib import Path
import shutil

from build_linux_support import ARCHIVES
from checkpoint_release import digest, write_source_archive

ROOT = Path(__file__).resolve().parent.parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--evidence-root', type=Path, required=True)
    parser.add_argument('--linux-provenance', type=Path, required=True)
    parser.add_argument('--gate-report', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--evidence', type=Path, action='append', default=[])
    args = parser.parse_args()
    gate = json.loads(args.gate_report.read_text())
    if gate.get('passed') is not True or len(gate.get('checks', [])) < 3:
        raise SystemExit('A passing release gate is required')
    provenance = json.loads(args.linux_provenance.read_text())
    if not provenance['libraries'] or any(not rows or any(not row.get('package') for row in rows)
                                          for rows in provenance['libraries'].values()):
        raise SystemExit('Unresolved Linux library provenance')
    out = args.output.resolve()
    out.mkdir(parents=True, exist_ok=False)
    write_source_archive(out / 'source.tar.gz')
    shutil.copy2(args.gate_report, out / 'validation.json')
    shutil.copy2(args.linux_provenance, out / 'linux_provenance.json')
    evidence_dir = out / 'evidence'
    evidence_dir.mkdir()
    for path in args.evidence:
        data = json.loads(path.read_text(encoding='utf-8-sig'))
        rows = data if isinstance(data, list) else [data]
        if not rows or not all(row.get('passed') for row in rows):
            raise SystemExit('Supplemental evidence failed: ' + str(path))
        destination = evidence_dir / path.name
        if destination.exists():
            raise SystemExit('Duplicate evidence filename: ' + path.name)
        shutil.copy2(path, destination)
    notices = out / 'notices'
    shutil.copytree(ROOT / 'windows/licenses', notices / 'windows')
    for name, data in provenance['packages'].items():
        directory = notices / 'linux' / name
        directory.mkdir(parents=True)
        (directory / 'package.json').write_text(json.dumps(data['metadata'], indent=2) + '\n')
        for index, (origin, text) in enumerate(sorted(data['notices'].items())):
            (directory / ('%02d_' % index + Path(origin).name)).write_text(text)
    common = notices / 'linux/common'
    common.mkdir()
    for origin, text in provenance['common_notices'].items():
        (common / Path(origin).name).write_text(text)
    if not (common / 'GPL-3.0-or-later.txt').is_file():
        raise SystemExit('Missing GPLv3 notice for bundled Linux components')
    staticx_license = args.evidence_root / ('deps/packaging_env/lib/python3.14/site-packages/'
                                           'staticx-0.14.2.dist-info/licenses/LICENSE.txt')
    if not staticx_license.is_file():
        candidates = list((args.evidence_root / 'deps/packaging_env').glob(
            'lib/python*/site-packages/staticx-0.14.2.dist-info/licenses/LICENSE.txt'))
        if len(candidates) != 1:
            raise SystemExit('Missing pinned StaticX license')
        staticx_license = candidates[0]
    shutil.copy2(staticx_license, notices / 'staticx.txt')
    pins = json.loads((ROOT / 'scripts/windows_dependencies.json').read_text())
    sources = {name: dict(spec, role='build_tool' if name == 'llvm_mingw' else 'source')
               for name, spec in pins.items()}
    for archive, (checksum, directory) in ARCHIVES.items():
        sources[directory] = {'archive': archive, 'sha256': checksum, 'role': 'source'}
    archive_dir = out / 'archives'
    archive_dir.mkdir()
    for name, spec in sources.items():
        candidates = [args.evidence_root / 'deps/sources' / spec['archive'],
                      args.evidence_root / 'deps' / spec['archive'],
                      ROOT / '.obj/deps' / spec['archive']]
        archive = next((p for p in candidates if p.is_file()), None)
        if archive is None or digest(archive) != spec['sha256']:
            raise SystemExit('Missing or changed pinned archive: ' + spec['archive'])
        shutil.copy2(archive, archive_dir / spec['archive'])
    manifest = {
        'utc': datetime.now(timezone.utc).isoformat(), 'review_complete': True,
        'daily_use_gate_passed': True, 'publication_ready': False,
        'candidate_files': gate['files'], 'archives': sources,
        'linux_package_count': len(provenance['packages']),
        'linux_library_names': len(provenance['libraries']),
        'distribution_blockers': [
            'Exact corresponding sources, downstream patches and build recipes for the bundled '
            'Linux distribution libraries and runtime data are not included.',
            'LLVM-MinGW binary archive and runtime notices are included; the precise corresponding '
            'MinGW/winpthreads runtime source revision still needs to be assembled.',
            'Post-build package ownership records do not prove byte-for-byte provenance of '
            'StaticX-patched libraries; future packaging must record input hashes and package versions.',
            'The Linux bundle includes GPL-3.0-or-later components (libcdio/libcdio-paranoia); '
            'review combined distribution terms rather than labelling the whole bundle GPLv2-only.'
        ],
        'files': {str(p.relative_to(out)): digest(p) for p in sorted(out.rglob('*')) if p.is_file()}}
    (out / 'review.json').write_text(json.dumps(manifest, indent=2) + '\n')
    print(out / 'review.json')


if __name__ == '__main__':
    main()

#!/usr/bin/env python3
"""Repeatable local release gate; consumes hash-bound Windows test evidence.

Runs isolated real Linux interactions, checks decoder/fuzz evidence, and records
artifact hashes and timings. Does not install, upload, commit, or publish.
"""
import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tarfile
import time
import uuid

ROOT = Path(__file__).resolve().parent.parent


def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def require(condition, message):
    if not condition:
        raise ValueError(message)


def load_json(path):
    return json.loads(path.read_text(encoding='utf-8-sig'))


def relocated_evidence(root, original):
    require('/.obj/' in original, 'Evidence must belong to the project .obj tree')
    path = (root / original.split('/.obj/', 1)[1]).resolve()
    require(path.is_relative_to(root.resolve()), 'Evidence escapes its root')
    return path


def runtime_source(path):
    return (path.suffix in ('.c', '.h') and 'tests' not in path.parts
            and not path.name.startswith('check_')) or path.parts[0] == 'data'


def runtime_files(root):
    for directory, children, names in os.walk(root):
        children[:] = [name for name in children if not name.startswith('.') and name != 'out']
        for name in names:
            path = Path(directory) / name
            if runtime_source(path.relative_to(root)):
                yield path


def verify_evidence(root, files):
    """Check saved hashes and required coverage before executing any test player."""
    verified = {}
    checkpoints = {}
    for name in ('02_persistence', '03_large_library', '04_validation'):
        directory = root / 'checkpoints' / name
        checkpoint = load_json(directory / 'checkpoint.json')
        checkpoints[name] = checkpoint
        require(checkpoint['files'] == files, 'Checkpoint candidate mismatch: ' + name)
        require(checkpoint.get('windows_synced_verified') is True,
                'Missing actual Windows installation verification: ' + name)
        source = directory / 'source.tar.gz'
        require(digest(source) == checkpoint['source_sha256'], 'Changed checkpoint source: ' + name)
        for original, expected in checkpoint['evidence'].items():
            path = relocated_evidence(root, original)
            require(digest(path) == expected, 'Changed evidence: ' + str(path))
            verified[str(path)] = {'sha256': expected}

    persistence_paths = [relocated_evidence(root, name)
                         for name in checkpoints['02_persistence']['evidence']]
    persistence = [load_json(p) for p in persistence_paths if p.suffix == '.json']
    require(any(row.get('passed') is True and len(row.get('checks', [])) == 4
                and not row.get('changed_after_failed_save') for row in persistence),
            'Missing successful persistence/failure/restart coverage')
    require(any(p.suffix == '.txt' and 'playlist source checks: PASS' in p.read_text()
                for p in persistence_paths), 'Missing actual Windows playlist persistence coverage')
    library_paths = [relocated_evidence(root, name)
                     for name in checkpoints['03_large_library']['evidence']]
    libraries = [load_json(p) for p in library_paths if p.suffix == '.json']
    require(any(row.get('passed') is True and row.get('count') == 50000
                and len(row.get('checks', [])) == 5 for row in libraries),
            'Missing actual Linux 50000-file coverage')
    require(any(p.suffix == '.txt' and 'large library checks: PASS' in p.read_text()
                for p in library_paths), 'Missing actual Windows 50000-file coverage')

    # Reject stale binaries when production C/resources changed. Test harnesses,
    # release scripts and documentation may evolve without rebuilding the player.
    with tarfile.open(root / 'checkpoints/04_validation/source.tar.gz') as archive:
        expected_sources = {}
        for member in archive.getmembers():
            relative = Path(member.name).relative_to('cmus_xp')
            if member.isfile() and relative.parts and runtime_source(relative):
                expected_sources[relative] = hashlib.sha256(archive.extractfile(member).read()).hexdigest()
    current_sources = {p.relative_to(ROOT): digest(p) for p in runtime_files(ROOT)}
    require(expected_sources == current_sources, 'Runtime source differs from tested checkpoint; rebuild and revalidate')

    validation = load_json(root / 'milestones/validation/results.json')
    require(validation.get('passed') is True, 'Validation milestone failed')
    groups = [('milestones/validation/decoders/results_native_sanitized.json', 56),
              ('milestones/validation/windows/results.json', 56),
              ('hardening/malformed/results.json', 400), ('reliability/results.json', 4)]
    for relative, count in groups:
        rows = load_json(root / relative)
        require(isinstance(rows, list) and len(rows) == count and
                all(row.get('passed') in (True, 1) for row in rows), 'Incomplete coverage: ' + relative)
        verified[str(root / relative)]['checks'] = count
    alpine = relocated_evidence(root, validation['alpine_report'])
    require(str(alpine) in {str(Path(p).resolve()) for p in verified}, 'Alpine report is not hash-bound')
    alpine_data = load_json(alpine)
    require(alpine_data.get('passed') is True and len(alpine_data['checks']) == 5,
            'Incomplete alternate-distribution coverage')
    return verified


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--linux-dir', type=Path, required=True)
    parser.add_argument('--windows-exe', type=Path, required=True)
    parser.add_argument('--windows-report', type=Path, required=True)
    parser.add_argument('--evidence-root', type=Path, required=True,
                        help='Original or recovered .obj with checkpoints 02–04 and their evidence')
    parser.add_argument('--rootfs', type=Path)
    parser.add_argument('--evidence', type=Path, action='append', default=[])
    args = parser.parse_args()
    out = ROOT / '.obj/release_gate' / uuid.uuid4().hex[:12]
    out.mkdir(parents=True)
    files = {n: digest(args.linux_dir / n) for n in ('cmus', 'cmus-remote')}
    files['cmus.exe'] = digest(args.windows_exe)
    windows = load_json(args.windows_report)
    report = {'utc': datetime.now(timezone.utc).isoformat(), 'files': files,
              'windows': windows, 'checks': [], 'evidence': {}, 'passed': False}
    binary_args = ['--binary', str((args.linux_dir / 'cmus').resolve()),
                   '--remote', str((args.linux_dir / 'cmus-remote').resolve())]
    checks = [('single_file', 'check_single_file.py', [], ['dbus-run-session', '--']),
              ('persistence', 'check_persistence.py', ['--long-paths'], []),
              ('portable_runtime', 'check_portable_runtime.py', [], [])]
    if args.rootfs:
        checks.append(('alternate_distribution', 'check_portable_runtime.py',
                       ['--rootfs', str(args.rootfs.resolve())], []))
    try:
        require(windows.get('passed') is True and windows.get('sha256') == files['cmus.exe'],
                'Windows evidence does not match candidate')
        require(windows.get('native_large_library') is True and windows.get('decoder_checks') == 56
                and windows.get('runtime') == 'Windows read-only runtime checks: 5005 passed, 0 failed',
                'Windows report lacks required runtime/decoder/library coverage')
        report['windows_report_sha256'] = digest(args.windows_report)
        report['evidence'] = verify_evidence(args.evidence_root.resolve(), files)
        for name, script, extra, prefix in checks:
            start = time.monotonic()
            with (out / (name + '.log')).open('w') as log:
                process = subprocess.run(prefix + [sys.executable, str(ROOT / 'scripts' / script),
                    *binary_args, *extra], stdout=log, stderr=subprocess.STDOUT, timeout=90)
            report['checks'].append({'name': name, 'passed': process.returncode == 0,
                                     'seconds': time.monotonic() - start})
            if process.returncode:
                raise SystemExit('Release check failed: ' + name + '; see ' + str(out))
        for path in args.evidence:
            data = load_json(path)
            rows = data if isinstance(data, list) else [data]
            require(rows and all(row.get('passed') for row in rows), 'Failing evidence: ' + str(path))
            report['evidence'][str(path.resolve())] = {'sha256': digest(path), 'checks': len(rows)}
        report['passed'] = True
    except (OSError, ValueError, KeyError, TypeError, subprocess.TimeoutExpired) as error:
        report['error'] = str(error)
        raise SystemExit('Release gate failed: ' + str(error) + '; see ' + str(out)) from error
    finally:
        (out / 'results.json').write_text(json.dumps(report, indent=2) + '\n')
    print(out / 'results.json')


if __name__ == '__main__':
    main()

#!/usr/bin/env python3
"""Final local release gate for cmus_xp 1.0.0.

Verifies the checkpoint chain, installed artifacts, pinned dependency archives
and a fresh Linux plugin/version smoke check. Does not publish or upload.
"""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parent.parent
MONOCHROME = ROOT.parents[3] / 'monochrome'


def digest(path):
    return hashlib.file_digest(path.open('rb'), 'sha256').hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--linux-dir', type=Path,
                        default=ROOT / '.obj/ck14/linux_candidate/app')
    parser.add_argument('--windows-exe', type=Path,
                        default=ROOT / '.obj/ck14/windows_candidate/app/cmus.exe')
    parser.add_argument('--windows-install', type=Path,
                        default=ROOT / '.obj/ck14/windows_installed.json')
    parser.add_argument('--output', type=Path,
                        default=ROOT / '.obj/release_gate/final/results.json')
    args = parser.parse_args()
    checks = []

    def require(name, condition, detail=''):
        checks.append({'name': name, 'passed': bool(condition), 'detail': detail})
        return condition

    # 1. Checkpoint chain and evidence hashes.
    for name in ('06_compatibility_baseline', '07_remote_control', '08_streaming',
                 '09_cue', '10_native_audio', '11_platform_parity',
                 '12_terminal_input', '13_daily_use'):
        directory = ROOT / '.obj/checkpoints' / name
        checkpoint_path = directory / 'checkpoint.json'
        if not require(name + ' checkpoint present', checkpoint_path.is_file()):
            continue
        checkpoint = json.loads(checkpoint_path.read_text())
        source = directory / 'source.tar.gz'
        require(name + ' source archive hash',
                source.is_file() and digest(source) == checkpoint['source_sha256'])
        broken = []
        for original, expected in checkpoint.get('evidence', {}).items():
            if '/.obj/' in original:
                path = ROOT / '.obj' / original.split('/.obj/', 1)[1]
            else:
                path = Path(original)
            if not path.is_file() or digest(path) != expected:
                broken.append(str(path))
        require(name + ' evidence hashes', not broken, ','.join(broken))
        if name != '06_compatibility_baseline':
            require(name + ' Windows installation verified',
                    checkpoint.get('windows_synced_verified') is True)

    # 2. Final candidate runtime.
    version = subprocess.run([str(args.linux_dir / 'cmus'), '--version'],
                             capture_output=True, text=True)
    require('Linux version 1.0.0', version.returncode == 0 and
            version.stdout.splitlines()[0] == 'cmus 1.0.0')
    plugins = subprocess.run([str(args.linux_dir / 'cmus'), '--plugins'],
                             capture_output=True, text=True)
    for plugin in ('stream:', 'ffmpeg:', 'cue:', 'modplug:'):
        require('Linux plugin ' + plugin, plugin in plugins.stdout)

    # 3. Installed Linux artifacts equal the final candidates.
    installed = MONOCHROME / 'app'
    for name in ('cmus', 'cmus-remote'):
        require('installed Linux ' + name,
                (installed / name).is_file() and
                digest(installed / name) == digest(args.linux_dir / name))
    launcher = MONOCHROME / 'bin' / 'cmus'
    require('Linux launcher script', launcher.is_file() and
            launcher.read_text().startswith('#!/bin/sh'))

    # 4. Windows installation verification and candidate hash.
    if args.windows_install.is_file():
        windows = json.loads(args.windows_install.read_text())
        require('Windows synced hash and runtime', windows.get('passed') is True)
        require('Windows candidate hash',
                windows.get('cmus.exe') == digest(args.windows_exe))
        require('Windows launcher version', '1.0.0' in windows.get('version', ''))
    else:
        require('Windows installation report present', False, str(args.windows_install))

    # 5. Pinned archives.
    pins = json.loads((ROOT / 'scripts/windows_dependencies.json').read_text())
    from build_linux_support import ARCHIVES
    archives = {name: spec['archive'] for name, spec in pins.items()}
    for name, spec in pins.items():
        archive = ROOT / '.obj/deps' / spec['archive']
        require('pinned archive ' + name,
                archive.is_file() and digest(archive) == spec['sha256'])
    for archive, (checksum, directory) in ARCHIVES.items():
        candidates = [ROOT / '.obj/deps/sources' / archive, ROOT / '.obj/deps' / archive]
        path = next((p for p in candidates if p.is_file()), candidates[0])
        require('Linux support archive ' + directory,
                path.is_file() and digest(path) == checksum, str(path))

    report = {
        'version': '1.0.0',
        'passed': all(c['passed'] for c in checks),
        'checks': checks,
        'files': {
            'cmus': digest(args.linux_dir / 'cmus'),
            'cmus-remote': digest(args.linux_dir / 'cmus-remote'),
            'cmus.exe': digest(args.windows_exe),
        },
    }
    if (args.windows_exe.parent / 'cmus-remote.exe').is_file():
        report['files']['cmus-remote.exe'] = digest(args.windows_exe.parent / 'cmus-remote.exe')
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps({'passed': report['passed'],
                      'failed': [c['name'] for c in checks if not c['passed']]}, indent=2))
    return 0 if report['passed'] else 1


if __name__ == '__main__':
    raise SystemExit(main())

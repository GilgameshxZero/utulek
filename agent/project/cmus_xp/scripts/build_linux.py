#!/usr/bin/env python3
"""Build/install cmus_xp with embedded Linux plugins and runtime resources.

Requires the private dependencies prepared by prepare_linux.py, plus system
development headers for the native audio plugins. Runtime libraries are bundled.
No downloads, package installation, or remote operations are performed.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import re
import shlex
import subprocess
import sys

from build_local import sync as stage_sources

ROOT = Path(__file__).resolve().parent.parent
# Match the distro cmus feature set instead of silently dropping native plugins
# and their saved options merely because FFmpeg can decode the same file types.
LINUX_FEATURES = ('AAC', 'ALSA', 'AO', 'CDDB', 'CDIO', 'CUE', 'DISCID', 'FFMPEG',
                  'FLAC', 'JACK', 'MAD', 'MODPLUG', 'MPC', 'MPRIS', 'OPUS',
                  'PULSE', 'SAMPLERATE', 'VORBIS', 'WAV', 'WAVPACK')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--install-dir', type=Path, default=ROOT.parents[3] / 'monochrome/app')
    parser.add_argument('--launcher-dir', type=Path, default=ROOT.parents[3] / 'monochrome/bin')
    parser.add_argument('--audio-libs', type=Path)
    parser.add_argument('--packaging-python', type=Path,
                        default=ROOT / '.obj/deps/packaging_env/bin/python')
    parser.add_argument('--compress', action='store_true',
                        help='Smaller portable files at the cost of startup decompression')
    parser.add_argument('--jobs', type=int, default=min(os.cpu_count() or 2, 8))
    args = parser.parse_args()
    if not sys.platform.startswith('linux'):
        parser.error('This entry point builds the Linux application')
    if args.jobs < 1:
        parser.error('--jobs must be positive')
    if not args.packaging_python.is_file():
        parser.error('Packaging Python not found; prepare a private environment with '
                     'scripts/linux_packaging_requirements.txt')
    app = args.install_dir.resolve()
    launcher = args.launcher_dir.resolve()
    env = dict(os.environ)
    http = ROOT / '.obj/deps/http_host'
    if not (http / 'lib/libcurl.a').is_file():
        parser.error('Static HTTP libraries missing; run scripts/build_http_deps.py --host first')
    env['PKG_CONFIG_PATH'] = str(http / 'lib/pkgconfig') + os.pathsep + env.get('PKG_CONFIG_PATH', '')
    support = ROOT / '.obj/deps/linux'
    if not (support / 'lib/libao.a').is_file():
        parser.error('Embedded libao drivers missing; prepare dependencies with scripts/prepare_linux.py')
    # The extracted distro development packages carry the remaining plugins.
    for base, pkg, include, lib in (
            (ROOT / '.obj/deps/linux_audio/usr', 'lib/x86_64-linux-gnu/pkgconfig',
             'include', 'lib/x86_64-linux-gnu'),
            (support, 'lib/pkgconfig', 'include', 'lib')):
        if not base.is_dir():
            continue
        for variable, directory in (('PKG_CONFIG_PATH', pkg), ('C_INCLUDE_PATH', include),
                                    ('LIBRARY_PATH', lib)):
            path = base / directory
            if path.is_dir():
                env[variable] = str(path) + os.pathsep + env.get(variable, '')
    if args.audio_libs is None and (ROOT / '.obj/deps/audio_host/lib/libavcodec.a').is_file():
        args.audio_libs = ROOT / '.obj/deps/audio_host'
    if args.audio_libs is None:
        parser.error('Static FFmpeg missing; prepare dependencies with scripts/prepare_linux.py')
    if args.audio_libs:
        path = str(args.audio_libs.resolve() / 'lib/pkgconfig')
        env['PKG_CONFIG_PATH'] = path + os.pathsep + env.get('PKG_CONFIG_PATH', '')
    subprocess.run(['pkg-config', '--exists', 'libcurl >= 8.23'], env=env, check=True)
    stage = stage_sources()
    configure = ['./configure', 'EMBEDDED=y']
    configure += ['CONFIG_' + name + '=n' for name in
                  ('OSS', 'SNDIO', 'ROAR', 'ARTS', 'COREAUDIO', 'WAVEOUT',
                   'AAUDIO', 'BASS', 'MIKMOD', 'MP4', 'VTX')]
    configure += ['CONFIG_' + name + '=y' for name in LINUX_FEATURES]
    if args.audio_libs:
        configure += ['FFMPEG_PKG_CONFIG=pkg-config --static']
    subprocess.run(configure, cwd=stage, env=env, check=True)
    raw = ROOT / '.obj/linux_unbundled'
    raw.mkdir(parents=True, exist_ok=True)
    subprocess.run(['make', '-j' + str(args.jobs), 'BINDIR=' + str(raw), 'main'],
                   cwd=stage, env=env, check=True)
    # Resolving a venv Python symlink loses the venv; preserve its original path.
    packaging_env = dict(env, PATH=str(args.packaging_python.absolute().parent) +
                         os.pathsep + env.get('PATH', ''))
    for name in ('cmus', 'cmus-remote'):
        finished = ROOT / '.bin' / name
        pending = finished.with_name(name + '.pending')
        command = [str(args.packaging_python.absolute()), str(ROOT / 'scripts/package_linux.py'),
                   str(raw / name), str(pending)]
        if name == 'cmus':
            command.append('--player')
        if args.compress:
            command.append('--compress')
        subprocess.run(command, env=packaging_env, check=True)
        pending.replace(finished)
    enabled = set(re.findall(r'^CONFIG_(\w+)\s*=\s*y\s*$',
                            (stage / 'config.mk').read_text(), re.MULTILINE))
    enabled.add('STREAM')
    # Verify both completed artifacts before replacing an installed program.
    for name in ('cmus', 'cmus-remote'):
        metadata = json.loads((ROOT / '.obj' / (name + '-portable.json')).read_text())
        if hashlib.sha256((ROOT / '.bin' / name).read_bytes()).hexdigest() != metadata['sha256']:
            raise SystemExit('Portable artifact checksum mismatch: ' + name)
        subprocess.run([str(ROOT / '.bin' / name), '--version'], check=True,
                       stdout=subprocess.DEVNULL)
    # All plugins and defaults are part of the executable.
    app.mkdir(parents=True, exist_ok=True)
    launcher.mkdir(parents=True, exist_ok=True)
    for name in ('cmus', 'cmus-remote'):
        binary = app / name
        # Atomic replacement permits updating an executable already running.
        pending = binary.with_name('.' + name + '.install')
        shutil.copy2(ROOT / '.bin' / name, pending)
        pending.replace(binary)
        link = launcher / name
        pending_link = launcher / ('.' + name + '.install')
        pending_link.unlink(missing_ok=True)
        relative = shlex.quote(os.path.relpath(binary, launcher))
        pending_link.write_text('#!/bin/sh\nexec "$(dirname -- "$0")"/' + relative + ' "$@"\n')
        pending_link.chmod(0o755)
        pending_link.replace(link)
    plugins = subprocess.check_output([str(app / 'cmus'), '--plugins'], text=True)
    if plugins.count('built into executable') != 2:
        raise SystemExit('Installed player does not have embedded plugins')
    if '  stream:\n' not in plugins:
        raise SystemExit('Installed player is missing the streaming plugin')
    # Remove only known files from the previous generated runtime tree.
    # Preserve any unrecognized files instead of deleting user additions.
    legacy = app / 'cmus_xp-runtime'
    if legacy.is_dir() and not legacy.is_symlink():
        for kind in ('ip', 'op'):
            directory = legacy / 'lib/cmus' / kind
            for path in directory.glob('*.so'):
                if (ROOT / kind / (path.stem + '.c')).is_file():
                    path.unlink()
        for path in (legacy / 'share/cmus').glob('*'):
            if (ROOT / 'data' / path.name).is_file() and (path.is_file() or path.is_symlink()):
                path.unlink()
        for directory in sorted(legacy.rglob('*'), key=lambda p: len(p.parts), reverse=True):
            if directory.is_dir() and not directory.is_symlink():
                try:
                    directory.rmdir()
                except OSError:
                    pass
        try:
            legacy.rmdir()
        except OSError:
            pass
    report = {'binary': str(app / 'cmus'), 'launcher': str(launcher / 'cmus'),
              'remote': str(app / 'cmus-remote'), 'embedded_resources': True,
              'features': sorted(enabled),
              'plugins': plugins,
              'legacy_files_preserved': sorted(str(p) for p in legacy.rglob('*') if p.is_file())}
    (ROOT / '.obj/linux_build.json').write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()

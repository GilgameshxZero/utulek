#!/usr/bin/env python3
"""Build/install cmus-xp with Linux audio plugins and its runtime resources.

Uses system development libraries by default. --audio-libs selects the optional
pinned static FFmpeg prefix; PKG_CONFIG_PATH can supply other local dependencies.
No downloads, package installation, or remote operations are performed.
"""
import argparse
import json
import os
from pathlib import Path
import shutil
import re
import shlex
import subprocess
import sys

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
    parser.add_argument('--jobs', type=int, default=min(os.cpu_count() or 2, 8))
    args = parser.parse_args()
    if not sys.platform.startswith('linux'):
        parser.error('This entry point builds the Linux application')
    if args.jobs < 1:
        parser.error('--jobs must be positive')
    app = args.install_dir.resolve()
    launcher = args.launcher_dir.resolve()
    runtime = app / 'cmus-xp-runtime'
    env = dict(os.environ)
    if args.audio_libs:
        path = str(args.audio_libs.resolve() / 'lib/pkgconfig')
        env['PKG_CONFIG_PATH'] = path + os.pathsep + env.get('PKG_CONFIG_PATH', '')
    configure = ['./configure', 'prefix=' + str(runtime), 'CONFIG_OSS=n']
    configure += ['CONFIG_' + name + '=y' for name in LINUX_FEATURES]
    if args.audio_libs:
        configure += ['FFMPEG_PKG_CONFIG=pkg-config --static']
    subprocess.run(configure, cwd=ROOT, env=env, check=True)
    subprocess.run(['make', '-j' + str(args.jobs), 'main', 'plugins'], cwd=ROOT, env=env, check=True)
    enabled = set(re.findall(r'^CONFIG_(\w+)\s*=\s*y\s*$',
                            (ROOT / '.obj/build/config.mk').read_text(), re.MULTILINE))
    # Only runtime inputs/outputs and themes are installed; build tools and
    # test executables stay in .bin/. Metadata is kept under ignored .obj/.
    app.mkdir(parents=True, exist_ok=True)
    for kind in ('ip', 'op'):
        target = runtime / 'lib/cmus' / kind
        target.mkdir(parents=True, exist_ok=True)
        plugins = [p for p in (ROOT / '.obj/build' / kind).glob('*.so')
                   if p.stem.upper() in enabled]
        for plugin in plugins:
            shutil.copy2(plugin, target / plugin.name)
        for stale in target.glob('*.so'):
            if stale.name not in {plugin.name for plugin in plugins}:
                stale.unlink()
    shutil.copytree(ROOT / 'data', runtime / 'share/cmus', dirs_exist_ok=True)
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
    report = {'binary': str(app / 'cmus'), 'launcher': str(launcher / 'cmus'),
              'remote': str(app / 'cmus-remote'), 'runtime': str(runtime),
              'features': sorted(enabled),
              'plugins': {kind: sorted(p.name for p in (runtime / 'lib/cmus' / kind).glob('*.so'))
                          for kind in ('ip', 'op')}}
    (ROOT / '.obj/linux_build.json').write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()

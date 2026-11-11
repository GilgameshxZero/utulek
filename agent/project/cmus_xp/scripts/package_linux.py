#!/usr/bin/env python3
"""Bundle Linux ELF dependencies, audio modules and terminal data with StaticX.

Run with the private packaging environment's Python. All libraries are inside
the resulting file; StaticX extracts a private temporary tree and removes it on
exit. No FUSE, Python or installed codec libraries are needed at runtime.
"""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import tempfile

from staticx.api import StaticxGenerator, verify_tools
from staticx.elf import get_shobj_deps
from staticx.version import __version__
from elftools.elf.elffile import ELFFile

ROOT = Path(__file__).resolve().parent.parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('binary', type=Path)
    parser.add_argument('output', type=Path)
    parser.add_argument('--player', action='store_true')
    parser.add_argument('--compress', action='store_true')
    args = parser.parse_args()
    if __version__ != '0.14.2':
        raise SystemExit('Use the pinned scripts/linux_packaging_requirements.txt')
    verify_tools()
    args.output.parent.mkdir(parents=True, exist_ok=True)
    temp = ROOT / '.obj/packaging_tmp'
    temp.mkdir(parents=True, exist_ok=True)
    tempfile.tempdir = str(temp)
    # Include the common ALSA bridges and PipeWire client modules loaded with
    # dlopen (which ordinary DT_NEEDED traversal cannot discover).
    modules = []
    data_dirs = []
    with tempfile.TemporaryDirectory(dir=temp) as work:
        data = Path(work)
        if args.player:
            # Ship only the distribution's public Mozilla roots, never local
            # additions from the build machine's merged certificate store.
            certificates = sorted(Path('/usr/share/ca-certificates/mozilla').glob('*.crt'))
            if not certificates:
                raise SystemExit('Missing public Mozilla CA certificates')
            cert_dir = data / 'certs'
            cert_dir.mkdir()
            (cert_dir / 'ca-certificates.crt').write_bytes(
                b'\n'.join(path.read_bytes() for path in certificates))
            data_dirs.append(cert_dir)
            lib_dirs = [Path('/usr/lib'), Path('/usr/lib64')]
            lib_dirs += sorted(Path('/usr/lib').glob('*-linux-gnu'))
            for lib in lib_dirs:
                for name in ('pcm_pulse', 'ctl_pulse', 'conf_pulse', 'pcm_pipewire',
                             'ctl_pipewire', 'pcm_jack', 'pcm_oss', 'ctl_oss',
                             'pcm_upmix', 'pcm_vdownmix', 'rate_samplerate'):
                    path = lib / 'alsa-lib' / ('libasound_module_' + name + '.so')
                    if path.is_file():
                        modules.append(path)
                for name in ('protocol-native', 'client-node', 'client-device',
                             'adapter', 'metadata', 'session-manager', 'rt'):
                    path = lib / 'pipewire-0.3' / ('libpipewire-module-' + name + '.so')
                    if path.is_file():
                        modules.append(path)
                for name in ('support/libspa-support.so', 'audioconvert/libspa-audioconvert.so'):
                    path = lib / 'spa-0.2' / name
                    if path.is_file():
                        modules.append(path)
                        link = data / 'spa' / name
                        link.parent.mkdir(parents=True, exist_ok=True)
                        if not link.is_symlink():
                            link.symlink_to('../../' + path.name)
            for name in ('terminfo', 'alsa', 'pipewire'):
                source = Path('/usr/share') / name
                if not source.is_dir():
                    raise SystemExit('Missing runtime data: ' + str(source))
                destination = data / name
                # Hardware firmware/topology/UCM are host resources, not needed
                # for the player's ALSA PCM interface.
                ignore = shutil.ignore_patterns('firmware', 'topology', 'ucm2', 'init')
                shutil.copytree(source, destination, symlinks=False, ignore=ignore)
                data_dirs.append(destination)
            if (data / 'spa').exists():
                data_dirs.append(data / 'spa')
        with StaticxGenerator(str(args.binary), strip=True, compress=args.compress) as generator:
            for directory in data_dirs:
                generator.sxar.add_file(str(directory), arcname=directory.name)
            for module in modules:
                generator.add_library(str(module), exist_ok=True)
                for dep in get_shobj_deps(str(module)):
                    generator.add_library(dep, exist_ok=True)
            generator.generate(str(args.output))
            libraries = sorted(generator._added_libs)
    args.output.chmod(0o755)
    with args.output.open('rb') as stream:
        elf = ELFFile(stream)
        if any(segment['p_type'] in ('PT_INTERP', 'PT_DYNAMIC') for segment in elf.iter_segments()):
            raise SystemExit('Portable output unexpectedly requires a host dynamic loader')
    final_name = args.output.name.removesuffix('.pending')
    report = {'binary': str(args.output.with_name(final_name)), 'staticx': __version__,
              'compressed': args.compress, 'bytes': args.output.stat().st_size,
              'sha256': hashlib.sha256(args.output.read_bytes()).hexdigest(),
              'libraries': libraries, 'audio_modules': sorted({p.name for p in modules}),
              'data': [p.name for p in data_dirs]}
    report_path = ROOT / '.obj' / (final_name + '-portable.json')
    report_path.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report))


if __name__ == '__main__':
    main()

#!/usr/bin/env python3
"""Build static, local-file audio decoding libraries from supplied FFmpeg source.

All build/install files stay in the project's ignored .obj/ directory.
No dependency downloads or remote operations are performed.
"""
import argparse
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parent.parent
DECODERS = ('opus,vorbis,flac,mp3,mp3float,mp2,mp2float,aac,aac_fixed,alac,'
            'ac3,eac3,wavpack,ape,tta,tak,wmav1,wmav2,wmapro,wmalossless,'
            'mpc7,mpc8,dsd_lsbf,dsd_msbf,pcm_*')
DEMUXERS = ('ogg,mp3,flac,mov,matroska,aac,ac3,eac3,wav,aiff,asf,ape,wv,'
            'tta,tak,mpc,mpc8,dsf,au')
PARSERS = 'opus,vorbis,flac,mpegaudio,aac,ac3'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--toolchain', type=Path, default=ROOT / '.obj/deps/llvm_mingw')
    parser.add_argument('--source', type=Path, default=ROOT / '.obj/deps/ffmpeg')
    parser.add_argument('--host', action='store_true', help='Build local Linux decoder-check libraries')
    parser.add_argument('--jobs', type=int, default=4)
    args = parser.parse_args()
    if args.jobs < 1:
        parser.error('--jobs must be positive')
    source = args.source.resolve()
    if not (source / 'configure').is_file():
        parser.error('FFmpeg source not found: ' + str(source))
    platform = 'host' if args.host else 'windows'
    build = ROOT / ('.obj/audio_' + platform)
    prefix = ROOT / ('.obj/deps/audio_' + platform)
    build.mkdir(parents=True, exist_ok=True)
    configure = [str(source / 'configure'), '--prefix=' + str(prefix),
                 '--disable-autodetect', '--disable-shared', '--enable-static',
                 '--disable-programs', '--disable-doc', '--disable-debug',
                 '--disable-network', '--disable-avdevice', '--disable-avfilter',
                 '--disable-swscale', '--disable-everything', '--disable-x86asm',
                 '--enable-avcodec', '--enable-avformat', '--enable-swresample',
                 '--enable-protocol=file', '--enable-decoder=' + DECODERS,
                 '--enable-demuxer=' + DEMUXERS, '--enable-parser=' + PARSERS]
    if not args.host:
        toolchain = args.toolchain.resolve() / 'bin'
        configure += ['--enable-cross-compile', '--target-os=mingw32', '--arch=x86_64',
                      '--cc=' + str(toolchain / 'x86_64-w64-mingw32-clang'),
                      '--ar=' + str(toolchain / 'llvm-ar'),
                      '--ranlib=' + str(toolchain / 'llvm-ranlib'),
                      '--nm=' + str(toolchain / 'llvm-nm'),
                      '--strip=' + str(toolchain / 'llvm-strip')]
    env = dict(os.environ, LC_ALL='C')
    subprocess.run(configure, cwd=build, env=env, check=True)
    subprocess.run(['make', '-j' + str(args.jobs)], cwd=build, env=env, check=True)
    subprocess.run(['make', 'install'], cwd=build, env=env, check=True)
    print(prefix)


if __name__ == '__main__':
    main()

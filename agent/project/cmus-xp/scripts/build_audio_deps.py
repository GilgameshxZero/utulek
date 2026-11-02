#!/usr/bin/env python3
"""Build static, local-file audio decoding libraries from supplied FFmpeg source.

All build/install files stay in the project's ignored .obj/ directory.
No dependency downloads or remote operations are performed.
"""
import argparse
import os
import platform
from pathlib import Path
import shutil
import subprocess

ROOT = Path(__file__).resolve().parent.parent
DECODERS = ('opus,vorbis,flac,mp3,mp3float,mp2,mp2float,aac,aac_fixed,alac,'
            'ac3,eac3,wavpack,ape,tta,tak,wmav1,wmav2,wmapro,wmalossless,'
            'mpc7,mpc8,dsd_lsbf,dsd_msbf,dsd_lsbf_planar,dsd_msbf_planar,'
            'dca,truehd,mlp,shorten,atrac3,atrac3p,amrnb,amrwb,gsm,gsm_ms,'
            'speex,nellymoser,sbc,adpcm_*,pcm_*')
DEMUXERS = ('ogg,mp3,flac,mov,matroska,aac,ac3,eac3,wav,aiff,asf,ape,wv,'
            'tta,tak,mpc,mpc8,dsf,dsdiff,au,dts,dtshd,truehd,mlp,shn,'
            'amr,rm,oma,flv,avi,mpegts,mpeg,sbc')
PARSERS = 'opus,vorbis,flac,mpegaudio,aac,ac3,dca,mlp,amr,gsm,sbc'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--toolchain', type=Path, default=ROOT / '.obj/deps/llvm_mingw')
    parser.add_argument('--source', type=Path, default=ROOT / '.obj/deps/ffmpeg')
    parser.add_argument('--host', action='store_true', help='Build local Linux decoder-check libraries')
    parser.add_argument('--jobs', type=int, default=4)
    parser.add_argument('--build-dir', type=Path, help='Alternate ignored build directory')
    parser.add_argument('--prefix', type=Path, help='Alternate static-library prefix')
    parser.add_argument('--disable-x86asm', action='store_true',
                        help='Explicit portable C fallback when NASM is unavailable')
    args = parser.parse_args()
    if args.jobs < 1:
        parser.error('--jobs must be positive')
    source = args.source.resolve()
    if not (source / 'configure').is_file():
        parser.error('FFmpeg source not found: ' + str(source))
    target_platform = 'host' if args.host else 'windows'
    build = (args.build_dir or ROOT / ('.obj/audio_' + target_platform)).resolve()
    prefix = (args.prefix or ROOT / ('.obj/deps/audio_' + target_platform)).resolve()
    build.mkdir(parents=True, exist_ok=True)
    configure = [str(source / 'configure'), '--prefix=' + str(prefix),
                 '--disable-autodetect', '--disable-shared', '--enable-static', '--enable-pic',
                 '--disable-programs', '--disable-doc', '--disable-debug',
                 '--disable-network', '--disable-avdevice', '--disable-avfilter',
                 '--disable-swscale', '--disable-everything', '--enable-runtime-cpudetect',
                 '--enable-avcodec', '--enable-avformat', '--enable-swresample',
                 '--enable-protocol=file', '--enable-decoder=' + DECODERS,
                 '--enable-demuxer=' + DEMUXERS, '--enable-parser=' + PARSERS]
    if args.disable_x86asm:
        configure += ['--disable-x86asm']
    elif not args.host or platform.machine().lower() in ('x86_64', 'amd64', 'i386', 'i686'):
        nasm = ROOT / '.obj/deps/nasm/nasm'
        assembler = str(nasm) if nasm.is_file() else shutil.which('nasm')
        if not assembler:
            parser.error('NASM required for optimized x86 audio; run '
                         'build_windows_release.py --prepare-only, install NASM, '
                         'or explicitly use --disable-x86asm')
        configure += ['--x86asmexe=' + assembler]
    if not args.host:
        toolchain = args.toolchain.resolve() / 'bin'
        configure += ['--enable-cross-compile', '--target-os=mingw32', '--arch=x86_64',
                      '--cc=' + str(toolchain / 'x86_64-w64-mingw32-clang'),
                      '--ar=' + str(toolchain / 'llvm-ar'),
                      '--ranlib=' + str(toolchain / 'llvm-ranlib'),
                      '--nm=' + str(toolchain / 'llvm-nm'),
                      '--strip=' + str(toolchain / 'llvm-strip')]
    else:
        # A same-release reference decoder avoids differences in system FFmpeg.
        # Hidden C symbols also allow these PIC static archives in Linux plugins.
        configure += ['--extra-cflags=-fvisibility=hidden', '--enable-ffmpeg',
                      '--enable-avfilter', '--enable-filter=aresample,aformat,anull',
                      '--enable-encoder=pcm_s16le,pcm_s32le',
                      '--enable-muxer=pcm_s16le,pcm_s32le', '--enable-protocol=pipe']
    env = dict(os.environ, LC_ALL='C')
    subprocess.run(configure, cwd=build, env=env, check=True)
    subprocess.run(['make', '-j' + str(args.jobs)], cwd=build, env=env, check=True)
    subprocess.run(['make', 'install'], cwd=build, env=env, check=True)
    print(prefix)


if __name__ == '__main__':
    main()

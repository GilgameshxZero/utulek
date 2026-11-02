#!/usr/bin/env python3
"""Cross-build the native Windows preview using LLVM-MinGW and PDCurses 3.9.

Builds locally, then installs cmus.exe in monochrome/app with a bin/cmus.bat launcher.
No downloads or remote operations are performed by this script.
"""
import argparse
import concurrent.futures
import json
import os
from pathlib import Path
import shutil
import subprocess

ROOT = Path(__file__).resolve().parent.parent
SOURCES = '''ape browser buffer cache channelmap cmdline cmus command_mode comment
convert cue cue_utils debug discid editable expr filters format_print gbuf glob
help history id3 input job keys keyval lib load_dir locking mergesort misc options
output pcm player play_queue pl pl_env rbtree read_wrapper search_mode search
 tabexp_file tabexp track_info track tree uchar u_collate ui_curses window worker
xstrjoin file path prog xmalloc'''.split()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--toolchain', type=Path, default=ROOT / '.obj/deps/llvm_mingw')
    parser.add_argument('--pdcurses', type=Path, default=ROOT / '.obj/deps/pdcurses')
    parser.add_argument('--audio-libs', type=Path, default=ROOT / '.obj/deps/audio_windows')
    parser.add_argument('--install-dir', type=Path,
                        default=ROOT.parents[3] / 'monochrome/app')
    parser.add_argument('--launcher-dir', type=Path,
                        default=ROOT.parents[3] / 'monochrome/bin')
    parser.add_argument('--jobs', type=int, default=4)
    args = parser.parse_args()
    if args.jobs < 1:
        parser.error('--jobs must be positive')
    cc = args.toolchain.resolve() / 'bin/x86_64-w64-mingw32-clang'
    pdc = args.pdcurses.resolve()
    audio = args.audio_libs.resolve()
    if not cc.is_file():
        parser.error('LLVM-MinGW compiler not found: ' + str(cc))
    if not (pdc / 'curses.h').is_file():
        parser.error('PDCurses source not found: ' + str(pdc))
    if not (audio / 'lib/libavformat.a').is_file():
        parser.error('Static audio libraries not found; run scripts/build_audio_deps.py first')
    out = ROOT / '.obj/native'
    out.mkdir(parents=True, exist_ok=True)
    config = out / 'config'
    config.mkdir(exist_ok=True)
    for name, content in {
        'libdir': '#define LIBDIR ""', 'datadir': '#define DATADIR ""',
        'debug': '#define DEBUG 1', 'wcwidth': '#define HAVE_WCWIDTH 0', 'mpris': '', 'iconv': '', 'utils': '',
        'ffmpeg': '',
        'xmalloc': '#define HAVE_STRDUP 1',
        'curses': '#define HAVE_RESIZETERM 1\n#define HAVE_USE_DEFAULT_COLORS 1\n#define HAVE_ITALIC 0',
    }.items():
        (config / (name + '.h')).write_text(content + '\n')

    # Embed upstream defaults and themes; no installed share/data tree needed.
    resources = []
    entries = []
    for i, path in enumerate(sorted((ROOT / 'data').iterdir())):
        if path.is_file():
            raw = path.read_bytes()
            if path.name == 'rc':
                raw += b'\n# Windows WaveOut has no hardware mixer in this preview.\nset softvol=true\nset softvol_state=100 100\n'
            resources.append('static const char resource_%d[] = {%s,0};' %
                             (i, ','.join(map(str, raw))))
            entries.append('{%s,resource_%d,%d}' % (json.dumps(':cmus-data:/' + path.name), i, len(raw)))
    resource_c = out / 'resources.c'
    resource_c.write_text('#include <string.h>\n#include <stddef.h>\n' + '\n'.join(resources) +
        '\nstatic const struct { const char *name, *data; size_t size; } resources[] = {\n' +
        ',\n'.join(entries) + '\n};\n' + '''
int win_resource(const char *name, const char **data, size_t *size) {
    for (size_t i = 0; i < sizeof(resources)/sizeof(resources[0]); i++)
        if (!strcmp(name, resources[i].name)) {
            *data = resources[i].data; *size = resources[i].size; return 1;
        }
    return 0;
}
''')
    common = [str(cc), '-std=gnu11', '-O2', '-g', '-D_FILE_OFFSET_BITS=64', '-ffile-prefix-map=' + str(ROOT) + '=.',
              '-D_WIN32_WINNT=0x0A00', '-DPDC_WIDE', '-DPDC_FORCE_UTF8',
              '-I' + str(pdc), '-I' + str(ROOT), '-I' + str(out), '-I' + str(audio / 'include')]
    native = ['-DCMUS_NATIVE_WINDOWS', '-DHAVE_CONFIG', '-DHAVE_STRDUP',
              '-DVERSION="cmus-win-0.3.0-dev"', '-D_CRT_SECURE_NO_WARNINGS',
              '-I' + str(ROOT / 'windows/include'), '-include', str(ROOT / 'windows/compat.h')]
    jobs = []
    for path in [ROOT / (s + '.c') for s in SOURCES] + [ROOT / 'ip/wav.c', ROOT / 'ip/ffmpeg.c',
                 ROOT / 'op/waveout.c', ROOT / 'windows/compat.c',
                 ROOT / 'windows/services.c', ROOT / 'windows/static_plugins.c',
                 ROOT / 'windows/curses_vt.c',
                 ROOT / 'windows/path_windows.c', ROOT / 'windows/check_runtime.c', resource_c]:
        obj = out / (path.parent.name + '_' + path.stem + '.o')
        extra = ['-DCMUS_WIN_IMPLEMENTATION'] if path.name == 'compat.c' else []
        if path.name == 'ffmpeg.c':
            extra += ['-DCMUS_FFMPEG_STEREO']
        if path.parent.name == 'ip':
            symbols = ['ip_ops', 'ip_priority', 'ip_extensions', 'ip_mime_types',
                       'ip_options', 'ip_abi_version']
            extra += ['-D' + symbol + '=' + path.stem + '_' + symbol for symbol in symbols]
        jobs.append((path, obj, common + extra + native))
    for path in sorted((pdc / 'pdcurses').glob('*.c')) + sorted((pdc / 'wincon').glob('*.c')):
        if path == pdc / 'wincon/pdcdisp.c':
            continue  # windows/curses_vt.c supplies the display adapter.
        obj = out / ('pdc_' + path.stem + '.o')
        jobs.append((path, obj, common))

    def compile_one(item):
        path, obj, flags = item
        result = subprocess.run(flags + ['-c', str(path), '-o', str(obj)], capture_output=True, text=True)
        if result.stderr:
            print(result.stderr, end='', flush=True)
        if result.returncode:
            return str(path.relative_to(ROOT))
        return None

    with concurrent.futures.ThreadPoolExecutor(max_workers=args.jobs) as pool:
        failures = [name for name in pool.map(compile_one, jobs) if name]
    if failures:
        raise SystemExit("Compilation failed: " + ", ".join(failures))
    exe = ROOT / '.bin/cmus.exe'
    exe.parent.mkdir(parents=True, exist_ok=True)
    subprocess.run([str(cc), '-static', '-Wl,--no-insert-timestamp', '-o', str(exe)] + [str(j[1]) for j in jobs] +
                   ['-L' + str(audio / 'lib'), '-lavformat', '-lavcodec', '-lswresample',
                    '-lavutil', '-lwinpthread', '-lwinmm', '-ladvapi32', '-luser32',
                    '-lbcrypt', '-lole32', '-lsecur32', '-lws2_32'], check=True)
    destination = args.install_dir.resolve() / 'cmus.exe'
    destination.parent.mkdir(parents=True, exist_ok=True)
    if destination != exe:
        shutil.copy2(exe, destination)
    launcher_dir = args.launcher_dir.resolve()
    launcher_dir.mkdir(parents=True, exist_ok=True)
    # Relative to the launcher so the installation can sync to Windows intact.
    relative_exe = os.path.relpath(destination, launcher_dir).replace('/', '\\')
    launcher = launcher_dir / 'cmus.bat'
    launcher.write_bytes(('@"%~dp0' + relative_exe + '" %*\r\n').encode('utf-8'))
    print(destination)
    print(launcher)


if __name__ == '__main__':
    main()

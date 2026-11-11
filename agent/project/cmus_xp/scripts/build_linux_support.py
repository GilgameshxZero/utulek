#!/usr/bin/env python3
"""Build private static libao/CDDB from supplied, pinned source archives.

No downloads or system installation. Libao's Pulse, ALSA and OSS drivers are
linked into its archive so it needs no runtime plugin directory.
"""
import argparse
import hashlib
from pathlib import Path
import subprocess
import tarfile

ROOT = Path(__file__).resolve().parent.parent
ARCHIVES = {
    'libao-1.2.2.tar.gz': ('cc9e68134e74f62a1feb6d07e4a0fb565351d80205d1e5d48d540b6abc4cf07d',
                         'libao-1.2.2+20180113'),
    'libcddb-1.3.2.tar.gz': ('93f7860bdafa26f21d60a4d0ce2a8e55a0573841116aade79d90b64651b2b664',
                          'libcddb-1.3.2'),
}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--archives', type=Path, default=ROOT / '.obj/deps/sources')
    parser.add_argument('--jobs', type=int, default=4)
    args = parser.parse_args()
    if args.jobs < 1:
        parser.error('--jobs must be positive')
    prefix = ROOT / '.obj/deps/linux'
    build = ROOT / '.obj/linux_support'
    build.mkdir(parents=True, exist_ok=True)
    for archive_name, (digest, dirname) in ARCHIVES.items():
        archive = args.archives / archive_name
        if hashlib.sha256(archive.read_bytes()).hexdigest() != digest:
            raise SystemExit('Source checksum mismatch: ' + str(archive))
        with tarfile.open(archive) as tf:
            tf.extractall(build, filter='data')
        source = build / dirname
        if dirname.startswith('libcddb'):
            path = source / 'lib/cddb_net.c'
            path.write_text(path.read_text().replace('size_t l;', 'socklen_t l;'))
        else:
            path = source / 'src/audio_out.c'
            text = path.read_text().replace('extern ao_functions ao_null;',
                'extern ao_functions ao_builtin_pulse, ao_builtin_alsa, ao_builtin_oss;\n'
                'extern ao_functions ao_null;')
            text = text.replace('static ao_functions *static_drivers[] = {',
                'static ao_functions *static_drivers[] = {\n'
                '&ao_builtin_pulse, &ao_builtin_alsa, &ao_builtin_oss,')
            path.write_text(text)
        with (build / (dirname + '.log')).open('w') as log:
            def run(command):
                subprocess.run(command, cwd=source, stdout=log,
                               stderr=subprocess.STDOUT, check=True)
            run(['./configure', '--prefix=' + str(prefix), '--disable-shared', '--enable-static'])
            run(['make', '-j' + str(args.jobs)])
            run(['make', 'install'])
            if dirname.startswith('libao'):
                functions = ['test', 'driver_info', 'device_init', 'set_option',
                             'open', 'play', 'close', 'device_clear']
                objects = []
                for driver in ('pulse', 'alsa', 'oss'):
                    wrapper = source / ('builtin_' + driver + '.c')
                    obj = wrapper.with_suffix('.o')
                    text = ''.join(f'#define ao_plugin_{f} ao_builtin_{driver}_{f}\n'
                                   for f in functions)
                    text += f'#include "src/plugins/{driver}/ao_{driver}.c"\n'
                    text += f'ao_functions ao_builtin_{driver} = {{' + ','.join(
                        f'ao_builtin_{driver}_{f}' for f in functions) + ',0};\n'
                    wrapper.write_text(text)
                    run(['cc', '-O2', '-DAO_BUILDING_LIBAO',
                         '-DPACKAGE_BUGREPORT="monty@xiph.org"',
                         '-DHAVE_SYS_SOUNDCARD_H=1', '-DBROKEN_OSS=1',
                         '-I' + str(source), '-I' + str(source / 'include'),
                         '-I' + str(source / 'include/ao'), '-c', str(wrapper), '-o', str(obj)])
                    objects.append(str(obj))
                run(['ar', 'rcs', str(prefix / 'lib/libao.a'), *objects])
                pc = prefix / 'lib/pkgconfig/ao.pc'
                pc.write_text(pc.read_text().replace('Libs: -L${libdir} -lao',
                    'Libs: -L${libdir} -lao -lpulse-simple -lpulse -lasound'))
        print('Built ' + dirname, flush=True)


if __name__ == '__main__':
    main()

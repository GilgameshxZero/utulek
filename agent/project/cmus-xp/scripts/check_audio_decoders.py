#!/usr/bin/env python3
"""Check the shared decoder code locally against generated, tagged audio files.

Requires host libraries from build_audio_deps.py --host, GCC, and the local
FFmpeg executable for fixture generation/reference PCM. Does not access Windows.
"""
import array
import argparse
import json
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parent.parent
FORMATS = [('opus', 'libopus'), ('mp3', 'libmp3lame'), ('flac', 'flac'),
           ('ogg', 'libvorbis'), ('m4a', 'aac'), ('alac.m4a', 'alac'),
           ('wav', 'pcm_s16le'), ('aiff', 'pcm_s16be'), ('wma', 'wmav2'),
           ('wv', 'wavpack'), ('mono.opus', 'libopus'),
           ('24bit.flac', 'flac'), ('44k.mp3', 'libmp3lame')]
FORMATS += [('tta', 'tta'), ('ac3', 'ac3'), ('eac3', 'eac3'),
            ('dts', 'dca'), ('truehd', 'truehd'), ('mlp', 'mlp'),
            ('sbc', 'sbc'), ('spx', 'libspeex'),
            ('ima.wav', 'adpcm_ima_wav'), ('ms.wav', 'adpcm_ms'),
            ('alaw.wav', 'pcm_alaw'), ('mulaw.wav', 'pcm_mulaw'),
            ('float.wav', 'pcm_f32le'), ('double.wav', 'pcm_f64le'),
            ('32bit.wav', 'pcm_s32le'), ('768k.wav', 'pcm_s16le'),
            ('surround.flac', 'flac')]
RAW_FORMATS = {'ac3', 'eac3', 'dts', 'truehd', 'mlp', 'sbc'}


def build_checker(args):
    binaries = ROOT / '.bin'
    binaries.mkdir(exist_ok=True)
    sources = ['tests/check_decoder.c', str(args.source), 'comment.c', 'keyval.c',
               'xmalloc.c', 'xstrjoin.c', 'uchar.c', 'gbuf.c', 'prog.c',
               'debug.c', 'convert.c', 'channelmap.c', 'u_collate.c']
    suffix = '_native' if args.native_channels else ''
    suffix += '_sanitized' if args.sanitize else ''
    checker = binaries / ('check_decoder' + suffix + ('.exe' if args.windows else ''))
    host = args.audio_libs or ROOT / ('.obj/deps/audio_windows' if args.windows else '.obj/deps/audio_host')
    compiler = str(ROOT / '.obj/deps/llvm_mingw/bin/x86_64-w64-mingw32-clang') if args.windows else 'gcc'
    flags = [] if args.native_channels else ['-DCMUS_FFMPEG_STEREO']
    if args.sanitize:
        flags += ['-g', '-fsanitize=address,undefined', '-fno-omit-frame-pointer']
    libraries = ['-lm', '-lpthread']
    if args.windows:
        flags += ['-DCMUS_NATIVE_WINDOWS']
        libraries += ['-lbcrypt', '-lole32', '-lsecur32', '-lws2_32']
    else:
        libraries += ['-latomic']
    subprocess.run([compiler, '-std=gnu11', '-O2', '-D_DEFAULT_SOURCE',
                    '-D_FILE_OFFSET_BITS=64', '-DHAVE_STRDUP', '-DDEBUG=1'] + flags + [
                    '-ffunction-sections', '-fdata-sections', '-I' + str(host / 'include'),
                    '-I' + str(ROOT), '-I' + str(ROOT / 'ip')] + [str(ROOT / s) for s in sources] +
                   ['-L' + str(host / 'lib'), '-Wl,--gc-sections', '-lavformat',
                    '-lavcodec', '-lswresample', '-lavutil'] + libraries +
                   ['-o', str(checker)], check=True)
    return checker


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--native-channels', action='store_true', help='Test Linux policy preserving surround channels')
    parser.add_argument('--sanitize', action='store_true')
    parser.add_argument('--windows', action='store_true', help='Cross-compile the Windows checker only')
    parser.add_argument('--build-only', action='store_true')
    parser.add_argument('--source', type=Path, default=ROOT / 'ip/ffmpeg.c')
    parser.add_argument('--audio-libs', type=Path)
    parser.add_argument('--output-dir', type=Path)
    args = parser.parse_args()
    out = args.output_dir or ROOT / ('.obj/decoder_checks_native' if args.native_channels else '.obj/decoder_checks')
    out.mkdir(parents=True, exist_ok=True)
    checker = build_checker(args)
    if args.build_only or args.windows:
        print(checker)
        return
    ffmpeg = ['ffmpeg', '-hide_banner', '-loglevel', 'error', '-y']
    reference_ffmpeg = [str(ROOT / '.obj/deps/audio_host/bin/ffmpeg'),
                        '-hide_banner', '-loglevel', 'error', '-y']
    results = []
    env = dict(os.environ, ASAN_OPTIONS='detect_leaks=0', UBSAN_OPTIONS='halt_on_error=1')
    # Containers with actual video, surround audio, and alternate/default tracks.
    containers = [
        ('video_ac3.mkv', 'ac3', True, False),
        ('video_dts.mkv', 'dca', True, False),
        ('video_truehd.mkv', 'truehd', True, False),
        ('default_track.mkv', 'flac', False, True),
        ('video_opus.webm', 'libopus', False, False),
        ('video_aac.mp4', 'aac', False, False),
        ('video_pcm.avi', 'pcm_s16le', False, False),
    ]
    fixtures = []
    for extension, encoder in FORMATS:
        path = out / ('café-音.' + extension)
        channels = '1' if extension in ('mono.opus', 'spx') else '6' if extension == 'surround.flac' else '2'
        rate = '44100' if extension == '44k.mp3' else '768000' if extension == '768k.wav' else '32000' if extension == 'spx' else '48000'
        extra = ['-sample_fmt', 's32'] if extension == '24bit.flac' else []
        if encoder in ('dca', 'truehd', 'mlp'): extra += ['-strict', '-2']
        if extension == 'truehd': extra += ['-f', 'truehd']
        subprocess.run(ffmpeg + ['-f', 'lavfi', '-i',
                        'sine=frequency=440:sample_rate=' + rate + ':duration=4', '-ac', channels,
                        '-c:a', encoder] + extra + ['-metadata', 'title=Decoder check',
                        '-metadata', 'artist=Test Artist', str(path)], check=True)
        fixtures.append((path, '0:a:0', extension not in RAW_FORMATS))
    for name, encoder, surround, alternate in containers:
        path = out / name
        video_codec = 'libvpx-vp9' if path.suffix == '.webm' else 'mpeg4'
        audio = ('aevalsrc=' + '|'.join('0.02*sin(2*PI*%d*t)' % (220 + 110*i) for i in range(6)) +
                 ':s=48000:d=4:c=5.1') if surround else 'sine=frequency=440:sample_rate=48000:duration=4'
        command = ffmpeg + ['-f', 'lavfi', '-i', 'testsrc2=size=64x64:rate=5:duration=4',
                            '-f', 'lavfi', '-i', audio]
        if alternate:
            command += ['-f', 'lavfi', '-i', 'sine=frequency=880:sample_rate=48000:duration=4']
        command += ['-map', '0:v', '-map', '1:a']
        if alternate: command += ['-map', '2:a']
        command += ['-c:v', video_codec, '-c:a', encoder, '-strict', '-2',
                    '-metadata', 'title=Decoder check']
        if alternate:
            command += ['-disposition:a:0', '0', '-disposition:a:1', 'default',
                        '-metadata:s:a:0', 'title=Wrong track',
                        '-metadata:s:a:1', 'title=Decoder check']
        subprocess.run(command + [str(path)], check=True)
        fixtures.append((path, '0:a:1' if alternate else '0:a:0', True))
    for path, audio_map, title_expected in fixtures:
        pcm = Path(str(path) + '.pcm')
        process = subprocess.run([str(checker), str(path), str(pcm)],
                                 capture_output=True, text=True, env=env, timeout=30)
        result = {'file': path.name, 'exit': process.returncode, 'passed': False}
        if process.returncode == 0:
            result.update(json.loads(process.stdout))
            reference = Path(str(path) + '.reference.pcm')
            pcm_type = 's' + str(result['bits']) + 'le'
            # Explicitly select the same decoder (not an external replacement,
            # e.g. libspeex with different delay/rate handling).
            subprocess.run(reference_ffmpeg + ['-c:a', result['codec'], '-i', str(path), '-map', audio_map, '-ac', str(result['channels']),
                            '-ar', str(result['rate']), '-f', pcm_type,
                            '-c:a', 'pcm_' + pcm_type, str(reference)], check=True)
            decoded, expected = pcm.read_bytes(), reference.read_bytes()
            result['length_ok'] = len(decoded) == len(expected)
            sample_type = 'h' if result['bits'] == 16 else 'i'
            a, b = array.array(sample_type, decoded), array.array(sample_type, expected)
            result['max_sample_delta'] = max((abs(x-y) for x, y in zip(a, b)), default=0)
            # Tolerate small floating-point differences between FFmpeg versions.
            result['pcm_ok'] = result['max_sample_delta'] / (1 << (result['bits']-1)) < 1e-5
            result['passed'] = (result['length_ok'] and result['pcm_ok'] and
                                (result['title_ok'] or not title_expected) and result['seek_ok'] and result['restart_ok'] and result['restart_pcm_ok'])
        else:
            result['stderr'] = process.stderr
            result['stdout'] = process.stdout
        results.append(result)
        print(path.name + ': ' + ('PASS' if result['passed'] else 'FAIL'), flush=True)
    malformed_inputs = {
        'malformed.opus': b'not an audio container',
        'truncated.opus': (out / 'café-音.opus').read_bytes()[:24],
        'empty.flac': b'',
        'oversized_fmt.wav': b'RIFF' + (32).to_bytes(4, 'little') + b'WAVEfmt ' + (0x7fffffff).to_bytes(4, 'little'),
    }
    video_only = out / 'video_only.mkv'
    subprocess.run(ffmpeg + ['-f', 'lavfi', '-i', 'color=size=16x16:duration=1',
                            '-an', '-c:v', 'mpeg4', str(video_only)], check=True)
    malformed_inputs['video_only.mkv'] = video_only.read_bytes()
    for name, contents in malformed_inputs.items():
        path = out / name; path.write_bytes(contents)
        process = subprocess.run([str(checker), str(path), str(out / 'malformed.pcm')],
                                 capture_output=True, text=True, env=env, timeout=30)
        results.append({'file': name, 'passed': process.returncode == 3,
                        'exit': process.returncode})
    report = 'results_native.json' if args.native_channels else 'results.json'
    if args.sanitize: report = report.replace('.json', '_sanitized.json')
    (out / report).write_text(json.dumps(results, indent=2, ensure_ascii=False) + '\n')
    if not all(r['passed'] for r in results):
        raise SystemExit('Decoder checks failed; see .obj/decoder_checks/results.json')


if __name__ == '__main__':
    main()

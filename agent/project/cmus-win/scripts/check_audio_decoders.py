#!/usr/bin/env python3
"""Check the shared decoder code locally against generated, tagged audio files.

Requires host libraries from build_audio_deps.py --host, GCC, and the local
FFmpeg executable for fixture generation/reference PCM. Does not access Windows.
"""
import array
import json
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parent.parent
OUT = ROOT / '.obj/decoder_checks'
HOST = ROOT / '.obj/deps/audio_host'
FORMATS = [('opus', 'libopus'), ('mp3', 'libmp3lame'), ('flac', 'flac'),
           ('ogg', 'libvorbis'), ('m4a', 'aac'), ('alac.m4a', 'alac'),
           ('wav', 'pcm_s16le'), ('aiff', 'pcm_s16be'), ('wma', 'wmav2'),
           ('wv', 'wavpack'), ('mono.opus', 'libopus'),
           ('24bit.flac', 'flac'), ('44k.mp3', 'libmp3lame')]


def main():
    OUT.mkdir(parents=True, exist_ok=True)
    binaries = ROOT / '.bin'
    binaries.mkdir(exist_ok=True)
    sources = ['windows/check_decoder.c', 'ip/ffmpeg.c', 'comment.c', 'keyval.c',
               'xmalloc.c', 'xstrjoin.c', 'uchar.c', 'gbuf.c', 'prog.c',
               'debug.c', 'convert.c', 'channelmap.c', 'u_collate.c']
    checker = binaries / 'check_decoder'
    subprocess.run(['gcc', '-std=gnu11', '-O2', '-D_DEFAULT_SOURCE',
                    '-D_FILE_OFFSET_BITS=64', '-DHAVE_STRDUP', '-DCMUS_FFMPEG_STEREO', '-DDEBUG=1',
                    '-ffunction-sections', '-fdata-sections', '-I' + str(HOST / 'include'),
                    '-I' + str(ROOT)] + [str(ROOT / s) for s in sources] +
                   ['-L' + str(HOST / 'lib'), '-Wl,--gc-sections', '-lavformat',
                    '-lavcodec', '-lswresample', '-lavutil', '-lm', '-latomic',
                    '-lpthread', '-o', str(checker)], check=True)
    ffmpeg = ['ffmpeg', '-hide_banner', '-loglevel', 'error', '-y']
    results = []
    for extension, encoder in FORMATS:
        path = OUT / ('café-音.' + extension)
        channels = '1' if extension == 'mono.opus' else '2'
        rate = '44100' if extension == '44k.mp3' else '48000'
        extra = ['-sample_fmt', 's32'] if extension == '24bit.flac' else []
        subprocess.run(ffmpeg + ['-f', 'lavfi', '-i',
                        'sine=frequency=440:sample_rate=' + rate + ':duration=4', '-ac', channels,
                        '-c:a', encoder] + extra + ['-metadata', 'title=Decoder check',
                        '-metadata', 'artist=Test Artist', str(path)], check=True)
        pcm = Path(str(path) + '.pcm')
        process = subprocess.run([str(checker), str(path), str(pcm)],
                                 capture_output=True, text=True)
        result = {'file': path.name, 'exit': process.returncode, 'passed': False}
        if process.returncode == 0:
            result.update(json.loads(process.stdout))
            reference = Path(str(path) + '.reference.pcm')
            pcm_type = 's' + str(result['bits']) + 'le'
            subprocess.run(ffmpeg + ['-i', str(path), '-map', '0:a:0', '-ac', str(result['channels']), '-f', pcm_type,
                            '-c:a', 'pcm_' + pcm_type, str(reference)], check=True)
            decoded, expected = pcm.read_bytes(), reference.read_bytes()
            result['length_ok'] = len(decoded) == len(expected)
            sample_type = 'h' if result['bits'] == 16 else 'i'
            a, b = array.array(sample_type, decoded), array.array(sample_type, expected)
            result['max_sample_delta'] = max((abs(x-y) for x, y in zip(a, b)), default=0)
            # Tolerate small floating-point differences between FFmpeg versions.
            result['pcm_ok'] = result['max_sample_delta'] / (1 << (result['bits']-1)) < 1e-5
            result['passed'] = (result['length_ok'] and result['pcm_ok'] and
                                result['title_ok'] and result['seek_ok'] and result['restart_ok'])
        else:
            result['stderr'] = process.stderr
        results.append(result)
        print(path.name + ': ' + ('PASS' if result['passed'] else 'FAIL'), flush=True)
    malformed_inputs = {
        'malformed.opus': b'not an audio container',
        'truncated.opus': (OUT / 'café-音.opus').read_bytes()[:24],
        'empty.flac': b'',
        'oversized_fmt.wav': b'RIFF' + (32).to_bytes(4, 'little') + b'WAVEfmt ' + (0x7fffffff).to_bytes(4, 'little'),
    }
    for name, contents in malformed_inputs.items():
        path = OUT / name; path.write_bytes(contents)
        process = subprocess.run([str(checker), str(path), str(OUT / 'malformed.pcm')],
                                 capture_output=True, text=True)
        results.append({'file': name, 'passed': process.returncode == 3,
                        'exit': process.returncode})
    (OUT / 'results.json').write_text(json.dumps(results, indent=2, ensure_ascii=False) + '\n')
    if not all(r['passed'] for r in results):
        raise SystemExit('Decoder checks failed; see .obj/decoder_checks/results.json')


if __name__ == '__main__':
    main()

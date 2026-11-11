#!/usr/bin/env python3
"""Check the real CUE/input layer with exact PCM and sanitizers."""
import argparse
import json
import os
import subprocess
import threading
import uuid
import wave
from pathlib import Path
r = Path(__file__).resolve().parent.parent
parser = argparse.ArgumentParser(description='Validate real CUE/input decoding with sanitizers, exact PCM and malformed inputs.')
parser.add_argument('--build-only', action='store_true')
args = parser.parse_args()
out = r / '.obj/cue' / ('decoder_' + uuid.uuid4().hex[:10])
out.mkdir(parents=True)
base = ['gcc', '-std=gnu11', '-O1', '-g', '-fsanitize=address,undefined', '-fno-omit-frame-pointer', '-ffunction-sections', '-fdata-sections', '-D_DEFAULT_SOURCE', '-D_FILE_OFFSET_BITS=64', '-DHAVE_STRDUP', '-DCMUS_EMBEDDED', '-DVERSION="cue-check"', '-I' + str(r), '-I' + str(r / '.obj/build'), '-I' + str(r / '.obj/deps/audio_host/include')]
objects = []
for source in ['tests/check_cue_decoder.c', 'input.c', 'cue.c', 'cue_utils.c', 'ip/cue.c', 'ip/wav.c', 'ip/ffmpeg.c', 'pcm.c', 'file.c', 'path.c', 'comment.c', 'keyval.c', 'xmalloc.c', 'xstrjoin.c', 'uchar.c', 'gbuf.c', 'prog.c', 'debug.c', 'convert.c', 'channelmap.c', 'u_collate.c', 'mergesort.c', 'locking.c', 'misc.c']:
    p = Path(source)
    obj = out / (source.replace('/', '_') + '.o')
    flags = []
    if p.parent.name == 'ip':
        flags = ['-D' + s + '=' + p.stem + '_' + s for s in ['ip_ops', 'ip_priority', 'ip_extensions', 'ip_mime_types', 'ip_options', 'ip_abi_version']]
    subprocess.run(base + flags + ['-c', str(r / p), '-o', str(obj)], check=True)
    objects.append(str(obj))
subprocess.run(['gcc', '-fsanitize=address,undefined', '-Wl,--gc-sections', *objects, '-L' + str(r / '.obj/deps/audio_host/lib'), '-lavformat', '-lavcodec', '-lswresample', '-lavutil', '-lm', '-lpthread', '-latomic', '-o', str(out / 'check_cue')], check=True)
if args.build_only:
    print(out / 'check_cue')
    raise SystemExit
results = []
env = dict(os.environ, ASAN_OPTIONS='detect_leaks=0', UBSAN_OPTIONS='halt_on_error=1')

def run(name, cue, track, expected=None):
    pcm = out / (name + '.pcm')
    process = subprocess.run([str(out / 'check_cue'), 'cue://' + str(cue) + '/' + str(track), str(pcm)], capture_output=True, text=True, timeout=20, env=env)
    metadata = json.loads(process.stdout) if process.stdout.strip() else None
    if expected is None:
        passed = process.returncode == 3
    else:
        passed = process.returncode == 0 and metadata['restart'] and metadata['seek'] and metadata['invalid_seek'] and metadata['title_present'] and (pcm.read_bytes() == expected)
    row = {'case': name, 'passed': passed, 'exit': process.returncode, 'metadata': metadata}
    if not passed:
        row.update(stderr=process.stderr, stdout=process.stdout)
    results.append(row)
    print(name + ': ' + ('PASS' if passed else 'FAIL'), flush=True)
for bits, channels, rate in [(8, 1, 8000), (8, 2, 8000), (16, 1, 44100), (16, 2, 44100)]:
    name = f'{bits}bit_{channels}ch'
    audio = out / (name + ' ü.wav')
    raw = bytearray()
    reference = bytearray()
    for frame in range(rate * 4):
        value = frame * 17 % 251
        if bits == 8:
            raw.extend(bytes([value]) * channels)
            reference.extend(int((value - 128) * 256).to_bytes(2, 'little', signed=True) * 2)
        else:
            value = value * 100 - 12000
            raw.extend(value.to_bytes(2, 'little', signed=True) * channels)
            reference.extend(value.to_bytes(2, 'little', signed=True) * 2)
    with wave.open(str(audio), 'wb') as f:
        f.setparams((channels, bits // 8, rate, 0, 'NONE', 'not compressed'))
        f.writeframes(raw)
    for absolute in (False, True):
        cue = out / (name + ('_absolute' if absolute else '') + '.cue')
        cue.write_text('\ufeffTITLE "Album"\nPERFORMER "Artist"\nFILE "' + str(audio if absolute else audio.name) + '" WAVE\nTRACK 01 AUDIO\nTITLE "First"\nINDEX 01 00:00:00\nTRACK 02 AUDIO\nTITLE "Second"\nINDEX 01 00:02:00\n')
        for track in (1, 2):
            run(name + ('_absolute' if absolute else '') + '_' + str(track), cue, track, bytes(reference[(track - 1) * rate * 8:track * rate * 8]))
    compressed = out / (name + '.flac')
    subprocess.run(['ffmpeg', '-v', 'error', '-y', '-i', str(audio), str(compressed)], check=True)
    cue = out / (name + '_flac.cue')
    cue.write_text('FILE "' + compressed.name + '" FLAC\nTRACK 01 AUDIO\nTITLE "First"\nINDEX 01 00:00:00\nTRACK 02 AUDIO\nTITLE "Second"\nINDEX 01 00:02:00\n')
    for track in (1, 2):
        run(name + '_flac_' + str(track), cue, track, bytes(reference[(track - 1) * rate * 8:track * rate * 8]))
    for extension, source in [('wav', audio), ('flac', compressed)]:
        cue = out / (name + '_fraction_' + extension + '.cue')
        cue.write_text('FILE "' + source.name + '" WAVE\nTRACK 03 AUDIO\nTITLE "First"\nINDEX 01 00:00:01\nTRACK 07 AUDIO\nTITLE "Second"\nINDEX 00 00:01:01\nINDEX 01 00:02:01\n')
        start = int(rate / 75 + .5)
        run(name + '_fraction_pregap_' + extension, cue, 3, bytes(reference[start * 4:(start + rate) * 4]))
        cue = out / (name + '_short_fraction_' + extension + '.cue')
        cue.write_text('FILE "' + source.name + '" WAVE\nTRACK 03 AUDIO\nTITLE "First"\nINDEX 01 00:00:01\nTRACK 07 AUDIO\nTITLE "Second"\nINDEX 01 00:00:02\n')
        end = int(rate * 2 / 75 + .5)
        run(name + '_short_fraction_' + extension, cue, 3, bytes(reference[start * 4:end * 4]))
        cue = out / (name + '_multi_' + extension + '.cue')
        cue.write_text('FILE "' + source.name + '" WAVE\nTRACK 01 AUDIO\nTITLE "First"\nINDEX 01 00:01:00\nFILE "' + source.name + '" WAVE\nTRACK 05 AUDIO\nTITLE "Second"\nINDEX 01 00:02:00\n')
        run(name + '_multiple_files_' + extension, cue, 1, bytes(reference[rate * 4:]))
        run(name + '_multiple_files_last_' + extension, cue, 5, bytes(reference[rate * 8:]))
valid = 'FILE "16bit_2ch ü.wav" WAVE\nTRACK 01 AUDIO\nTITLE "First"\nINDEX 01 00:00:00\n'
invalids = {'missing_file': valid.replace('16bit_2ch ü.wav', 'missing.wav'), 'empty': '', 'missing_file_command': 'TRACK 01 AUDIO\nINDEX 01 00:00:00\n', 'missing_index': valid.replace('INDEX 01 00:00:00', ''), 'overflow_track': valid.replace('TRACK 01', 'TRACK 4294967297'), 'zero_track': valid.replace('TRACK 01', 'TRACK 00'), 'overflow_timestamp': valid.replace('00:00:00', '999999999999999:00:00'), 'bad_seconds': valid.replace('00:00:00', '00:60:00'), 'bad_frames': valid.replace('00:00:00', '00:00:75'), 'bad_colons': valid.replace('00:00:00', '00:00:00:00'), 'unterminated_quote': valid.replace('"First"', '"First'), 'backwards_index': valid.replace('INDEX 01 00:00:00', 'INDEX 00 00:01:00\nINDEX 01 00:00:00')}
for name, text in invalids.items():
    cue = out / (name + '.cue')
    cue.write_text(text)
    run(name, cue, 1)
for track in (0, -1, 2147483648, 999):
    run('invalid_url_track_' + str(track), out / '16bit_2ch.cue', track)
changing = out / 'concurrent_truncate.cue'
stop = threading.Event()


def truncate_source():
    while not stop.is_set():
        changing.write_text(valid)
        changing.write_bytes(b'')


writer = threading.Thread(target=truncate_source)
writer.start()
safe = True
try:
    for attempt in range(30):
        process = subprocess.run([str(out / 'check_cue'), 'cue://' + str(changing) + '/1',
            str(out / 'concurrent.pcm')], capture_output=True, text=True, timeout=20, env=env)
        if process.returncode not in (0, 3):
            (out / 'concurrent_failure.log').write_text(process.stdout + process.stderr)
            safe = False
            break
finally:
    stop.set()
    writer.join()
results.append({'case': 'concurrent_cue_truncation_30_runs', 'passed': safe})
(out / 'results.json').write_text(json.dumps({'passed': all((x['passed'] for x in results)), 'checks': results}, indent=2) + '\n')
print(out / 'results.json')
if not all((x['passed'] for x in results)):
    raise SystemExit(1)

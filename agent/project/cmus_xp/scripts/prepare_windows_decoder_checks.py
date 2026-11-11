#!/usr/bin/env python3
"""Package generated fixtures and pinned PCM references for Windows validation.

Requires a completed native-channel check_audio_decoders.py run. Windows output
policy expands mono and downmixes surround to stereo; references follow that
policy, including the selected default stream. No uploads are performed.
"""
import argparse
import json
from pathlib import Path
import subprocess
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parent.parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--fixtures', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--checker', type=Path)
    parser.add_argument('--native-channels', action='store_true', help='Preserve native channel counts instead of the historical stereo policy')
    parser.add_argument('--ffmpeg', type=Path, default=ROOT / '.obj/deps/audio_host/bin/ffmpeg',
                        help='Runnable reference FFmpeg from the pinned release')
    args = parser.parse_args()
    checker = args.checker or ROOT / ('.bin/check_decoder_native.exe' if args.native_channels else '.bin/check_decoder.exe')
    rows = json.loads((args.fixtures / 'results_native_sanitized.json').read_text())
    if not all(row['passed'] for row in rows):
        raise SystemExit('Native decoder reference suite must pass first')
    args.output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(dir=args.output.parent) as temporary:
        cases = []
        staged = Path(temporary) / 'decoder_checks.zip'
        with zipfile.ZipFile(staged, 'w', zipfile.ZIP_DEFLATED) as archive:
            archive.write(checker, 'check_decoder.exe')
            archive.write(ROOT / 'windows/check_decoders.ps1', 'check_decoders.ps1')
            for i, row in enumerate(rows):
                source = args.fixtures / row['file']
                case = {'file': row['file'], 'input': 'case_%03d' % i + source.suffix,
                        'expected_exit': row['exit'], 'title_expected': bool(row.get('title_ok'))}
                archive.write(source, case['input'])
                if row['exit'] == 0:
                    reference = Path(str(source) + '.reference.pcm')
                    case['output_channels_expected'] = row['channels'] if args.native_channels else 2
                    if not args.native_channels and row['channels'] != 2:
                        reference = Path(temporary) / ('reference_%03d.pcm' % i)
                        pcm_type = 's' + str(row['bits']) + 'le'
                        audio_map = '0:a:1' if row['file'] == 'default_track.mkv' else '0:a:0'
                        subprocess.run([str(args.ffmpeg.resolve()),
                            '-v', 'error', '-y', '-c:a', row['codec'], '-i', str(source),
                            '-map', audio_map, '-ac', '2', '-ar', str(row['rate']),
                            '-f', pcm_type, '-c:a', 'pcm_' + pcm_type, str(reference)], check=True)
                    case['reference'] = 'reference_%03d.pcm' % i
                    archive.write(reference, case['reference'])
                cases.append(case)
            archive.writestr('cases.json', json.dumps(cases))
        staged.replace(args.output)
    print(args.output)


if __name__ == '__main__':
    main()

#!/usr/bin/env python3
"""Verify real application saves preserve existing state when writes fail."""
import argparse
import hashlib
import json
from pathlib import Path
import time
import uuid
import wave

from linux_test_session import Session

ROOT = Path(__file__).resolve().parent.parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', type=Path, required=True)
    parser.add_argument('--remote', type=Path, required=True)
    parser.add_argument('--long-paths', action='store_true')
    args = parser.parse_args()
    out = ROOT / '.obj/milestones/persistence' / uuid.uuid4().hex[:12]
    config = out / 'config 音'
    if args.long_paths:
        config = config / ('音' * 70) / ('楽' * 70)
    config.mkdir(parents=True)
    (config / 'rc').write_text('set mpris=false\nset softvol=true\nset resume=true\n'
                             'set output_plugin=alsa\nset dsp.alsa.device=null\n')
    track = out / 'tone ü.wav'
    with wave.open(str(track), 'wb') as f:
        f.setnchannels(2); f.setsampwidth(2); f.setframerate(8000)
        f.writeframes(bytes(8000 * 4 * 30))
    source = out / 'source 音.m3u'
    source.write_text(str(track) + '\n')
    second = out / 'new_source.m3u'
    second.write_text(str(track) + '\n')
    screens = []
    checks = []
    session = Session(args.binary, args.remote, config)
    try:
        session.start()
        session.command('add ' + str(track), 'pl-import ' + str(source), 'set buffer_seconds=8')
        session.keys(':set repeat=true\r')
        session.keys('/tone\r')
        time.sleep(.2)
        session.stop(); screens.append(bytes(session.screen))
        names = ('autosave', 'resume', 'command-history', 'search-history', 'playlist_sources', 'lib.pl', 'cache')
        before = {name: (config / name).read_bytes() for name in names}
        checks.append('successful_unicode_save')
        session = Session(args.binary, args.remote, config).start()
        session.command('set buffer_seconds=12', 'pl-import ' + str(second), 'view 3')
        session.keys(':set repeat=false\r')
        session.keys('/different\r')
        session.limit_file_size(1)
        session.stop(); screens.append(bytes(session.screen))
        changed = [name for name in names if (config / name).read_bytes() != before[name]]
        report = {'changed_after_failed_save': changed,
                  'original_sha256': {n: hashlib.sha256(v).hexdigest() for n, v in before.items()},
                  'checks': checks}
        (out / 'results.json').write_text(json.dumps(report, indent=2) + '\n')
        assert not changed, 'Failed writes replaced existing files: ' + ', '.join(changed)
        assert not list(config.rglob('*.tmp.*')), 'Failed-save temporary files remain'
        checks.append('failed_writes_preserve_all_previous_state')
        session = Session(args.binary, args.remote, config).start()
        session.command('save -l ' + str(out / 'restarted.pl'))
        session.stop(); screens.append(bytes(session.screen))
        assert (out / 'restarted.pl').read_text().strip() == str(track)
        assert 'set buffer_seconds=8\n' in (config / 'autosave').read_text()
        checks.append('restart_after_failed_save')
        cache = config / 'cache'
        cache.write_bytes(cache.read_bytes()[:17])
        session = Session(args.binary, args.remote, config).start()
        session.command('save -l ' + str(out / 'recovered.pl'))
        session.stop(); screens.append(bytes(session.screen))
        assert (out / 'recovered.pl').read_text().strip() == str(track)
        checks.append('truncated_cache_recovery')
        report.update(checks=checks, passed=True)
        (out / 'results.json').write_text(json.dumps(report, indent=2) + '\n')
        print(json.dumps({'results': str(out / 'results.json'), 'checks': checks}))
    finally:
        if hasattr(session, 'process') and session.process.poll() is None:
            session.stop()
        (out / 'screen.bin').write_bytes(b'\n'.join(screens) + bytes(session.screen))


if __name__ == '__main__':
    main()

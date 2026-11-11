#!/usr/bin/env python3
"""Inventory the port against supplied pinned upstream source; do not run players.

The resulting JSON distinguishes shared source declarations from verified user
behavior. It is an audit baseline, not a claim of completed feature parity.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parent.parent


def commands(root):
    text = (root / 'command_mode.c').read_text()
    table = text.split('struct command commands[] = {', 1)[1].split('\n};', 1)[0]
    return {name: body.strip() for name, body in re.findall(r'\{\s*"([^"]+)"\s*,([^\n]+)\}', table)}


def bindings(root):
    return [line for line in (root / 'data/rc').read_text().splitlines() if line.startswith('bind ')]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--upstream', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    upstream = commands(args.upstream)
    current = commands(ROOT)
    differences = []
    for path in sorted(args.upstream.rglob('*')):
        if not path.is_file() or path.suffix not in ('.c', '.h'):
            continue
        relative = path.relative_to(args.upstream)
        local = ROOT / relative
        if not local.is_file() or local.read_bytes() != path.read_bytes():
            differences.append({'file': str(relative), 'upstream_sha256': hashlib.sha256(path.read_bytes()).hexdigest(),
                                'port_sha256': hashlib.sha256(local.read_bytes()).hexdigest() if local.is_file() else None})
    conditionals = []
    for path in sorted(ROOT.glob('*.c')):
        for number, line in enumerate(path.read_text().splitlines(), 1):
            if 'CMUS_NATIVE_WINDOWS' in line:
                conditionals.append({'file': path.name, 'line': number, 'guard': line.strip()})
    features = [
        ('shared_commands', 6, 'shared_declarations', 'Typed commands, bindings, config and remote commands use command_mode.c'),
        ('remote_control', 7, 'missing', 'server services are stubs; Windows --listen is rejected; no Windows remote executable'),
        ('streaming', 8, 'missing', 'Windows ip_open rejects remote input; HTTP services are stubs'),
        ('cue', 9, 'missing', 'Shared CUE parser exists; Windows input registry omits cue plugin'),
        ('multichannel', 10, 'missing', 'WaveOut rejects more than two channels; FFmpeg forces stereo'),
        ('legacy_tags', 11, 'missing', 'Windows build omits iconv and convert falls back to copying bytes'),
        ('external_commands_hooks', 11, 'partial', 'Shared commands use a basic _spawnvp implementation'),
        ('tracker_modules', 11, 'missing', 'Linux modplug plugin is absent from the Windows registry'),
        ('audio_cd', 11, 'missing', 'Linux cdio/CDDB/discid path has no Windows input plugin'),
        ('desktop_controls', 11, 'missing', 'Linux MPRIS is present; no native Windows media-session equivalent'),
        ('platform_backends', 11, 'platform_equivalents_required', 'ALSA/Pulse/JACK-specific options need native equivalents where meaningful'),
        ('terminal_mouse_unicode', 12, 'partial', 'Direct Windows Unicode tested; populated tmux redraw remains unresolved'),
        ('bracketed_paste', 12, 'missing', 'Windows build excludes bracketed paste enable/disable'),
        ('daily_use_reliability', 13, 'partial', 'Checkpoints 01-05 pass; endurance/hardware transitions remain'),
        ('release_source_provenance', 14, 'partial', 'Review kit exists; complete corresponding-source assembly remains')]
    if 'win_remote_listen' in (ROOT / 'server.c').read_text():
        features[1] = ('remote_control', 7, 'implemented_requires_behavioral_evidence',
                       'Shared server/CLI have native local and TCP transports; assess actual checkpoint test reports')
    report = {'audit_complete': True, 'parity_complete': False,
        'upstream_revision': '521444c83beda6f13feb2eebcd27d12f3f23d70d',
        'commands': {'upstream': upstream, 'port': current,
            'missing': sorted(upstream.keys() - current.keys()), 'added': sorted(current.keys() - upstream.keys()),
            'changed_declarations': sorted(name for name in upstream.keys() & current.keys() if upstream[name] != current[name])},
        'bindings': {'upstream': bindings(args.upstream), 'port': bindings(ROOT)},
        'option_registration_lines': [line.strip() for line in (ROOT / 'options.c').read_text().splitlines() if 'option_add' in line],
        'upstream_source_differences': differences, 'windows_conditionals': conditionals,
        'features': [{'feature': name, 'checkpoint': checkpoint, 'status': status, 'basis': basis}
                     for name, checkpoint, status, basis in features],
        'entry_points': ['typed command', 'default and configured key binding', 'browser/playlist action',
                         'config and status hook', 'remote raw/cooked/stdin command', 'desktop media controls'],
        'validation_policy': 'Source inventory is not behavioral proof; implementation checkpoints must exercise real entry points on both platforms.'}
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps({'audit': str(args.output), 'upstream_commands': len(upstream), 'port_commands': len(current),
                      'missing_commands': report['commands']['missing'], 'feature_groups': len(features)}))


if __name__ == '__main__':
    main()

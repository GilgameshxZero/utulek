#!/usr/bin/env python3
"""Check calendar reply wire properties through the shipped helper CLI.

Owned synthetic invitations only. No network or messages are sent.
"""
import argparse
import email
import shlex
import json
import re
import subprocess
import sys
from pathlib import Path


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('helper', type=Path)
    parser.add_argument('root', type=Path)
    args = parser.parse_args()
    args.root.mkdir(parents=True, exist_ok=True)
    capture = args.root / 'capture_message.py'
    capture.write_text("import sys\nfrom pathlib import Path\nPath(sys.argv[1]).write_bytes(sys.stdin.buffer.read())\n")
    results = []
    for scenario, extra in (
        ('initial', 'SEQUENCE:0\r\n'),
        ('updated', 'SEQUENCE:7\r\n'),
        ('recurring_instance', 'SEQUENCE:3\r\nRECURRENCE-ID:20300501T100000Z\r\n'),
    ):
        invitation = (
            'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Owned test//EN\r\n'
            'METHOD:REQUEST\r\nBEGIN:VEVENT\r\nUID:protocol-test@example.invalid\r\n'
            'DTSTAMP:20300401T000000Z\r\nDTSTART:20300501T100000Z\r\n'
            'DTEND:20300501T110000Z\r\nSUMMARY:Owned calendar protocol test\r\n'
            'ORGANIZER:mailto:organizer@example.invalid\r\n'
            'ATTENDEE;RSVP=TRUE;PARTSTAT=NEEDS-ACTION:mailto:reader@example.invalid\r\n'
            + extra + 'END:VEVENT\r\nEND:VCALENDAR\r\n'
        )
        source = args.root / (scenario + '.ics')
        source.write_bytes(invitation.encode())
        for option, state in (('-a', 'ACCEPTED'), ('-d', 'DECLINED'), ('-t', 'TENTATIVE')):
            reply = args.root / (scenario + '-' + state.lower() + '.ics')
            process = subprocess.run(
                [sys.executable, str(args.helper.resolve()), '-e', 'reader@example.invalid',
                 option, '-g', str(reply.resolve()), str(source.resolve())],
                capture_output=True, text=True,
            )
            payload = reply.read_text() if reply.is_file() else ''
            payload = re.sub(r'\n[ \t]', '', payload)
            lines = payload.splitlines()
            checks = {
                'helper_success': process.returncode == 0,
                'method_reply': 'METHOD:REPLY' in lines,
                'uid_preserved': 'UID:protocol-test@example.invalid' in lines,
                'organizer_preserved': 'ORGANIZER:mailto:organizer@example.invalid' in lines,
                'one_responding_attendee': len([s for s in lines if s.startswith('ATTENDEE')]) == 1,
                'correct_partstat': any(s.startswith('ATTENDEE;') and
                    'PARTSTAT=' + state in s and s.endswith(':mailto:reader@example.invalid') for s in lines),
                'utc_dtstamp': any(re.fullmatch(r'DTSTAMP:\d{8}T\d{6}Z', s) for s in lines),
                'revision_preserved': scenario == 'initial' or
                    ('SEQUENCE:7' if scenario == 'updated' else 'SEQUENCE:3') in lines,
                'instance_preserved': scenario != 'recurring_instance' or
                    'RECURRENCE-ID:20300501T100000Z' in lines,
            }
            wire = args.root / (scenario + '-' + state.lower() + '.eml')
            command = shlex.join([sys.executable, str(capture.resolve()), str(wire.resolve())])
            sent = subprocess.run(
                [sys.executable, str(args.helper.resolve()), '-e', 'reader@example.invalid',
                 option, '-s', command, str(source.resolve())], capture_output=True, text=True)
            message = email.message_from_bytes(wire.read_bytes()) if wire.exists() else None
            checks['send_helper_success'] = sent.returncode == 0
            checks['single_calendar_message'] = bool(message and not message.is_multipart() and
                message.get_content_type() == 'text/calendar' and message.get_param('method') == 'REPLY')
            checks['wire_recipient'] = bool(message and email.utils.parseaddr(message['To'])[1] ==
                'organizer@example.invalid')
            results.append({'scenario': scenario, 'state': state, 'checks': checks,
                            'passed': all(checks.values()), 'error': process.stderr})
    report = {'passed': all(r['passed'] for r in results), 'cases': results}
    (args.root / 'results.json').write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps({'passed': report['passed'], 'cases': len(results),
                      'failures': [{ 'scenario': r['scenario'], 'state': r['state'],
                          'checks': [k for k, v in r['checks'].items() if not v]}
                          for r in results if not r['passed']]}))
    return 0 if report['passed'] else 1


if __name__ == '__main__':
    raise SystemExit(main())

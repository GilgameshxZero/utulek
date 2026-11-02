#!/usr/bin/env python3
"""Differential mutation checks on disposable books; Linux PTY for confirmations."""
import argparse
import json
import os
from pathlib import Path
import pty
import subprocess
import sys
import tempfile
import termios

FIXTURE = 'BEGIN:VCARD\r\nVERSION:3.0\r\nUID:source-id\r\nN:Example;Alice;;;\r\nFN:Alice Example\r\nEMAIL;TYPE=work:alice@example.invalid\r\nTEL:+15550100001\r\nNOTE:before\r\nX-KEEP;X-PARAM=one:opaque\r\nEND:VCARD\r\n'


def invoke(exe, args, answers):
    master, slave = pty.openpty()
    settings = termios.tcgetattr(slave)
    settings[3] &= ~termios.ECHO
    termios.tcsetattr(slave, termios.TCSANOW, settings)
    try:
        process = subprocess.Popen([exe, *args], stdin=slave, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        os.close(slave)
        slave = None
        if answers:
            os.write(master, answers.encode())
        try:
            out, err = process.communicate(timeout=15)
        except subprocess.TimeoutExpired:
            process.kill()
            out, err = process.communicate()
            raise RuntimeError(f'timed out: {args}: {out[-2000:]} {err[-1000:]}')
        return {'exit_code': process.returncode, 'stdout': out, 'stderr': err}
    finally:
        os.close(master)
        if slave is not None:
            os.close(slave)


def snapshot(root):
    result = {}
    for book in ['source', 'destination']:
        cards = []
        for path in (root / book).glob('*.vcf'):
            text = path.read_text(encoding='utf-8').replace('\n ', '')
            lines = text.splitlines()
            uid = next(line[4:] for line in lines if line.startswith('UID:'))
            # Generated identifiers and revision timestamps intentionally vary.
            identity = uid if uid in ['source-id', 'target-id'] else '<generated>'
            properties = sorted(line.replace('X-ABLabel:', 'X-ABLABEL:') for line in lines if not line.startswith(('UID:', 'REV:', 'PRODID:')))
            cards.append({'uid': identity, 'properties': properties})
        result[book] = sorted(cards, key=lambda c: (c['uid'], c['properties']))
    return result


def run_case(exe, label, candidate):
    with tempfile.TemporaryDirectory(prefix='khard operations ') as temporary:
        root = Path(temporary)
        for name in ['source', 'destination']:
            (root / name).mkdir()
        (root / 'source/source.vcf').write_bytes(FIXTURE.encode())
        if label.startswith(('merge', 'duplicate')):
            target = FIXTURE.replace('UID:source-id', 'UID:target-id')
            if label.startswith('merge'):
                target = target.replace('Alice', 'Bob')
            else:
                target = target.replace('NOTE:before', 'NOTE:old target')
            (root / 'destination/target.vcf').write_bytes(target.encode())
        helper = root / 'editor.py'
        helper.write_text('import pathlib,sys\np=pathlib.Path(sys.argv[-1])\np.write_text(p.read_text().replace("before", "merged"))\n' if label != 'merge unchanged' else 'pass\n')
        config = root / 'khard.conf'
        config.write_text(f"[addressbooks]\n[[source]]\npath = '{root / 'source'}'\n[[destination]]\npath = '{root / 'destination'}'\n[general]\neditor = {sys.executable}, '{helper}'\nmerge_editor = {sys.executable}, '{helper}'\n[contact table]\ndisplay = formatted_name\nsort = formatted_name\n[vcard]\nsearch_in_source_files = no\n")
        answers = ''
        if label in ['copy', 'move']:
            args = [label, '-a', 'source', '-A', 'destination', 'uid:source-id']
        elif label.startswith('duplicate'):
            action = label.split()[-1]
            args = ['copy', '-a', 'source', '-A', 'destination', 'uid:source-id']
            answers = '1\n' + action + '\n'
        elif label.startswith('merge'):
            args = ['merge', '-a', 'source', '-A', 'destination', '-t', 'uid:target-id', 'uid:source-id']
            answers = 'n\n' if label == 'merge cancelled' else 'y\n'
        else:
            mail = root / 'mail.eml'
            address = 'alice@example.invalid' if label == 'email skip' else 'new@example.invalid'
            name = 'New Person' if label == 'email create' else 'Alice Example'
            mail.write_text(f'From: {name} <{address}>\n')
            args = ['add-email', '-a', 'source', '-i', str(mail)]
            if label == 'email skip':
                args.append('--skip-already-added')
            else:
                answers = 'y\ncreate\n1\n\n\n\ny\nwork\n' if label == 'email create' else ('y\nyes\nTeam\n' if label == 'email custom' else 'y\nyes\nwork\n')
        result = invoke(exe, ['-c', str(config), *args], answers)
        result['snapshot'] = snapshot(root)
        return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--reference', default='khard')
    parser.add_argument('--candidate', default='.bin/khard')
    parser.add_argument('--report', default='.obj/operation_compatibility.json')
    args = parser.parse_args()
    cases = []
    for label in ['copy', 'move', 'merge saved', 'merge unchanged', 'merge cancelled', 'email add', 'email skip', 'email create', 'email custom', 'duplicate copy', 'duplicate overwrite']:
        reference = run_case(args.reference, label, False)
        candidate = run_case(args.candidate, label, True)
        # Cancellation is an explicit nonzero outcome in khard-xp.
        expected_status = 1 if label == 'merge cancelled' else reference['exit_code']
        # Upstream infers voice on rewritten untyped phones; khard-xp preserves them.
        if label == 'merge saved':
            for card in reference['snapshot']['destination']:
                card['properties'] = sorted(p.replace('TEL;TYPE=voice:', 'TEL:') for p in card['properties'] if p not in ['X-KIND:individual','KIND:individual'])
        passed = reference['exit_code'] == 0 and candidate['exit_code'] == expected_status and reference['snapshot'] == candidate['snapshot']
        cases.append({'case': label, 'passed': passed, 'expected_difference': 'upstream infers voice and serializes default individual kind on rewritten merges' if label == 'merge saved' else ('cancellation returns nonzero' if label == 'merge cancelled' else None), 'reference': reference, 'candidate': candidate})
    report = {'reference_version': subprocess.run([args.reference, '--version'], capture_output=True, text=True, check=True).stdout.strip(), 'synthetic_only': True, 'passed': all(c['passed'] for c in cases), 'cases': cases}
    Path(args.report).write_text(json.dumps(report, indent=2, ensure_ascii=False) + '\n')
    for case in cases:
        if not case['passed']:
            print(json.dumps(case, ensure_ascii=False))
    print(f"{sum(c['passed'] for c in cases)}/{len(cases)} operation cases passed; report: {args.report}")
    raise SystemExit(0 if report['passed'] else 1)


if __name__ == '__main__':
    main()

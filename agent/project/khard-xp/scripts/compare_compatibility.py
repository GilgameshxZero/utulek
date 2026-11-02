#!/usr/bin/env python3
"""Compare upstream and khard-xp using disposable, synthetic contacts only."""
import argparse
import json
from pathlib import Path
import subprocess
import tempfile


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--reference', default='khard')
    parser.add_argument('--candidate', default='.bin/khard')
    parser.add_argument('--report', default='.obj/upstream_compatibility.json')
    args = parser.parse_args()
    cases = []
    with tempfile.TemporaryDirectory(prefix='khard compatibility ') as root:
        root = Path(root)
        book = root / 'contacts'
        book.mkdir()
        config = root / 'khard.conf'
        config.write_text(f"[addressbooks]\n[[test]]\npath = '{book}'\n[general]\neditor = vim\n[contact table]\ndisplay = formatted_name\nsort = formatted_name\n[vcard]\nsearch_in_source_files = no\n", encoding='utf-8')
        fixture = "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:example-one\r\nN:Zoë;Alice;Marie;Dr.;Jr.\r\nFN:Alice Zoë\r\nNICKNAME:Ally\r\nTEL;TYPE=home:+49 (30) 123-4567\r\nTEL;TYPE=work:030 765-4321\r\nEMAIL;TYPE=work:alice@example.invalid\r\nADR;TYPE=home:;;12 Test Street;Test City;;10000;Test Country\r\nORG:Example;Engineering\r\nTITLE:Engineer\r\nROLE:Developer\r\nNOTE:Old\\nsecond line\r\nCATEGORIES:Friends,Test\\,One\r\nURL:https://example.invalid\r\nEND:VCARD\r\n"
        (book / 'alice.vcf').write_bytes(fixture.encode())
        (book / 'bob.vcf').write_bytes(b'BEGIN:VCARD\r\nVERSION:3.0\r\nUID:example-two\r\nN:Builder;Bob;;;\r\nFN:Bob Builder\r\nEND:VCARD\r\n')
        queries = ['', 'name:Alice', 'name:Zoë, Alice Marie', 'name:Ally', 'phone_numbers:49301234567', 'phone_numbers:0301234567', 'phone_numbers:+49307654321', 'phone_numbers:home', 'phone_numbers:999999', 'emails:alice', 'post_addresses:Test City', 'organisations:Engineering', 'titles:Engineer', 'roles:Developer', 'notes:second line', 'categories:Test,One', 'nicknames:Ally', 'webpages:example.invalid', 'kind:individual', 'version:3.0', 'uid:example-one', 'missing:value']
        commands = [('query '+query, ['list', '-p'] + ([query] if query else [])) for query in queries]
        for field in ['index', 'name', 'first_name', 'last_name', 'formatted_name', 'kind', 'version', 'emails.work.0', 'emails.work.7', 'phone_numbers.home.0', 'post_addresses.home.0.city', 'organisations.0.0', 'organisations.0.1', 'roles.0', 'titles.0', 'notes.0', 'categories.0', 'nicknames.0', 'webpages.0', 'emails', 'roles', 'organisations']:
            commands.append(('column '+field, ['list', '-p', '-F', 'uid,'+field]))
        for order in ['first_name', 'last_name', 'formatted_name']:
            commands.append(('display '+order, ['list', '-p', '-d', order, '-s', order]))
        for label, command in commands:
            results = [subprocess.run([exe, '-c', str(config), *command], capture_output=True, text=True) for exe in [args.reference, args.candidate]]
            rows = [result.stdout.splitlines() for result in results]
            expected_difference = 'ASCII controls sanitized in TSV fields' if label == 'column notes.0' else None
            expected = results[0].stdout.replace('Old\nsecond line', 'Old second line').splitlines() if expected_difference else rows[0]
            cases.append({'case': label, 'passed': results[0].returncode == results[1].returncode and expected == rows[1], 'exit_codes': [r.returncode for r in results], 'expected_difference': expected_difference, 'reference': rows[0], 'candidate': rows[1], 'errors': [r.stderr for r in results]})
    report = {'reference_version': subprocess.run([args.reference, '--version'], capture_output=True, text=True, check=True).stdout.strip(), 'synthetic_only': True, 'passed': all(c['passed'] for c in cases), 'cases': cases}
    Path(args.report).write_text(json.dumps(report, indent=2, ensure_ascii=False)+'\n', encoding='utf-8')
    for case in cases:
        if not case['passed']:
            print(json.dumps(case, ensure_ascii=False))
    print(f"{sum(c['passed'] for c in cases)}/{len(cases)} compatibility cases passed; report: {args.report}")
    raise SystemExit(0 if report['passed'] else 1)


if __name__ == '__main__':
    main()

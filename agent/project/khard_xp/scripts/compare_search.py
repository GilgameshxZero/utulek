#!/usr/bin/env python3
"""Stage 3 search comparisons on synthetic books; retain detailed results."""
import argparse
import json
from pathlib import Path
import subprocess
import tempfile

FIXTURES = {
    'alice': 'VERSION:3.0\nUID:alice\nN:Example;Alice;Marie;Dr.;Jr.\nFN:Alice Example\nNICKNAME:Ally,Test\\,Nick\nTEL;TYPE=home:+49 (30) 123-4567\nEMAIL;TYPE=work:alice@example.invalid\nADR;TYPE=home:Box;Extended;First Street,Second Street;City;Region;10000;Country\nORG:Example;Engineering\nTITLE:Engineer\nROLE:Developer\nCATEGORIES:Friends,Test\\,One\nBDAY:19900403\nX-ANNIVERSARY:20010102\nNOTE:Old\\nsecond line\nURL:https://example.invalid\nX-SECRET:visible-secret\nX-HIDDEN:hidden-value\nREV:20260101T000000Z\n',
    'bob': 'VERSION:4.0\nUID:bob\nN:Builder;Bob;;;\nFN:Bob Builder\nEMAIL:bob@example.invalid\nTEL:030 765-4321\nBDAY;VALUE=text:Spring equinox\nANNIVERSARY:--04-03\nNOTE:missing:value\n',
    'folded': 'VERSION:3.0\nUID:folded\nFN:Ali\n ce Folded\nNOTE:colon: value\n',
    'tagged': 'VERSION:3.0\nUID:tagged\nFN:Tagged Contact\nitem1.TITLE:Captain\nitem1.X-ABLabel:jobtag\nitem2.NOTE:tagged note\nitem2.X-ABLabel:memotag\nitem3.X-SECRET:private tagged value\nitem3.X-ABLabel:privatetag\nitem4.EMAIL;TYPE=work,X-Team:team@example.invalid\nitem4.X-ABLabel:Team\n',
    'empty': 'VERSION:3.0\nUID:empty\nFN:Empty Person\n',
}
QUERIES = [
    'Alice', 'Engineering', 'Dr. Alice Marie Example Jr.', 'Example;Engineering',
    'Name: Alice', 'Full name: Dr.', 'address book: synthetic', 'kind:individual',
    'General:', 'miscellaneous', 'E-Mail', 'Phone', 'Private:', 'SECRET',
    'visible-secret', 'hidden-value', 'X-HIDDEN:hidden-value', 'REV:', '20260101',
    '3.0', 'BEGIN:VCARD', 'VCARD', 'version:3.0', 'uid:alice', 'uid:',
    'first_name:Alice', 'last_name:Example', 'formatted_name:Alice', 'name:Ally',
    'name:Test,Nick', 'nicknames:Test,Nick', 'nicknames:', 'categories:Test,One',
    'organisations:Engineering', 'organisations:Example;Engineering',
    'roles:Developer', 'titles:Engineer', 'emails:work', 'emails:internet', 'emails:',
    'post_addresses:street', 'post_addresses:Second Street', 'post_addresses:First Street,Second Street',
    'post_addresses:10000', 'notes:second line', 'notes:Old\nsecond line',
    'missing:value', 'unknown:value', 'name:Alice Folded', 'phone_numbers:home',
    'phone_numbers:voice', 'phone_numbers:0301234567', 'phone_numbers:+49307654321',
    'birthday:1990-04-03', 'birthday:1990', 'birthday:', 'birthday:spring',
    'anniversary:2001-01-02', 'anniversary:1900-04-03', 'anniversary:04-03',
    'jobtag', 'titles:jobtag', 'notes:memotag', 'privatetag', 'private tagged value', 'emails:Team', 'emails:X-Team', '1990-04-03', '2001-01-02', '1900-04-03', '--04-03',
]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--reference', default='khard')
    parser.add_argument('--candidate', default='.bin/khard')
    parser.add_argument('--report', default='.obj/search_compatibility.json')
    args = parser.parse_args()
    cases = []
    with tempfile.TemporaryDirectory(prefix='khard search ') as temporary:
        root = Path(temporary)
        book = root / 'contacts'
        book.mkdir()
        for name, content in FIXTURES.items():
            (book / (name + '.vcf')).write_text('BEGIN:VCARD\n' + content + 'END:VCARD\n')
        config = root / 'khard.conf'
        config.write_text(f"[addressbooks]\n[[synthetic]]\npath = '{book}'\n[contact table]\nsort = formatted_name\nlocalize_dates = no\n[vcard]\nprivate_objects = SECRET,\nsearch_in_source_files = no\n")
        queries = [[q] for q in QUERIES] + [['Alice', 'Engineering'], ['name:Alice', 'notes:second line'], ['Alice', 'missing']]
        for fast in [False, True]:
            for query in queries:
                command = ['-c', str(config), 'list', '-p', '-F', 'uid', *(['-f'] if fast else []), '--', *query]
                results = [subprocess.run([exe, *command], capture_output=True, text=True, timeout=10) for exe in [args.reference, args.candidate]]
                # Case-insensitive field names are an intentional existing extension.
                exception = query in [['Name: Alice'], ['Private:'], ['name:Test,Nick'], ['nicknames:Test,Nick']]
                expected_extension = query == ['Private:'] or query == ['Name: Alice'] and not fast or query in [['name:Test,Nick'], ['nicknames:Test,Nick']] and not fast
                exception_ok = exception and results[1].returncode == (0 if expected_extension else 1)
                if query == ['Private:']:
                    exception_ok = exception_ok and sorted(results[1].stdout.splitlines()) == sorted(FIXTURES)
                elif expected_extension:
                    exception_ok = exception_ok and results[1].stdout == 'alice\n'
                elif exception:
                    exception_ok = exception_ok and results[1].stdout == ''
                passed = results[0].returncode == results[1].returncode and sorted(results[0].stdout.splitlines()) == sorted(results[1].stdout.splitlines())
                cases.append({'query': query, 'source_search': fast, 'passed': passed or exception_ok, 'expected_difference': 'existing YAML/case-insensitive alias extension' if query in [['Private:'], ['Name: Alice']] else ('retain all comma-separated nicknames; upstream 0.19.1 reads only the first' if exception else None), 'reference': {'exit_code': results[0].returncode, 'stdout': results[0].stdout, 'stderr': results[0].stderr}, 'candidate': {'exit_code': results[1].returncode, 'stdout': results[1].stdout, 'stderr': results[1].stderr}})
        base = config.read_text()
        variants = {
            'multiline command': base + '[general]\neditor = """vim\n-i NONE""" # literal multiline scalar\n',
            'multiline book path': base.replace("path = '" + str(book) + "'", 'path = """' + str(book) + '""" # triple quoted path'),
            'debug config': base + '[general]\ndebug = yes\n',
            'explicit vdir type': base.replace('[contact table]', 'type = vdir\n[contact table]'),
        }
        for label, value in variants.items():
            config.write_text(value)
            results = [subprocess.run([exe, '-c', str(config), 'list', '-p', '-F', 'uid', 'name:Alice'], capture_output=True, text=True, timeout=10) for exe in [args.reference, args.candidate]]
            passed = results[0].returncode == results[1].returncode == 0 and sorted(results[0].stdout.splitlines()) == sorted(results[1].stdout.splitlines())
            if label == 'debug config':
                passed = passed and 'DEBUG' in results[1].stderr
            cases.append({'case': label, 'passed': passed, 'reference': {'exit_code': results[0].returncode, 'stdout': results[0].stdout, 'stderr': results[0].stderr}, 'candidate': {'exit_code': results[1].returncode, 'stdout': results[1].stdout, 'stderr': results[1].stderr}})
    report = {'reference_version': subprocess.run([args.reference, '--version'], capture_output=True, text=True, check=True).stdout.strip(), 'synthetic_only': True, 'passed': all(c['passed'] for c in cases), 'cases': cases}
    Path(args.report).write_text(json.dumps(report, indent=2, ensure_ascii=False) + '\n')
    for case in cases:
        if not case['passed']:
            print(json.dumps(case, ensure_ascii=False))
    print(f"{sum(c['passed'] for c in cases)}/{len(cases)} search cases passed; report: {args.report}")
    raise SystemExit(0 if report['passed'] else 1)


if __name__ == '__main__':
    main()

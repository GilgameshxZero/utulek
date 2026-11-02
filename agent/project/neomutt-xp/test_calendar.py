#!/usr/bin/env python3
"""Compare the stdlib helper to saved output from the original vobject helper."""
import argparse
from datetime import date, datetime, timedelta, timezone
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parent
sys.path.insert(0, str(ROOT / 'python'))
import ical_support

BASE = 'BEGIN:VCALENDAR\nVERSION:2.0\nMETHOD:REQUEST\n{zones}BEGIN:VEVENT\nUID:fixture-event\nSUMMARY:{summary}\n{times}{extra}ORGANIZER:mailto:organizer@example.test\nATTENDEE;CN="Fixture Person";EMAIL=person@example.test:mailto:person@example.test\nEND:VEVENT\nEND:VCALENDAR\n'
ZONE = '''BEGIN:VTIMEZONE
TZID:Custom/Eastern
BEGIN:STANDARD
DTSTART:19701101T020000
TZOFFSETFROM:-0400
TZOFFSETTO:-0500
RRULE:FREQ=YEARLY;BYMONTH=11;BYDAY=1SU
END:STANDARD
BEGIN:DAYLIGHT
DTSTART:19700308T020000
TZOFFSETFROM:-0500
TZOFFSETTO:-0400
RRULE:FREQ=YEARLY;BYMONTH=3;BYDAY=2SU
END:DAYLIGHT
END:VTIMEZONE
'''


def fixtures():
    times = {
        'utc_duration': 'DTSTART:20300102T120000Z\nDURATION:PT1H30M\n',
        'utc_end': 'DTSTART:20300102T120000Z\nDTEND:20300102T133000Z\n',
        'floating': 'DTSTART:20300102T120000\nDTEND:20300102T133000\n',
        'all_day': 'DTSTART;VALUE=DATE:20300102\nDTEND;VALUE=DATE:20300103\n',
        'iana_winter': 'DTSTART;TZID=America/New_York:20300102T120000\nDURATION:PT1H\n',
        'iana_summer': 'DTSTART;TZID=Europe/Berlin:20300702T120000\nDURATION:PT1H\n',
        'custom_winter': 'DTSTART;TZID=Custom/Eastern:20300102T120000\nDURATION:PT1H\n',
        'custom_summer': 'DTSTART;TZID=Custom/Eastern:20300702T120000\nDURATION:PT1H\n',
        'custom_fall_fold': 'DTSTART;TZID=Custom/Eastern:20301103T013000\nDURATION:PT1H\n',
        'custom_spring_gap': 'DTSTART;TZID=Custom/Eastern:20300310T023000\nDURATION:PT1H\n',
    }
    result = {}
    for name, value in times.items():
        result[name] = BASE.format(zones=ZONE if name.startswith('custom') else '',
            summary='Fixture meeting', times=value, extra='')
    result['unicode_folded'] = BASE.format(zones='',summary='Café 世界\\, semicolon\\; slash\\\\\n continuation',
        times=times['utc_duration'],extra='DESCRIPTION:Line one\\nLine two\\, more\nLOCATION:Room\\, A\n')
    result['recurring'] = BASE.format(zones='',summary='Recurring fixture',times=times['utc_end'],
        extra='RECURRENCE-ID:20300102T120000Z\nSEQUENCE:7\nRRULE:FREQ=WEEKLY;COUNT=3\n')
    return result


def canonical(calendar):
    result = {}
    for key, props in calendar.vevent.contents.items():
        if key == 'dtstamp':
            continue
        values = []
        for prop in props:
            value = prop.value
            if isinstance(value, datetime):
                value = value.astimezone(timezone.utc).isoformat() if value.tzinfo else value.isoformat()
            elif isinstance(value, date):
                value = value.isoformat()
            elif isinstance(value, timedelta):
                value = value.total_seconds()
            # X-VOBJ-ORIGINAL-TZID is vobject's in-memory parsing bookkeeping,
            # not a property written into the reply's content line.
            params = {k:v for k,v in prop.params.items() if k not in ('TZID','VALUE','X-VOBJ-ORIGINAL-TZID')}
            values.append([value, params])
        result[key] = values
    result['method'] = calendar.method.value
    result['prodid'] = calendar.prodid.value
    return result


def run(script, directory, content, flags, answer='', isolated=False):
    invitation = directory / 'invite ü with spaces.ics'
    invitation.write_text(content, encoding='utf-8', newline='')
    output = directory / 'reply ü with spaces.ics'
    output.unlink(missing_ok=True)
    args = [sys.executable] + (['-S'] if isolated else []) + [str(script), *flags]
    if '-D' not in flags:
        args += ['-g', str(output)]
    args += [str(invitation)]
    env = {**os.environ,'NEOMUTT_FROM':'person@example.test','PYTHONUTF8':'1','PYTHONIOENCODING':'utf-8'}
    response = subprocess.run(args,input=answer.encode(),capture_output=True,env=env,timeout=10)
    return response, output


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--original', type=Path)
    parser.add_argument('--oracle', type=Path, required=True)
    parser.add_argument('--write-oracle', action='store_true')
    args = parser.parse_args()
    oracle = {} if args.write_oracle else json.loads(args.oracle.read_text())
    results = []
    with tempfile.TemporaryDirectory(prefix='neomutt_calendar_') as temporary:
        directory = Path(temporary)
        for name, content in fixtures().items():
            for action, flags, answer in [('display',['-D'],''),('accept',['-a'],''),
                    ('decline',['-d'],''),('tentative',['-t'],''),('interactive_accept',['-i'],'y\n'),
                    ('interactive_decline',['-i'],'n\n'),('interactive_tentative',['-i'],'t\n'),
                    ('cancel',['-i'],'c\n')]:
                key = name + '/' + action
                if args.write_oracle:
                    import vobject
                    response, output = run(args.original,directory,content,flags,answer)
                    oracle[key] = {'exit':response.returncode,'stdout':response.stdout.decode('utf-8'),
                                   'reply':canonical(vobject.readOne(output.read_text())) if output.exists() else None}
                else:
                    response, output = run(ROOT/'python/mutt-ical.py',directory,content,flags,answer,True)
                    reply = canonical(ical_support.readOne(output.read_text(encoding='utf-8'))) if output.exists() else None
                    actual = {'exit':response.returncode,'stdout':response.stdout.decode('utf-8'),'reply':reply}
                    results.append({'name':key,'passed':actual==oracle[key],
                        'detail': '' if actual==oracle[key] else {'expected':oracle[key],'actual':actual,
                                                                 'stderr':response.stderr.decode('utf-8',errors='replace')}})
    if args.write_oracle:
        args.oracle.write_text(json.dumps(oracle,ensure_ascii=True,indent=2))
        print('Recorded',len(oracle),'original-helper cases')
    else:
        print(json.dumps({'results':results},ensure_ascii=True,indent=2))
        return 0 if all(row['passed'] for row in results) else 1
    return 0


if __name__ == '__main__':
    raise SystemExit(main())

"""Shared reply-address fixtures: no real accounts or transport are involved."""
from pathlib import Path
import json

CASES = [
    dict(name='incoming_index', sender='outside@example.invalid', to='reader@owned.invalid', key='r', expected=['outside@example.invalid']),
    dict(name='incoming_pager', sender='outside@example.invalid', to='reader@owned.invalid', key='r', pager=True, expected=['outside@example.invalid']),
    dict(name='reply_to', sender='outside@example.invalid', to='reader@owned.invalid', reply_to='responses@example.invalid', key='r', expected=['responses@example.invalid']),
    dict(name='own_sent', sender='writer@owned.invalid', to='outside@example.invalid', key='r', expected=['outside@example.invalid']),
    dict(name='both_owned', sender='writer@owned.invalid', to='reader@owned.invalid', key='r', expected=['reader@owned.invalid']),
    dict(name='reply_self', sender='writer@owned.invalid', to='reader@owned.invalid', key='r', reply_self=True, expected=['writer@owned.invalid']),
    dict(name='group', sender='outside@example.invalid', to='reader@owned.invalid, other@example.invalid', cc='copy@example.invalid', key='g', expected=['outside@example.invalid', 'other@example.invalid', 'copy@example.invalid']),
]

def prepare(root, editor, delimiter):
    root = Path(root).resolve(); root.mkdir(parents=True, exist_ok=True)
    cases = []
    for case in CASES:
        case = dict(case); directory=root/case['name']; directory.mkdir(exist_ok=True)
        box=directory/'fixture.maildir'
        for d in ['cur','new','tmp']: (box/d).mkdir(parents=True,exist_ok=True)
        headers=f"From: {case['sender']}\nTo: {case['to']}\nSubject: Reply address fixture\nDate: Tue, 03 Nov 2026 00:00:00 +0000\n"
        for key, field in [('reply_to','Reply-To'),('cc','Cc')]:
            if key in case: headers+=f'{field}: {case[key]}\n'
        (box/'cur'/f'fixture{delimiter}2,S').write_text(headers+'\nSynthetic reply body.\n')
        draft=(directory/'draft.mbox').as_posix()
        rc=directory/'test.rc'
        rc.write_text(f'''set folder="{directory.as_posix()}" crypt_use_gpgme=no
set from="reader@owned.invalid" realname="Fixture Reader" charset="utf-8"
alternates @owned.invalid
set reverse_name=yes reply_self={'yes' if case.get('reply_self') else 'no'} reply_to=yes
set abort_unmodified=no
set autoedit=yes fast_reply=yes include=no quit=yes postpone=no recall=no move=no mark_old=no
set editor="{editor}" sleep_time=0 wait_key=no
set maildir_field_delimiter="{delimiter}"
unset record
color normal default default
color indicator reverse default default
color status bold reverse default default
macro compose Z "<write-fcc>{draft}<enter>"
''')
        case.update(directory=str(directory), box=str(box), rc=str(rc), draft=draft)
        cases.append(case)
    (root/'cases.json').write_text(json.dumps(cases,indent=2))
    return cases

if __name__=='__main__':
    import sys
    prepare(*sys.argv[1:])

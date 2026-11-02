#!/usr/bin/env python3
"""Stage shell adapters beside unchanged email scripts; never decrypt or install."""
import argparse
from pathlib import Path
import re
import shutil


def render(text):
    def replace(pattern, result):
        nonlocal text
        text, count = re.subn(pattern, lambda match: result(match) if callable(result) else result,
                              text, flags=re.M)
        if count != 1:
            raise ValueError(f'Expected one match for {pattern!r}; found {count}')
    # Resolve helper path after NeoMutt itself has expanded the home directory.
    # Quoting ./ is necessary for cmd; the unquoted spelling fails there.
    replace(r'^(set my_archive_dir=.*)$', lambda m: m[0] +
            '\ncd "$my_active_dir"\nset my_email_command="`"./neomutt-command" path`"')
    command = r'\"$my_email_command\"'
    cwd = re.search(r'^cd (.*snowfall/)$', text, re.M)
    if not cwd:
        raise ValueError('Cannot locate the final attachment working directory')
    final_cd = cwd[0]
    replace(r'^cd (.*snowfall/)$', '')
    # Unlike set, named-mailboxes does not expand custom config variables
    # inside backticks. Run the relative bootstrap while still in email/.
    replace(r'vault `ls -d "\$HOME/(.*?)"`',
            lambda m: 'vault "`"./neomutt-command" vault "~/' + m[1] + '"`"\n' + final_cd)
    replace(r'^source "`gpg .*?(?:\n.*?)*?&`"$',
            'source "' + command + ' password|"')
    replace(r'\$\{my_active_dir\}mutt_oauth2\.py', command + ' oauth')
    replace(r'(\$\{my_active_dir\}[^\s"\\]*token\.gpg)', lambda m: r'\"' + m[1] + r'\"')
    replace(r'^set query_command="echo %s \| \\\n\s*xargs khard email -d formatted_name --parsable --"$',
            'set query_command="' + command + ' query %s"')
    # Two capture macros share the same command spelling.
    text, count = re.subn(r'iconv -c --to-code=UTF-8 > \.event\.tmp\.ics',
                         lambda _: command + ' capture .event.tmp.ics', text)
    if count != 2:
        raise ValueError('Expected two calendar capture commands')
    replace(r'\$\{my_active_dir\}mutt-ical\.py', command + ' ical')
    replace(r'<shell-escape>rm \.event\.tmp\.ics', '<shell-escape>' + command + ' remove .event.tmp.ics')
    replace(r'mv \.event\.tmp\.ics (.*?)/\$\(uuidgen\)\.ics',
            lambda m: command + ' save .event.tmp.ics \\"' + m[1] + '\\"')
    replace(r'\$\{my_active_dir\}neomutt-ical-request\.sh', command + ' request')
    return text


def render_mailcap(text):
    text, html_count = re.subn(r'[^;\s]*neomutt-html-open\.sh',
                              '"@NEOMUTT_COMMAND@" html', text)
    text, ical_count = re.subn(r'[^;\s]*mutt-ical\.py',
                              '"@NEOMUTT_COMMAND@" ical', text)
    if html_count != 1 or ical_count != 3:
        raise ValueError('Unexpected HTML/calendar mailcap entries')
    return text


def stage(source, destination):
    destination.mkdir(parents=True, exist_ok=True)
    for name in ('mutt-ical.py', 'mutt_oauth2.py', 'neomutt-html-open.sh', 'neomutt-ical-request.sh'):
        shutil.copy2(source / name, destination / name)
    for path in (Path(__file__).parent / 'runtime').iterdir():
        shutil.copy2(path, destination / path.name)
    for path in (Path(__file__).parent / 'python').iterdir():
        if path.is_file():
            shutil.copy2(path, destination / path.name)
    text = render((source / 'neomuttrc').read_text())
    # Environment variables expand differently under cmd and sh. Invoke an
    # extensionless mailcap dispatcher from the snowfall working directory via
    # absolute path, emitted by a platform helper when config is sourced.
    text = text.replace('set mailcap_path="${my_active_dir}mailcap"',
                        'set mailcap_path="`"$my_email_command" mailcap`"')
    (destination / 'neomuttrc').write_text(text)
    (destination / 'mailcap').write_text(render_mailcap((source / 'mailcap').read_text()))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('source', type=Path)
    parser.add_argument('destination', type=Path)
    args = parser.parse_args()
    stage(args.source, args.destination)

#!/usr/bin/env python3
"""Compare parsed NeoMutt settings using fake GPG and disposable Maildirs."""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
from stage_config import stage


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('source', type=Path)
    parser.add_argument('output', type=Path)
    args = parser.parse_args()
    root = args.output.resolve()
    root.mkdir(parents=True, exist_ok=True)
    candidate = root / 'candidate with spaces ü'
    stage(args.source, candidate)
    original = (args.source / 'neomuttrc').read_text()
    adapted = (candidate / 'neomuttrc').read_text()
    home = root / 'home'
    (home / 'main.syncthing/snowfall').mkdir(parents=True, exist_ok=True)
    # Find the baseline's vault path without invoking its shell substitutions.
    vault = re.search(r'vault `ls -d "\$HOME/(.*?)"`', original)[1]
    for part in ('cur','new','tmp'):
        (home / vault / part).mkdir(parents=True, exist_ok=True)
    tools = root / 'tools'
    tools.mkdir(exist_ok=True)
    names = sorted(set(re.findall(r'my_smtp_pass_[a-z_]+', original)))
    dummy = '\n'.join('set ' + name + '=""' for name in names)
    fake_gpg = tools / 'gpg'
    fake_gpg.write_text('#!/usr/bin/env bash\ncat <<\'FIXTURE\'\n' + dummy + '\nFIXTURE\n')
    fake_gpg.chmod(0o755)
    environment = {**os.environ, 'HOME': str(home), 'PATH': str(tools) + os.pathsep + os.environ['PATH']}
    outputs, diagnostics = {}, {}
    for label, text in (('original', original), ('candidate', adapted)):
        text = re.sub(r'^set my_active_dir=.*$', lambda _: 'set my_active_dir="' + str(candidate) + '/"', text, flags=re.M)
        if label == 'original':
            text = re.sub(r'^source "`gpg .*?(?:\n.*?)*?&`"$', lambda _: dummy, text, flags=re.M)
        config = root / 'parser.neomuttrc'
        config.write_text(text)
        def parse():
            return subprocess.run(['neomutt', '-n', '-F', str(config), '-D'],
                                  capture_output=True, text=True, env=environment)
        # First read discovers resolved option paths, all restricted to root.
        first = parse()
        for value in re.findall(r'^set \w+ = "([^"\n]*\.maildir/)"$', first.stdout, re.M):
            value = str(home) + value[1:] if value.startswith('~/') else value
            mailbox = Path(value)
            if not mailbox.is_relative_to(root):
                raise ValueError('Fixture path escaped the test directory')
            for part in ('cur', 'new', 'tmp'):
                (mailbox / part).mkdir(parents=True, exist_ok=True)
        result = parse()
        (root / (label + '_parsed.txt')).write_text(result.stdout)
        diagnostics[label] = {'exit': result.returncode, 'stderr': result.stderr}
        outputs[label] = dict(line.removeprefix('set ').split(' = ', 1)
                              for line in result.stdout.splitlines() if ' = ' in line)
    expected = {'my_email_command', 'imap_oauth_refresh_command', 'smtp_oauth_refresh_command',
                'query_command', 'mailcap_path', 'my_ics_response_macro_pre', 'my_ics_response_macro_post'}
    changed = sorted(key for key in set(outputs['original']) | set(outputs['candidate'])
                     if outputs['original'].get(key) != outputs['candidate'].get(key))
    report = {'settings_compared': len(outputs['original']), 'changed_settings': changed,
              'only_expected_command_settings_changed': set(changed) <= expected,
              'diagnostics': diagnostics,
              'python_and_original_shell_scripts_unchanged': all(
                  (args.source / name).read_bytes() == (candidate / name).read_bytes()
                  for name in ('mutt-ical.py', 'mutt_oauth2.py', 'neomutt-html-open.sh', 'neomutt-ical-request.sh'))}
    (root / 'results.json').write_text(json.dumps(report, indent=2))
    print(json.dumps(report, indent=2))
    return 0 if report['only_expected_command_settings_changed'] and all(
        not row['stderr'] and row['exit'] == 0 for row in diagnostics.values()) else 1


if __name__ == '__main__':
    raise SystemExit(main())

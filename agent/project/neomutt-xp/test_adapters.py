#!/usr/bin/env python3
"""Disposable adapter regression tests. No account data or real mail operations."""
import argparse
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import time


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('originals', type=Path)
    parser.add_argument('--observer', type=Path)
    args = parser.parse_args()
    results = []
    with tempfile.TemporaryDirectory(prefix='neomutt adapters ü ') as temporary:
        root = Path(temporary)
        module = root / 'monochrome/module/email'
        module.mkdir(parents=True)
        home = root / 'home'
        home.mkdir()
        work = root / 'work'
        work.mkdir()
        tools = root / 'tools'
        tools.mkdir()
        runtime = Path(__file__).parent / 'runtime'
        for source in runtime.iterdir():
            shutil.copy2(source, module / source.name)
        for name in ('neomutt-ical-request.sh', 'neomutt-html-open.sh'):
            shutil.copy2(args.originals / name, module / name)
        environment = dict(os.environ, HOME=str(home), NEOMUTT_FROM='sender@example.test')
        environment['PATH'] = str(tools) + os.pathsep + environment['PATH']
        # Fake only external actions. The original shell pipelines still run.
        stubs = {
            'gpg': '#!/usr/bin/env bash\nprintf \'set my_test="fixture"\\n\'\nexit "${TEST_GPG_EXIT:-0}"\n',
            'uuidgen': '#!/usr/bin/env bash\necho 00000000-0000-4000-8000-000000000000\n',
            'fixture-browser': '#!/usr/bin/env bash\nprintf \'%s\\n\' "$@" > "$TEST_BROWSER_LOG"\n',
        }
        for name, script in stubs.items():
            path = tools / name
            path.write_text(script, encoding='utf-8', newline='\n')
            path.chmod(0o755)
        # Fake Python helpers establish exact argv/cwd handoff without touching
        # the Python portability work explicitly deferred by the user.
        for name in ('mutt-ical.py', 'mutt_oauth2.py'):
            (module / name).write_text('import json,os,sys\nprint(json.dumps({"argv":sys.argv[1:],"cwd":os.getcwd()},ensure_ascii=True))\n')
        command_path = str(module / 'neomutt-command').replace('\\', '/')
        def quote_windows(value):
            value = subprocess.list2cmdline([str(value)])
            return value if value.startswith('"') else '"' + value + '"'
        if os.name == 'nt':
            def call(action, arguments=(), data=None, env=None):
                # Actual cmd command-string interface, rather than incorrectly
                # applying CRT argv quoting to cmd.exe's /c argument.
                command = '"' + command_path + '" ' + action + ''.join(' ' + quote_windows(x) for x in arguments)
                return subprocess.run(command, shell=True, input=data, cwd=work,
                    env=env or environment, capture_output=True)
        else:
            def call(action, arguments=(), data=None, env=None):
                return subprocess.run([command_path, action, *map(str, arguments)], input=data,
                    cwd=work, env=env or environment, capture_output=True)
        def check(name, passed, detail=''):
            results.append({'name': name, 'passed': bool(passed), 'detail': detail})
        result = call('path')
        check('extensionless_dispatch_and_native_path', result.returncode == 0 and
              Path(result.stdout.decode().strip()).resolve() == (module / 'neomutt-command').resolve(), result.stderr.decode(errors='replace'))
        generated=work/'generated.ics'
        generated.write_bytes(b'fixture calendar\n')
        result=call('keep',[generated])
        check('mailcap_compose_preserves_generated_file',result.returncode==0 and
              generated.read_bytes()==b'fixture calendar\n' and not result.stdout)
        vault = home / 'vault & ü'
        vault.mkdir()
        result = call('vault', ['~/vault & ü'])
        check('vault_home_and_native_path', result.returncode == 0 and
              Path(result.stdout.decode().strip()).resolve() == vault.resolve(), result.stderr.decode(errors='replace'))
        (module / 'mailcap.template').write_text('text/html; "@NEOMUTT_COMMAND@" html %s\n',encoding='utf-8')
        result = call('mailcap')
        generated = Path(result.stdout.decode().strip())
        check('mailcap_native_path_and_command', result.returncode == 0 and generated.exists() and
              '@NEOMUTT_COMMAND@' not in generated.read_text(encoding='utf-8') and
              command_path in generated.read_text(encoding='utf-8'), result.stderr.decode(errors='replace'))
        for name, data in [('ascii', b'BEGIN:VCALENDAR\nEND:VCALENDAR\n'),
                           ('unicode', 'SUMMARY:café 世界\r\n'.encode()),
                           ('invalid_utf8', b'abc\xff\xc3\xa9\n'), ('empty', b'')]:
            destination = work / ('capture & ' + name + '.ics')
            result = call('capture', [destination], data)
            baseline = subprocess.run(['iconv', '-c', '--to-code=UTF-8'], input=data,
                                      capture_output=True, env=environment)
            check('capture_' + name, result.returncode == baseline.returncode and
                  destination.read_bytes() == baseline.stdout, result.stderr.decode(errors='replace'))
        result = call('password')
        check('password_output', result.returncode == 0 and result.stdout.replace(b'\r\n',b'\n') == b'set my_test="fixture"\n', result.stderr.decode(errors='replace'))
        failed_env = {**environment, 'TEST_GPG_EXIT': '2'}
        result = call('password', env=failed_env)
        check('password_failure_emits_no_partial_config', result.returncode != 0 and not result.stdout)
        for action in ('ical', 'oauth'):
            values = ['fixture with spaces & ü.ics', '-g', 'reply (1).ics'] if action == 'ical' else ['token with spaces.gpg', '--decryption-pipe', 'gpg --decrypt']
            result = call(action, values)
            try:
                handed_off = json.loads(result.stdout)
            except Exception:
                handed_off = {}
            check(action + '_argv_and_cwd', result.returncode == 0 and handed_off.get('argv') == values and
                  Path(handed_off.get('cwd','')).resolve() == work.resolve(), result.stderr.decode(errors='replace'))
        destination = work / 'calendar & ü'
        destination.mkdir()
        attachment = work / '.event.tmp.ics'
        attachment.write_bytes(b'ICS fixture\r\n')
        result = call('save', [attachment, destination])
        files = list(destination.glob('*.ics'))
        check('calendar_save_uuid_content_and_move', result.returncode == 0 and len(files) == 1 and
              re.fullmatch(r'[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}\.ics', files[0].name) and
              files[0].read_bytes() == b'ICS fixture\r\n' and not attachment.exists(), result.stderr.decode(errors='replace'))
        if os.name == 'nt':
            if not args.observer:
                raise ValueError('Windows query tests require the compiled argv observer')
            app = root / 'monochrome/app'
            app.mkdir()
            shutil.copy2(args.observer, app / 'khard.exe')
            khard = str(app / 'khard.exe').replace('\\','/')
        else:
            khard = str(tools / 'khard')
            Path(khard).write_text('#!/usr/bin/env bash\nexec python3 -c \'import json,sys; print(json.dumps(sys.argv[1:],ensure_ascii=True,separators=(",",":")))\' "$@"\n')
            Path(khard).chmod(0o755)
        query_baseline = root / 'query_baseline.sh'
        query_baseline.write_text('#!/usr/bin/env bash\necho "$1" | xargs "$2" email -d formatted_name --parsable --\n',newline='\n')
        for query in ('Alpha','two words','a&b','a^b','name;touch NO','quoted "two words"','O\'Brien','', '世界'):
            baseline_command = ['bash',str(query_baseline).replace('\\','/'),query,khard]
            if os.name == 'nt':
                baseline_command = 'bash ' + ' '.join(quote_windows(x) for x in baseline_command[1:])
            baseline = subprocess.run(baseline_command, shell=os.name == 'nt',
                capture_output=True,env=environment,cwd=work)
            result = call('query',[query])
            check('query_parity_' + query, (result.returncode,result.stdout,result.stderr) ==
                  (baseline.returncode,baseline.stdout,baseline.stderr),
                  repr(((baseline.returncode,baseline.stdout,baseline.stderr),
                        (result.returncode,result.stdout,result.stderr))))
        # Compare request script's success and failure behavior to the original.
        for duration in ('45m','1h','1h30m','0h','wrong'):
            runs = []
            for baseline in (True, False):
                run_dir = root / ('baseline_' if baseline else 'candidate_') / duration
                run_dir.mkdir(parents=True)
                draft = run_dir / 'draft.mbox'
                draft.write_bytes(b'From sender@example.test Tue Jan 1 00:00:00 2030\nTo: Person <person@example.test>\nSubject: fixture\n\nbody\n')
                output = run_dir / '.request.tmp.ics'
                output.write_bytes(b'stale fixture\n')
                data = ('\n2030-01-02 12:30\n' + duration + '\n').encode()
                if baseline:
                    original = str((args.originals / 'neomutt-ical-request.sh').resolve()).replace('\\','/')
                    run = subprocess.run(['bash', original, 'draft.mbox'], cwd=run_dir,
                                          input=data, capture_output=True, env=environment)
                else:
                    old_work = work
                    work = run_dir
                    run = call('request', ['draft.mbox'], data)
                    work = old_work
                body = output.read_bytes() if output.exists() else b''
                body = re.sub(rb'UID:[^@\r\n]*@', b'UID:<uuid>@', body)
                body = re.sub(rb'DTSTAMP:[^\r\n]*', b'DTSTAMP:<timestamp>', body)
                runs.append((run.returncode, run.stdout, run.stderr, body, draft.exists()))
            matched=runs[0] == runs[1]
            if duration=='wrong':
                # Owned placeholders must not leave a stale ICS after a failed
                # generator. Exit status, prompts and draft handling still match.
                matched=runs[0][:3]==runs[1][:3] and runs[0][4]==runs[1][4] and runs[1][3]==b''
            check('request_parity_' + duration, matched,'' if matched else repr(runs))
        # HTML: compare exact wrapper bytes and opener argv. Fake timer/rm avoid
        # waiting 64 seconds; this checks command behavior, not timer accuracy.
        html_tools = root / 'html_tools'
        html_tools.mkdir()
        for name in ('sleep', 'rm'):
            stub = html_tools / name
            stub.write_text('#!/usr/bin/env bash\n' +
                            ('printf done > "$TEST_CLEANUP_DONE"\n' if name == 'rm' else '') +
                            'exit 0\n', newline='\n')
            stub.chmod(0o755)
        bodies = []
        for baseline in (True, False):
            run_dir = root / ('html_baseline' if baseline else 'html_candidate')
            run_dir.mkdir()
            (run_dir / 'input.html').write_bytes(b'<p>caf\xc3\xa9</p>')
            env = {**environment, 'BROWSER': str(tools / 'fixture-browser').replace('\\','/'),
                   'TEST_BROWSER_LOG': str(run_dir / 'browser.log'),
                   'TEST_CLEANUP_DONE': str(run_dir / 'cleanup.done'),
                   'PATH': str(html_tools) + os.pathsep + environment['PATH']}
            if baseline:
                result = subprocess.run(['bash', str((args.originals / 'neomutt-html-open.sh').resolve()).replace('\\','/'), 'input.html'],
                    cwd=run_dir, env=env, capture_output=True)
            else:
                old_work = work
                work = run_dir
                result = call('html', ['input.html'], env=env)
                work = old_work
            for _ in range(200):
                if (run_dir / 'cleanup.done').exists():
                    break
                time.sleep(0.02)
            time.sleep(0.05)
            bodies.append(((run_dir / '.neomutt-html-open.tmp.html').read_bytes(),
                           (run_dir / 'browser.log').read_bytes(), result.returncode))
        check('html_bytes_and_browser_argv_parity', bodies[0] == bodies[1])
        if os.name == 'nt':
            # Default Windows browser bridge: replace xdg-open with a recorder,
            # so this exercises cmd quoting without opening an actual viewer.
            observer = root / 'observe_browser.py'
            observer.write_text('import json,os,sys\nfrom pathlib import Path\nPath(os.environ["TEST_BROWSER_LOG"]).write_text(json.dumps(sys.argv[1:]),encoding="utf-8")\n')
            (tools / 'xdg-open.bat').write_text('@"%TEST_PYTHON%" "%TEST_OBSERVER%" %*\n')
            run_dir = root / 'default browser & ü'
            run_dir.mkdir()
            (run_dir / 'input.html').write_bytes(b'<p>fixture</p>')
            env = {**environment, 'TEST_PYTHON': __import__('sys').executable,
                   'TEST_OBSERVER': str(observer), 'TEST_BROWSER_LOG': str(run_dir / 'browser.log'),
                   'TEST_CLEANUP_DONE': str(run_dir / 'cleanup.done'),
                   'PATH': str(html_tools) + os.pathsep + environment['PATH']}
            env.pop('BROWSER', None)
            old_work = work
            work = run_dir
            result = call('html', ['input.html'], env=env)
            work = old_work
            for _ in range(200):
                if (run_dir / 'cleanup.done').exists():
                    break
                time.sleep(0.02)
            time.sleep(0.05)
            log = run_dir / 'browser.log'
            seen = json.loads(log.read_text(encoding='utf-8')) if log.exists() else []
            check('windows_default_browser_bridge_argv', result.returncode == 0 and len(seen) == 1 and
                  Path(seen[0]).resolve() == (run_dir / '.neomutt-html-open.tmp.html').resolve(),
                  result.stderr.decode(errors='replace'))
    print(json.dumps({'platform': os.name, 'results': results}, indent=2, ensure_ascii=True))
    return 0 if all(row['passed'] for row in results) else 1


if __name__ == '__main__':
    raise SystemExit(main())

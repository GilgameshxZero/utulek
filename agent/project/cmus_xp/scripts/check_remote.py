#!/usr/bin/env python3
"""Real isolated Linux remote CLI/protocol and keyboard regression checks."""
import argparse
import concurrent.futures
import json
import os
from pathlib import Path
import secrets
import socket
import signal
import subprocess
import time
import uuid
import wave

from linux_test_session import Session

ROOT = Path(__file__).resolve().parent.parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', type=Path, required=True)
    parser.add_argument('--remote', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--regressions-only', action='store_true')
    args = parser.parse_args()
    out = args.output.resolve()
    out.mkdir(parents=True, exist_ok=True)
    config = out / ('config-' + uuid.uuid4().hex[:8])
    config.mkdir()
    (config / 'rc').write_text('set mpris=false\nset softvol=true\n')
    session = Session(args.binary, args.remote, config)
    checks = []

    def check(name, callback):
        try:
            callback()
            checks.append({'check': name, 'passed': True})
        except Exception as error:
            checks.append({'check': name, 'passed': False, 'error': repr(error)})

    def cli(*arguments, text=None, timeout=4):
        process = subprocess.Popen([str(args.remote.resolve()), '--server', str(session.socket), *arguments],
            stdin=subprocess.PIPE if text is not None else subprocess.DEVNULL,
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
        # communicate drains output while Session.drain keeps the real terminal moving.
        with concurrent.futures.ThreadPoolExecutor(max_workers=1) as pool:
            result = pool.submit(process.communicate, text.encode() if text is not None else None)
            end = time.monotonic() + timeout
            while not result.done() and time.monotonic() < end:
                session.drain(.01)
            if not result.done():
                os.killpg(process.pid, signal.SIGKILL)
                result.result()
                raise TimeoutError('remote CLI stalled')
            stdout, stderr = result.result()
        assert process.returncode == 0, stderr.decode(errors='replace')
        return stdout.decode()

    def query():
        return cli('-Q')

    def fragmented():
        session.command('set repeat=false')
        with socket.socket(socket.AF_UNIX) as connection:
            connection.settimeout(2)
            connection.connect(str(session.socket))
            connection.sendall(b'set repe')
            session.drain(.15)
            connection.sendall(b'at=true\n')
            assert connection.recv(8192).endswith(b'\n')
        assert 'set repeat true\n' in query()

    def long_command():
        word = 'remote_' + 'x' * 4096
        assert word in cli('-C', 'echo ' + word)

    def stdin_commands():
        word = 'stdin_' + 'y' * 4096
        answer = cli(text='set repeat=false\r\necho ' + word)
        assert word in answer
        assert 'set repeat false\n' in query()

    def keyboard_and_cooked():
        session.keys(':set repeat=false\r')
        assert 'set repeat false\n' in query()
        cli('-R')
        assert 'set repeat true\n' in query()
        cli('-v', '37%')
        assert 'set vol_left 37\n' in query()
        assert 'Error:' in cli('-C', 'no-such-command')

    def utf8_exports():
        directory = out
        for n in range(4):
            directory /= 'directory_' + str(n) + '_' + 'ü' * 55
            directory.mkdir()
        track = directory / '音 spaced track.wav'
        with wave.open(str(track), 'wb') as stream:
            stream.setnchannels(2); stream.setsampwidth(2); stream.setframerate(8000)
            stream.writeframes(b'\0' * 32000)
        cli('-l', str(track))
        for _ in range(30):
            exported = cli('-C', 'save -l -')
            if str(track) in exported:
                break
            session.drain(.05)
        assert str(track) in exported
        cli('-q', str(track))
        for _ in range(30):
            if str(track) in cli('-C', 'save -q -'):
                break
            session.drain(.05)
        else:
            raise AssertionError('Cooked queue add/export failed')
        cli('-q', '-c')
        assert str(track) not in cli('-C', 'save -q -')

    def pipeline_half_close():
        with socket.socket(socket.AF_UNIX) as connection:
            connection.settimeout(2)
            connection.connect(str(session.socket))
            connection.sendall(b'set repeat=true\n' * 100)
            connection.shutdown(socket.SHUT_WR)
            data = bytearray()
            while len(data) < 100:
                part = connection.recv(8192)
                if not part: break
                data.extend(part)
            assert bytes(data) == b'\n' * 100, repr(data)

    def concurrent_queries():
        def request(_):
            return subprocess.check_output([str(args.remote.resolve()), '--server', str(session.socket), '-Q'], timeout=4)
        with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool:
            pending = [pool.submit(request, n) for n in range(8)]
            while any(not item.done() for item in pending): session.drain(.01)
            assert all(b'status stopped\n' in item.result() for item in pending)

    try:
        session.start()
        check('fragmented_commands_preserved_between_reads', fragmented)
        check('raw_command_larger_than_512_bytes', long_command)
        if not args.regressions_only:
            check('long_stdin_crlf_and_final_line_without_newline', stdin_commands)
            check('keyboard_raw_cooked_controls_and_errors', keyboard_and_cooked)
            check('long_utf8_cooked_paths_and_stdout_exports', utf8_exports)
            check('pipelined_commands_and_half_close', pipeline_half_close)
            check('eight_concurrent_remote_clients', concurrent_queries)
        session.stop()
        if not args.regressions_only:
            session.start()
            check('restart_and_reconnect', lambda: cli('-Q'))
            session.stop()
            missing = subprocess.run([str(args.remote.resolve()), '--server', str(session.socket), '-C'],
                                     capture_output=True, timeout=3)
            checks.append({'check': 'quiet_missing_server_probe', 'passed': missing.returncode == 1 and not missing.stdout and not missing.stderr})
            with socket.socket() as reserve:
                reserve.bind(('127.0.0.1', 0))
                address = '127.0.0.1:%d' % reserve.getsockname()[1]
            tcp_config = out / ('tcp-' + uuid.uuid4().hex[:8])
            tcp_config.mkdir()
            password = 'test_' + secrets.token_hex(16)
            (tcp_config / 'rc').write_text(
                'set mpris=false\nset softvol=true\nset passwd=' + password + '\n')
            tcp = Session(args.binary, args.remote, tcp_config, address=address, password=password)
            try:
                tcp.start()
                answer = tcp.command('status')
                assert 'status stopped\n' in answer
                assert tcp.command('run cmus_missing_test_program') == ''
                assert tcp.command('save -l -') == ''
                wrong = subprocess.run([str(args.remote.resolve()), '--server', address, '--passwd', 'wrong', '-Q'],
                                       capture_output=True, timeout=4)
                assert wrong.returncode == 1 and b'authentication failed' in wrong.stdout
                missing_password = subprocess.run([str(args.remote.resolve()), '--server', address, '-Q'],
                                                  capture_output=True, timeout=4)
                assert missing_password.returncode == 1 and b'password required' in missing_password.stderr
                checks.append({'check': 'tcp_authentication_safe_commands_and_stdout_exception', 'passed': True})
            finally:
                if hasattr(tcp, 'process') and tcp.process.poll() is None: tcp.stop()
    except Exception as error:
        checks.append({'check': 'harness_completion', 'passed': False, 'error': repr(error)})
        raise
    finally:
        if hasattr(session, 'process') and session.process.poll() is None:
            session.stop()
        report = {'passed': bool(checks) and all(row['passed'] for row in checks), 'checks': checks}
        (out / 'results.json').write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report, indent=2))
    if not report['passed']: raise SystemExit(1)


if __name__ == '__main__':
    main()

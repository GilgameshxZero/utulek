#!/usr/bin/env python3
"""Run Windows IMAP UI against a real, isolated TLS Dovecot fixture over SSH."""
import argparse
import base64
import json
from pathlib import Path
import subprocess
import time

from dovecot_fixture import DovecotFixture
from network_fixture import DisconnectProxy, DisconnectControl


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--ssh-host', required=True)
    parser.add_argument('--app', required=True, help='Installed Windows executable')
    parser.add_argument('--terminal-module', required=True, help='Existing Windows xterm-headless module')
    parser.add_argument('--dovecot-prefix', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--interrupt', action='store_true')
    parser.add_argument('--concurrent-idle',action='store_true')
    parser.add_argument('--interruption-kind', choices=['flag','delete','uid_shift','uidvalidity'], default='flag')
    args = parser.parse_args()
    root = args.output.resolve()
    root.mkdir(parents=True, exist_ok=True)
    ssh = ['ssh', '-F', '/dev/null', '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=10']
    def ps(script, timeout=180):
        command = 'powershell -NoProfile -EncodedCommand ' + base64.b64encode(
            ("$ProgressPreference='SilentlyContinue';$ErrorActionPreference='Stop';\n"+script).encode('utf-16le')).decode()
        result = subprocess.run([*ssh, args.ssh_host, command], capture_output=True, text=True, timeout=timeout)
        if result.returncode:
            raise RuntimeError(result.stdout+result.stderr)
        return result.stdout.strip()
    def quote(text):
        return "'"+text.replace("'", "''")+"'"
    home = ps('$env:USERPROFILE').replace('\\', '/')
    remote = home+'/.obj/neomutt_xp/'+root.name
    ps('New-Item -ItemType Directory -Force '+quote(remote)+' | Out-Null')
    python = ps('python -c "import sys;print(sys.executable)"')
    port = int(ps("$s=[Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,0);$s.Start();$s.LocalEndpoint.Port;$s.Stop()"))
    control_port = int(ps("$s=[Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,0);$s.Start();$s.LocalEndpoint.Port;$s.Stop()"))
    observer_port = int(ps("$s=[Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,0);$s.Start();$s.LocalEndpoint.Port;$s.Stop()"))
    with DovecotFixture(args.dovecot_prefix, root/'server') as server, DisconnectProxy(server.port) as proxy, DisconnectControl(proxy) as controls:
        settings = {'host': 'localhost', 'port': port, 'cert': remote+'/cert.pem'}
        settings.update(interrupt=args.interrupt, control_url=f'http://127.0.0.1:{control_port}', control_token=controls.token)
        settings.update(interruption_kind=args.interruption_kind, mailbox='Owned' if args.interruption_kind=='uidvalidity' else 'INBOX')
        settings['observer_port'] = observer_port
        settings['concurrent_idle_delay_ms'] = 1600 if args.concurrent_idle else 0
        connection = root/'connection.json'
        connection.write_text(json.dumps(settings)+'\n')
        scripts = Path(__file__).resolve().parent
        for source in [scripts/'check_windows_imap.js', scripts/'imap_fixture_control.py', server.cert, connection]:
            subprocess.run(['scp', '-F', '/dev/null', str(source), args.ssh_host+':'+remote+'/'+source.name], check=True)
        ps('Copy-Item '+quote(args.terminal_module)+' '+quote(remote+'/terminal.cjs'))
        with (root/'ssh_forward_errors.txt').open('w') as errors:
            forward = subprocess.Popen([*ssh, '-o', 'ExitOnForwardFailure=yes', '-N', '-R',
                f'127.0.0.1:{port}:127.0.0.1:{proxy.port}', '-R',
                f'127.0.0.1:{control_port}:127.0.0.1:{controls.port}', '-R',
                f'127.0.0.1:{observer_port}:127.0.0.1:{server.port}', args.ssh_host], stdout=subprocess.DEVNULL, stderr=errors)
            try:
                time.sleep(.5)
                if forward.poll() is not None:
                    raise RuntimeError('SSH IMAP forward failed: '+(root/'ssh_forward_errors.txt').read_text())
                command = "$env:ELECTRON_RUN_AS_NODE='1'; & 'C:/Program Files/VSCodium/VSCodium.exe' " + ' '.join(map(quote, [
                    remote+'/check_windows_imap.js', 'C:/Program Files/VSCodium/resources/app/node_modules.asar/node-pty',
                    remote+'/terminal.cjs', python, args.app, remote+'/ui', remote+'/connection.json'])) + ' | Out-Host; exit $LASTEXITCODE'
                try:
                    output = ps(command)
                finally:
                    subprocess.run(['scp','-F','/dev/null',args.ssh_host+':'+remote+'/ui/results.json',str(root/'results.json')],check=True)
                print(output)
                results = json.loads((root/'results.json').read_text())
                return 0 if len(results)==2 and all(r['passed'] for r in results) else 1
            finally:
                forward.terminate()
                try:
                    forward.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    forward.kill()
                    forward.wait()


if __name__ == '__main__':
    raise SystemExit(main())

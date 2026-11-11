#!/usr/bin/env python3
"""Run native Windows streaming checks against private HTTP/TLS fixtures.

Uses an SSH reverse forward restricted to loopback. Stages only under the test
user's .obj/cmus_xp; retrieves evidence and cleans the complete unique stage.
Requires explicit task authorization for the supplied established test host.
"""
import argparse
import base64
import http.server
import json
from pathlib import Path
import posixpath
import re
import select
import socket
import ssl
import subprocess
import threading
import time
from urllib.parse import urlsplit
import uuid
import zipfile

ROOT = Path(__file__).resolve().parent.parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--host')
    parser.add_argument('--linux', action='store_true')
    parser.add_argument('--candidate', type=Path, required=True)
    parser.add_argument('--harness', type=Path)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    out = args.output.resolve()
    out.mkdir(parents=True, exist_ok=False)
    requests, requests_lock = [], threading.Lock()
    fixtures = out / 'fixtures'
    fixtures.mkdir()
    for extension, codec in [('mp3', 'libmp3lame'), ('opus', 'libopus'), ('flac', 'flac'), ('aac', 'aac'), ('ogg', 'libvorbis')]:
        subprocess.run(['ffmpeg', '-v', 'error', '-y', '-f', 'lavfi', '-i',
            'sine=frequency=440:sample_rate=48000:duration=30', '-af', 'volume=0.01',
            '-ac', '2', '-metadata', 'title=Network tone', '-metadata', 'artist=Fixture artist',
            '-c:a', codec, str(fixtures / ('audio.' + extension))], check=True)
    audio = {'.' + ext: (fixtures / ('audio.' + ext)).read_bytes() for ext in ('mp3', 'opus', 'flac', 'aac', 'ogg')}
    native_audio = {}
    for name, layout, rate, codec in [('mono44.mp3', 'mono', 44100, 'libmp3lame'),
            ('surround51.flac', '5.1', 48000, 'flac'), ('surround71.flac', '7.1', 48000, 'flac')]:
        path = fixtures / name
        subprocess.run(['ffmpeg', '-v', 'error', '-y', '-f', 'lavfi', '-i',
            'anullsrc=sample_rate=' + str(rate) + ':channel_layout=' + layout,
            '-t', '30', '-c:a', codec, str(path)], check=True)
        native_audio['/' + name] = path.read_bytes()

    class Handler(http.server.BaseHTTPRequestHandler):
        protocol_version = 'HTTP/1.1'

        def handle(self):
            try:
                super().handle()
            except (ConnectionResetError, BrokenPipeError, ssl.SSLError):
                pass

        def log_message(self, *_):
            pass

        def do_GET(self):
            path = posixpath.normpath(urlsplit(self.path).path)
            with requests_lock:
                requests.append({'path': path, 'utc_seconds': time.time(), 'icy_requested': self.headers.get('Icy-MetaData')})
            try:
                if path == '/stall_headers':
                    time.sleep(8); self.close_connection = True; return
                if path in ('/redirect', '/redirect_loop'):
                    self.send_response(302)
                    self.send_header('Location', '/audio.mp3' if path == '/redirect' else '/redirect_loop')
                    self.send_header('Content-Length', '0'); self.end_headers(); return
                if path == '/huge.m3u':
                    body, kind = b'#' * (1024 * 1024 + 1), 'audio/x-mpegurl'
                elif path == '/nested.m3u':
                    body, kind = b'nested.m3u\n', 'audio/x-mpegurl'
                elif path == '/hls.m3u8':
                    body, kind = b'#EXTM3U\n#EXT-X-VERSION:3\naudio.mp3\n', 'audio/x-mpegurl'
                elif path == '/list.m3u8':
                    body = b'\xef\xbb\xbf#EXTM3U\r\nmissing.mp3\r\n../audio.mp3\r\n'
                    kind = 'audio/x-mpegurl; charset=UTF-8'
                elif path == '/list.pls':
                    body = b'[playlist]\r\nFile2=audio.mp3\r\nFile1=missing.mp3\r\nNumberOfEntries=2\r\n'
                    kind = 'audio/x-scpls'
                elif path in ('/audio.mp3', '/chunked.mp3', '/icy.mp3', '/legacy_icy.mp3', '/stall_body'):
                    body, kind = audio['.mp3'], 'audio/mpeg'
                elif path == '/audio.aac':
                    body, kind = audio['.aac'], 'audio/aacp'
                elif path == '/audio.ogg':
                    body, kind = audio['.ogg'], 'audio/ogg'
                elif path == '/audio.opus':
                    body, kind = audio['.opus'], 'audio/ogg'
                elif path in native_audio:
                    body = native_audio[path]
                    kind = 'audio/mpeg' if path.endswith('.mp3') else 'audio/flac'
                elif path == '/audio.flac':
                    body, kind = audio['.flac'], 'audio/flac'
                else:
                    self.send_error(404); return
                if path in ('/icy.mp3', '/legacy_icy.mp3'):
                    packed = bytearray()
                    metadata = b"StreamTitle='Fixture artist - Fixture title';"
                    metadata += b'\0' * (-len(metadata) % 16)
                    for i in range(0, len(body), 4096):
                        part = body[i:i + 4096]
                        packed.extend(part)
                        if len(part) == 4096: packed.extend(bytes([len(metadata) // 16]) + metadata)
                    body = bytes(packed)
                if path == '/legacy_icy.mp3':
                    self.wfile.write(b'ICY 200 OK\r\n')
                else:
                    self.send_response(200)
                self.send_header('Content-Type', kind)
                if path in ('/icy.mp3', '/legacy_icy.mp3'):
                    self.send_header('icy-metaint', '4096')
                    self.send_header('icy-name', 'Fixture radio')
                    self.send_header('icy-genre', 'Test genre')
                if path == '/chunked.mp3': self.send_header('Transfer-Encoding', 'chunked')
                else: self.send_header('Content-Length', str(len(body)))
                self.end_headers()
                if path == '/stall_body':
                    self.wfile.write(body[:10]); self.wfile.flush(); time.sleep(8); self.close_connection = True; return
                if path == '/chunked.mp3':
                    for i in range(0, len(body), 4096):
                        part = body[i:i + 4096]
                        self.wfile.write(('%x\r\n' % len(part)).encode() + part + b'\r\n')
                    self.wfile.write(b'0\r\n\r\n')
                else: self.wfile.write(body)
            except (BrokenPipeError, ConnectionResetError, ssl.SSLError):
                pass

    class Server(http.server.ThreadingHTTPServer):
        daemon_threads = True

    plain_server = Server(('127.0.0.1', 0), Handler)
    tls_server = Server(('127.0.0.1', 0), Handler)
    subprocess.run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes',
        '-keyout', str(out / 'test_key.pem'), '-out', str(out / 'test_cert.pem'),
        '-days', '1', '-subj', '/CN=localhost', '-addext', 'subjectAltName=IP:127.0.0.1,DNS:localhost'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True)
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    context.load_cert_chain(out / 'test_cert.pem', out / 'test_key.pem')
    tls_server.socket = context.wrap_socket(tls_server.socket, server_side=True)
    for server in (plain_server, tls_server): threading.Thread(target=server.serve_forever, daemon=True).start()
    if args.linux:
        from check_linux_streaming import run_checks
        try:
            run_checks(args.candidate.resolve(), out, plain_server.server_port, tls_server.server_port)
        finally:
            for server in (plain_server, tls_server):
                server.shutdown(); server.server_close()
            (out / 'requests.json').write_text(json.dumps(requests, indent=2) + '\n')
        return
    if not args.host or not args.harness:
        parser.error('--host and --harness required for Windows')
    common = ['ssh', '-F', '/dev/null', '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=8']

    def powershell(code, timeout=40):
        encoded = base64.b64encode(code.encode('utf-16le')).decode()
        return subprocess.run(common + [args.host, 'powershell.exe', '-NoProfile', '-NonInteractive',
            '-ExecutionPolicy', 'Bypass', '-EncodedCommand', encoded], capture_output=True, text=True, timeout=timeout)

    setup = powershell("""$ErrorActionPreference='Stop';$ProgressPreference='SilentlyContinue'
New-Item -ItemType Directory -Force -Path (Join-Path $env:USERPROFILE '.obj/cmus_xp') | Out-Null
$ports=@();for ($i=0;$i -lt 2;$i++) {$l=[Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,0);$l.Start();$ports+=$l.LocalEndpoint.Port;$l.Stop()};$ports|ConvertTo-Json
""")
    setup.check_returncode()
    ports = json.loads(setup.stdout)
    token = 'stream_' + uuid.uuid4().hex[:10]
    remote = '.obj/cmus_xp/' + token
    state = {'stage': remote, 'zip': remote + '.zip', 'ports': ports, 'host': args.host}
    (out / 'state.json').write_text(json.dumps(state, indent=2) + '\n')
    archive = out / 'tests.zip'
    with zipfile.ZipFile(archive, 'w', zipfile.ZIP_DEFLATED) as z:
        for name in ('cmus.exe', 'cmus-remote.exe'): z.write(args.candidate / name, name)
        z.write(args.harness, 'check_streaming.exe')
        z.write(out / 'test_cert.pem', 'test_cert.pem')
    tunnel = None
    stage_created = False
    report = {'passed': False}
    try:
        subprocess.run(['scp', '-F', '/dev/null', '-o', 'BatchMode=yes', str(archive), args.host + ':' + state['zip']], check=True, timeout=90)
        stage_created = True
        forwards = []
        for port, server in zip(ports, (plain_server, tls_server)):
            forwards += ['-R', '127.0.0.1:%d:127.0.0.1:%d' % (port, server.server_port)]
        tunnel_log = (out / 'tunnel.log').open('w+')
        tunnel = subprocess.Popen(common + ['-o', 'ExitOnForwardFailure=yes', '-v', '-N', *forwards, args.host],
                                  stdout=subprocess.DEVNULL, stderr=tunnel_log)
        end = time.monotonic() + 10
        while time.monotonic() < end:
            tunnel_log.flush()
            if (out / 'tunnel.log').read_text().count('remote forward success for:') >= 2: break
            if tunnel.poll() is not None: raise RuntimeError('SSH reverse forwarding failed')
            time.sleep(.05)
        else: raise TimeoutError('SSH reverse forwarding readiness')
        code = r'''$ErrorActionPreference='Stop';$ProgressPreference='SilentlyContinue'
$dir=Join-Path $env:USERPROFILE 'STAGE';$zip=Join-Path $env:USERPROFILE 'STAGE.zip'
Expand-Archive -LiteralPath $zip -DestinationPath $dir
$work=Join-Path $dir 'work';New-Item -ItemType Directory -Path $work | Out-Null
$start=[Diagnostics.ProcessStartInfo]::new();$start.FileName=Join-Path $dir 'check_streaming.exe';$start.UseShellExecute=$false;$start.CreateNoWindow=$true
$start.Arguments='"'+(Join-Path $dir 'cmus.exe')+'" "'+(Join-Path $dir 'cmus-remote.exe')+'" "'+$work+'" "'+(Join-Path $dir 'report.txt')+'" http://127.0.0.1:HTTP_PORT https://127.0.0.1:TLS_PORT'
$p=[Diagnostics.Process]::Start($start);if (!$p.WaitForExit(150000)) {$p.Kill();throw 'Streaming harness timeout'}
$code=$p.ExitCode;$p.Dispose();Write-Output "Streaming native harness exit: $code";exit $code
'''.replace('STAGE', remote).replace('HTTP_PORT', str(ports[0])).replace('TLS_PORT', str(ports[1]))
        result = powershell(code, timeout=180)
        (out / 'windows_run.log').write_text(result.stdout + '\n' + result.stderr)
        subprocess.run(['scp', '-F', '/dev/null', '-o', 'BatchMode=yes', args.host + ':' + remote + '/report.txt', str(out / 'report.txt')], check=True, timeout=30)
        text = (out / 'report.txt').read_text(errors='replace')
        match = re.search(r'streaming checks: (\d+); result: PASS', text)
        report.update(passed=result.returncode == 0 and match is not None and ': FAIL' not in text,
                      native_checks=int(match[1]) if match else None, exit=result.returncode)
        print(text)
    finally:
        if stage_created:
            cleanup = powershell(r'''$ErrorActionPreference='Stop';$ProgressPreference='SilentlyContinue'
$dir=Join-Path $env:USERPROFILE 'STAGE';$zip=Join-Path $env:USERPROFILE 'STAGE.zip';$prefix=[IO.Path]::GetFullPath($dir)+'\'
$owned=@(Get-CimInstance Win32_Process|Where-Object {$_.ExecutablePath -and $_.ExecutablePath.StartsWith($prefix,[StringComparison]::OrdinalIgnoreCase)})
foreach($p in $owned) {Stop-Process -Id $p.ProcessId -Force}
if (Test-Path $dir) {Remove-Item -LiteralPath $dir -Recurse -Force};if(Test-Path $zip){Remove-Item -LiteralPath $zip -Force}
if ((Test-Path $dir) -or (Test-Path $zip)){throw 'Streaming cleanup failed'}
@{passed=$true;scratch_removed=$true;stopped_test_processes=$owned.Count}|ConvertTo-Json
'''.replace('STAGE', remote))
            (out / 'cleanup.log').write_text(cleanup.stdout + '\n' + cleanup.stderr)
            cleanup.check_returncode()
            report['cleanup'] = json.loads(cleanup.stdout)
        if tunnel:
            tunnel.terminate(); tunnel.wait(timeout=5); tunnel_log.close()
        for server in (plain_server, tls_server): server.shutdown(); server.server_close()
        (out / 'requests.json').write_text(json.dumps(requests, indent=2) + '\n')
        (out / 'results.json').write_text(json.dumps(report, indent=2) + '\n')
    print(out / 'results.json')
    if not report['passed']: raise SystemExit(1)


if __name__ == '__main__':
    main()

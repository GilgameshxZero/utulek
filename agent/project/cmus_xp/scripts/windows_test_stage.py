"""Unique, bounded Windows test staging on an explicitly authorized SSH host."""
import base64
import json
from pathlib import Path
import subprocess
import uuid
import zipfile


def ps_quote(value):
    return "'" + str(value).replace("'", "''") + "'"


class WindowsStage:
    def __init__(self, host, files, out):
        self.host, self.files, self.out = host, files, Path(out)
        self.relative = '.obj/cmus_xp/check_' + uuid.uuid4().hex[:12]
        self.common = ['-F', '/dev/null', '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=8']
        self.created = False

    def powershell(self, code, timeout=45):
        command = ['$ErrorActionPreference="Stop";', code]
        encoded = base64.b64encode('\n'.join(command).encode('utf-16le')).decode()
        return subprocess.run(['ssh', *self.common, self.host, 'powershell.exe',
            '-NoProfile', '-NonInteractive', '-EncodedCommand', encoded],
            capture_output=True, text=True, timeout=timeout)

    def __enter__(self):
        archive = self.out / 'upload.zip'
        with zipfile.ZipFile(archive, 'w', zipfile.ZIP_DEFLATED) as z:
            for name, source in self.files.items():
                z.write(source, name)
        try:
            setup = self.powershell("$ProgressPreference='SilentlyContinue';"
                "New-Item -ItemType Directory -Force (Join-Path $env:USERPROFILE '.obj/cmus_xp')|Out-Null")
            setup.check_returncode()
            self.created = True
            subprocess.run(['scp', *self.common, str(archive), self.host + ':' + self.relative + '.zip'], check=True, timeout=90)
            setup = self.powershell("$ProgressPreference='SilentlyContinue';$stage=Join-Path $env:USERPROFILE " +
                ps_quote(self.relative) + ";Expand-Archive -LiteralPath ($stage+'.zip') -DestinationPath $stage;" +
                '$stage|ConvertTo-Json')
            setup.check_returncode()
            self.path = json.loads(setup.stdout).replace('\\', '/')
            (self.out / 'stage.json').write_text(json.dumps({'path': self.path, 'host': self.host}, indent=2) + '\n')
            return self
        except BaseException:
            self.__exit__(None, None, None)
            raise

    def run(self, executable, arguments, timeout=150):
        # Arguments are Windows paths/URLs supplied by our fixture, without
        # embedded quotes. PowerShell passes the complete command line once.
        if any('"' in str(arg) for arg in arguments):
            raise ValueError('Embedded quote in test argument')
        command_line = ' '.join('"' + str(arg) + '"' for arg in arguments)
        code = ('$p=[Diagnostics.ProcessStartInfo]::new();$p.FileName=' +
            ps_quote(self.path + '/' + executable) + ';$p.UseShellExecute=$false;$p.CreateNoWindow=$true;$p.Arguments=' +
            ps_quote(command_line) + ';$child=[Diagnostics.Process]::Start($p);' +
            'if (!$child.WaitForExit(' + str(timeout * 1000) + ")) {$child.Kill();throw 'Test timeout'};" +
            '$code=$child.ExitCode;$child.Dispose();exit $code')
        return self.powershell(code, timeout=timeout + 15)

    def retrieve(self, name, destination):
        subprocess.run(['scp', *self.common, self.host + ':' + self.relative + '/' + name,
                        str(destination)], check=True, timeout=45)

    def __exit__(self, *_):
        if not self.created:
            return
        cleanup = self.powershell('$stage=Join-Path $env:USERPROFILE ' + ps_quote(self.relative) + r''';
$stopped=0;Get-CimInstance Win32_Process|Where-Object {$_.ExecutablePath -and $_.ExecutablePath.StartsWith($stage,[StringComparison]::OrdinalIgnoreCase)}|ForEach-Object {Stop-Process -Id $_.ProcessId -Force;$stopped++};
if(Test-Path -LiteralPath $stage){Remove-Item -LiteralPath $stage -Recurse -Force};
if(Test-Path -LiteralPath ($stage+'.zip')){Remove-Item -LiteralPath ($stage+'.zip') -Force};
@{removed=(!(Test-Path -LiteralPath $stage)) -and (!(Test-Path -LiteralPath ($stage+'.zip')));stopped_test_processes=$stopped}|ConvertTo-Json''')
        cleanup.check_returncode()
        report = json.loads(cleanup.stdout)
        (self.out / 'cleanup.json').write_text(json.dumps(report, indent=2) + '\n')
        assert report['removed'], report

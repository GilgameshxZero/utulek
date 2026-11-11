param([string]$Root = $PSScriptRoot)
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$cases = Get-Content (Join-Path $Root 'cases.json') -Raw | ConvertFrom-Json
$results = @()
foreach ($case in $cases) {
    $arguments = '"' + (Join-Path $Root $case.input) + '" -'
    if ($case.reference) { $arguments += ' "' + (Join-Path $Root $case.reference) + '"' }
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = Join-Path $Root 'check_decoder.exe'
    $start.Arguments = $arguments
    $start.UseShellExecute = $false
    $start.CreateNoWindow = $true
    $start.RedirectStandardOutput = $true
    $start.RedirectStandardError = $true
    $process = [Diagnostics.Process]::Start($start)
    if (!$process.WaitForExit(30000)) { $process.Kill(); throw "Decoder timeout: $($case.file)" }
    $code = $process.ExitCode
    $stdout = $process.StandardOutput.ReadToEnd()
    $stderr = $process.StandardError.ReadToEnd()
    $passed = $code -eq $case.expected_exit
    $metadata = $null
    if ($passed -and $case.expected_exit -eq 0) {
        $metadata = $stdout | ConvertFrom-Json
        $passed = $metadata.reference_ok -and $metadata.seek_ok -and $metadata.restart_ok -and
                  $metadata.restart_pcm_ok -and $metadata.invalid_seek_ok
        if ($case.title_expected) { $passed = $passed -and $metadata.title_ok }
        if ($case.output_channels_expected) { $passed = $passed -and ($metadata.channels -eq $case.output_channels_expected) }
    }
    $results += @{ file=$case.file; exit=$code; passed=[bool]$passed; metadata=$metadata;
                   stderr=$(if (!$passed) { $stderr } else { '' }) }
    $process.Dispose()
}
$results | ConvertTo-Json -Depth 6 | Set-Content (Join-Path $Root 'results.json') -Encoding UTF8
$failed = @($results | Where-Object { !$_.passed })
Write-Output "Windows decoder cases: $($results.Count); failures: $($failed.Count)"
if ($failed.Count) { $failed | ConvertTo-Json -Depth 6; exit 1 }

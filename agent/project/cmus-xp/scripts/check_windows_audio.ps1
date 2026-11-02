param([Parameter(Mandatory=$true)][string]$Directory)
$ErrorActionPreference = 'Stop'
$cases = Get-Content -Raw -Encoding UTF8 (Join-Path $Directory 'manifest.json') | ConvertFrom-Json
$results = @()
foreach ($case in $cases) {
    $output = Join-Path $Directory 'decoded.pcm'
    $arguments = @((Join-Path $Directory $case.file), $output)
    if ($case.reference) { $arguments += (Join-Path $Directory $case.reference) }
    $json = & (Join-Path $Directory 'check_decoder.exe') @arguments
    $code = $LASTEXITCODE
    $report = if ($json) { $json | ConvertFrom-Json } else { $null }
    $results += [pscustomobject]@{
        name = $case.name
        exit = $code
        passed = ($code -eq $case.expected_exit)
        report = $report
    }
}
Remove-Item -Force -ErrorAction SilentlyContinue (Join-Path $Directory 'decoded.pcm')
$results | ConvertTo-Json -Depth 5 | Set-Content -Encoding UTF8 (Join-Path $Directory 'results.json')
$results | Select-Object name,passed,exit | Format-Table -AutoSize
if (@($results | Where-Object { -not $_.passed }).Count) { exit 1 }
exit 0

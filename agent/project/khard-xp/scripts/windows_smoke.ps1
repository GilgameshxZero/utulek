param(
    [Parameter(Mandatory=$true)][string]$Executable,
    [string]$Launcher = ''
)
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
$utf8 = New-Object System.Text.UTF8Encoding($false)
$root = Join-Path ([IO.Path]::GetTempPath()) ('khard-xp smoke ' + [guid]::NewGuid())
$book = Join-Path $root 'Address Book with spaces'
New-Item -ItemType Directory -Path $book -Force | Out-Null
$config = Join-Path $root 'khard.conf'
$destination = Join-Path $root 'Destination with spaces'
New-Item -ItemType Directory -Path $destination -Force | Out-Null
$inputFile = Join-Path $root 'new contact.yaml'
$editFile = Join-Path $root 'edit contact.yaml'
$vcfFile = Join-Path $root 'exported contact.vcf'
$oldConfig = $env:KHARD_CONFIG
$checks = New-Object System.Collections.Generic.List[string]

function Invoke-Khard([string[]]$Arguments) {
    $output = & $Executable @Arguments 2>&1
    if ($LASTEXITCODE -ne 0) { throw "khard failed ($LASTEXITCODE): $output" }
    return ($output -join "`n")
}

try {
    [IO.File]::WriteAllText($config, "[addressbooks]`n[[smoke]]`npath = '$book'`n[[destination]]`npath = '$destination'`n[general]`ndefault_action = list`neditor = vim, -i, NONE`n[contact table]`ndisplay = formatted_name`nsort = formatted_name`n[vcard]`npreferred_version = 3.0`n", $utf8)
    $env:KHARD_CONFIG = $config
    $unicodeName = 'Windows Test ' + [char]0x96ea
    [IO.File]::WriteAllText($inputFile, "First name: Windows`nLast name: 'Test $([char]0x96ea)'`nPhone:`n    `"`": '+15550100009'`nEmail:`n    work: windows@example.invalid`nNote: smoke fixture only`n", $utf8)
    $version = Invoke-Khard -Arguments @('--version')
    if ($version -notmatch 'khard-xp') { throw 'version missing' }
    $checks.Add('native executable starts')
    $null = Invoke-Khard -Arguments @('new', '-a', 'smoke', '-i', $inputFile)
    $rows = Invoke-Khard -Arguments @('list', '-p', 'name:Windows', 'name:Test')
    if (-not $rows.Contains($unicodeName)) { throw "Unicode/AND-search failed: $rows" }
    $checks.Add('same-format config, environment lookup, Unicode and AND search')
    $phone = Invoke-Khard -Arguments @('phone', '-p', 'name:Windows')
    if (-not $phone.Contains('+15550100009')) { throw 'phone lookup failed' }
    $checks.Add('unlabeled phone lookup')
    $null = Invoke-Khard -Arguments @('show', '--format', 'vcard', '-o', $vcfFile, 'name:Windows')
    $original = [IO.File]::ReadAllText($vcfFile, $utf8)
    if (-not $original.Contains('TEL:+15550100009') -or $original.Contains('TYPE=cell')) { throw 'inferred phone label' }
    [IO.File]::WriteAllText($editFile, "Note: updated by smoke test`n", $utf8)
    $null = Invoke-Khard -Arguments @('edit', '-i', $editFile, 'name:Windows')
    $shown = Invoke-Khard -Arguments @('show', '--format', 'vcard', 'name:Windows')
    if (-not $shown.Contains('NOTE:updated by smoke test') -or -not $shown.Contains('TEL:+15550100009')) { throw 'edit lost fields' }
    $checks.Add('atomic edit preserves phone and unrelated fields')
    if ($Launcher -ne '') {
        $launchOutput = & $Launcher -c $config list -p 'name:Windows' 'name:Test' 2>&1
        if ($LASTEXITCODE -ne 0 -or -not ($launchOutput -join "`n").Contains($unicodeName)) { throw "launcher did not forward arguments: $launchOutput" }
        $ErrorActionPreference = 'Continue'
        $null = & $Launcher -c $config show 'name:missing' 2>&1
        $ErrorActionPreference = 'Stop'
        if ($LASTEXITCODE -ne 1) { throw 'launcher lost failure exit code' }
        $checks.Add('batch launcher forwards spaced path, multiple arguments, exit code')
    }
    $null = Invoke-Khard -Arguments @('copy', '-a', 'smoke', '-A', 'destination', 'name:Windows')
    if ((Get-ChildItem -LiteralPath $destination -Filter '*.vcf').Count -ne 1) { throw 'copy failed' }
    $null = Invoke-Khard -Arguments @('remove', '--force', '-a', 'destination', 'name:Windows')
    $null = Invoke-Khard -Arguments @('move', '-a', 'smoke', '-A', 'destination', 'name:Windows')
    if ((Get-ChildItem -LiteralPath $book -Filter '*.vcf').Count -ne 0) { throw 'move retained source' }
    $null = Invoke-Khard -Arguments @('move', '-a', 'destination', '-A', 'smoke', 'name:Windows')
    $mailFile = Join-Path $root 'message header.eml'
    [IO.File]::WriteAllText($mailFile, "From: Windows <windows@example.invalid>`n", $utf8)
    $null = Invoke-Khard -Arguments @('add-email', '-a', 'smoke', '-i', $mailFile, '--skip-already-added')
    $checks.Add('copy, move round trip and add-email skip through executable')
    $null = Invoke-Khard -Arguments @('remove', '--force', 'name:Windows')
    if ((Get-ChildItem -LiteralPath $book -Filter '*.vcf').Count -ne 0) { throw 'remove left a contact' }
    $checks.Add('remove fixture contact')
    [pscustomobject]@{passed=$true;checks=$checks.ToArray();version=$version} | ConvertTo-Json -Depth 4
} finally {
    $env:KHARD_CONFIG = $oldConfig
    Remove-Item -LiteralPath $root -Recurse -Force
}

param([Parameter(Mandatory=$true)][string]$Executable)
$ErrorActionPreference='Stop'
$ProgressPreference='SilentlyContinue'
[Console]::OutputEncoding=New-Object System.Text.UTF8Encoding($false)
$utf8=New-Object System.Text.UTF8Encoding($false)
$root=Join-Path ([IO.Path]::GetTempPath()) ('khard-xp locale '+[guid]::NewGuid())
$book=Join-Path $root 'Chinese contacts'
$oldLocale=$env:LC_ALL
$results=New-Object System.Collections.Generic.List[object]
New-Item -ItemType Directory -Path $book -Force | Out-Null
try {
 $names=ConvertFrom-Json '["\u674e\u660e","\u738b\u5c0f\u660e","\u5f35\u7f8e\u83ef","\u9648\u4f1f","\u9673\u5049","\u8d75\u654f","\u8d99\u654f","\u94b1\u5b66\u68ee","Zoe"]'
 for($i=0;$i -lt $names.Count;$i++) {
  $name=$names[$i]
  $card="BEGIN:VCARD`r`nVERSION:4.0`r`nUID:locale-$i`r`nN:$($name.Substring(0,1));$($name.Substring(1));;;`r`nFN:$name`r`nBDAY:19900403`r`nEND:VCARD`r`n"
  [IO.File]::WriteAllText((Join-Path $book "$i.vcf"),$card,$utf8)
 }
 $config=Join-Path $root 'khard.conf'
 [IO.File]::WriteAllText($config,"[addressbooks]`n[[test]]`npath = '$book'`n[contact table]`ndisplay = formatted_name`nsort = formatted_name`nlocalize_dates = yes`n[vcard]`nsearch_in_source_files = no`n",$utf8)
 foreach($locale in @('zh-CN','zh-TW','sv-SE','en-US')) {
  $env:LC_ALL=$locale
  $culture=New-Object Globalization.CultureInfo($locale)
  [string[]]$expected=$names.Clone()
  [Array]::Sort($expected,[StringComparer]::Create($culture,$true))
  $rows=@(& $Executable -c $config list -p)
  if($LASTEXITCODE -ne 0) {throw "locale executable failed: $locale"}
  $actual=@($rows | ForEach-Object {($_ -split "`t")[1]})
  if(($actual -join "`n") -cne ($expected -join "`n")) {throw "native Windows collation differs from .NET NLS oracle: $locale"}
  $date=(New-Object DateTime(1990,4,3)).ToString('d',$culture)
  $shown=(& $Executable -c $config show 'uid:locale-0') -join "`n"
  if($LASTEXITCODE -ne 0 -or -not $shown.Contains($date)) {throw "localized date mismatch for $locale expected $date"}
  $results.Add([pscustomobject]@{locale=$locale;ordered_names=$actual;date=$date;passed=$true})
 }
 # Exercise Chinese custom labels through the actual command and YAML import.
 $env:LC_ALL='zh-CN'
 $label=ConvertFrom-Json '"\u5de5\u4f5c"'
 $inputFile=Join-Path $root 'Chinese edit.yaml'
 [IO.File]::WriteAllText($inputFile,"Email:`n    ${label}: chinese@example.invalid`n",$utf8)
 $null=& $Executable -c $config edit --patch -i $inputFile 'uid:locale-0'
 if($LASTEXITCODE -ne 0) {throw 'Chinese label edit failed'}
 $emails=(& $Executable -c $config email -p 'uid:locale-0') -join "`n"
 if($LASTEXITCODE -ne 0 -or -not $emails.Contains($label)) {throw 'Chinese label was lost'}
 [pscustomobject]@{passed=$true;native_locale_checks=$results.ToArray();chinese_label_edit=$true} | ConvertTo-Json -Depth 5
} finally {
 $env:LC_ALL=$oldLocale
 Remove-Item -LiteralPath $root -Recurse -Force -ErrorAction SilentlyContinue
}

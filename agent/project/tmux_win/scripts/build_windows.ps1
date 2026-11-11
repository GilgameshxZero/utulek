# Build the pinned tmux_win source + console wrap patch using existing MSVC.
param([Parameter(Mandatory=$true)][string]$Root)
$ErrorActionPreference='Stop'
$cmake='C:/Program Files/Microsoft Visual Studio/2022/Community/Common7/IDE/CommonExtensions/Microsoft/CMake/CMake/bin/cmake.exe'
if(!(Test-Path $cmake)){$cmake="$Root/cmake-3.31.8-windows-x86_64/bin/cmake.exe"}
if(!(Test-Path $cmake)){throw 'Stage official portable CMake or install the VS CMake component'}
function Invoke-CMake([string[]]$Arguments,[string]$Log){
 $previousPreference=$ErrorActionPreference
 $ErrorActionPreference='Continue'
 & $cmake @Arguments *> "$Root/$Log"
 $code=$LASTEXITCODE
 $ErrorActionPreference=$previousPreference
 if($code -ne 0){Get-Content "$Root/$Log" -Tail 35;throw "CMake failed: $Log"}
}
New-Item -ItemType Directory -Force $Root | Out-Null
$dependencies=Get-Content "$PSScriptRoot/../dependencies.json" -Raw | ConvertFrom-Json
foreach($item in $dependencies){
 $file="$Root/$($item.archive)"
 if(!(Test-Path $file)){throw "Stage pinned development input $($item.url) in $Root"}
 if((Get-FileHash $file -Algorithm SHA256).Hash.ToLowerInvariant() -ne $item.sha256){throw "Source hash mismatch: $file"}
}
tar -xzf "$Root/libevent-2.1.12-stable.tar.gz" -C $Root
if($LASTEXITCODE -ne 0){throw 'libevent extraction failed'}
New-Item -ItemType Directory -Force "$Root/tmux-source" | Out-Null
tar -xzf "$Root/source.tar.gz" --strip-components=1 -C "$Root/tmux-source"
if($LASTEXITCODE -ne 0){throw 'tmux extraction failed'}
# Equivalent to patches/console_wrap.patch; assert the pinned source.
$tty="$Root/tmux-source/win32/win32-tty.c"
$text=[IO.File]::ReadAllText($tty)
$before="out_mode = ENABLE_PROCESSED_OUTPUT |`n"
$after="out_mode = ENABLE_PROCESSED_OUTPUT |`n`t    ENABLE_WRAP_AT_EOL_OUTPUT |`n"
if(!$text.Contains($before)){throw 'Unexpected pinned tmux console mode source'}
[IO.File]::WriteAllText($tty,$text.Replace($before,$after),(New-Object Text.UTF8Encoding($false)))
Invoke-CMake @('-S',"$Root/libevent-2.1.12-stable",'-B',"$Root/libevent-build",'-G','Visual Studio 17 2022','-A','x64',"-DCMAKE_INSTALL_PREFIX=$Root/libevent-prefix",'-DEVENT__LIBRARY_TYPE=STATIC','-DEVENT__MSVC_STATIC_RUNTIME=ON','-DEVENT__DISABLE_OPENSSL=ON','-DEVENT__DISABLE_TESTS=ON','-DEVENT__DISABLE_REGRESS=ON','-DEVENT__DISABLE_SAMPLES=ON','-DEVENT__DISABLE_BENCHMARK=ON') 'configure-libevent.log'
Invoke-CMake @('--build',"$Root/libevent-build",'--config','Release','--target','install','--parallel','4') 'build-libevent.log'
Invoke-CMake @('-S',"$Root/tmux-source",'-B',"$Root/tmux-build",'-G','Visual Studio 17 2022','-A','x64',"-DCMAKE_PREFIX_PATH=$Root/libevent-prefix") 'configure-tmux.log'
Invoke-CMake @('--build',"$Root/tmux-build",'--config','Release','--parallel','4') 'build-tmux.log'
Get-FileHash "$Root/tmux-build/Release/tmux.exe" | Select-Object Path,Hash | ConvertTo-Json

# cmus_xp

A native Windows port of cmus, preserving its terminal interface, commands,
library, queue, and playback engine. Version 0.4 adds optimized decoding,
expanded codecs, and tested audio playback from video containers.
Current version: **0.9.0-dev**. Version 0.5 adds file-backed playlist imports and `:update` synchronization;
see the [shared README](README.md) for usage and supported playlist formats.

## Run the preview

The local build stages `.bin/cmus.exe` and `.bin/cmus-remote.exe`, then copies
them to `monochrome/app/`, with argument-forwarding `cmus.bat` and
`cmus-remote.bat` launchers in `monochrome/bin/`. Run the launcher in a Windows console or
Windows Terminal. It needs no Cygwin, MSYS2, compiler installation, application
DLLs, or adjacent theme/configuration files. Its DLL imports are Windows system
components, including the Universal CRT provided with Windows 10 and later.
Only x64 has been built and checked.

Defaults and bundled themes are embedded. User configuration is stored in
`%APPDATA%/cmus-win`; `CMUS_HOME` can select another existing configuration
directory. Configuration and library files are separate from upstream cmus's
normal directories.

Use cmus's normal commands and bindings: `:add` followed by an audio file or music
folder imports tracks; `2` shows the sorted library; `x` plays, `c` pauses,
`v` stops, and `:quit` exits. For now, write Windows paths with forward slashes,
for example `C:/Users/example/Music`. Software volume is enabled by default
because the WaveOut backend does not expose a hardware mixer.

Supported common formats include **Opus** (`.opus`, and Opus inside `.ogg`),
MP3, FLAC, Ogg Vorbis, AAC/M4A/M4B, ALAC, WAV, AIFF, WMA, and WavPack.
Additional decoders include DTS, TrueHD/MLP, ADPCM, Speex, AMR, GSM, ATRAC3/3+,
Shorten, DSD, APE, TTA, TAK, Musepack, and AC-3/E-AC-3. Local MKV/MKA, WebM,
MP4, AVI, MPEG-TS, MPEG program streams, FLV, RealMedia and OMA containers are enabled. MKV playback
selects the default decodable audio track and discards unused video, subtitles,
and alternate audio; video is never decoded or displayed. FFmpeg surround
audio retains its native decoded channel layout. Windows may remix that PCM
to the selected playback device. Track
selection is automatic; there is no manual audio-track selector yet.
Not every enabled codec/container has fixtures. DRM-protected files are unsupported.
FFmpeg's libraries are linked into the executable; no separate FFmpeg
installation or codec DLLs are required.

## What has been checked

The original WAV-only executable was cross-compiled and run on Windows. An isolated console
harness exercised startup, importing a WAV with an accented/CJK filename,
playback, pause, seeking forward, resume, stop, and clean exit. The captured
screen retained the Unicode filename and box-drawing characters. The fixture
was silent: audio submission and playback progress were exercised, not audible
quality. PE import inspection found only Windows system DLL dependencies.

Local checks use the shared FFmpeg plugin and a reference tool built from the
same pinned FFmpeg release. The expanded 56-case suite covers the common formats
plus TTA, AC-3/E-AC-3, DTS, TrueHD/MLP, SBC, Speex, ADPCM, A-law/mu-law,
floating-point/32-bit PCM, high-rate resampling, surround audio, actual video
containers and default-track selection. Checks compare decoded lengths and PCM,
tags where supported, seeking, exact initial samples after restarting, and
malformed/audio-free file rejection. The native-channel suite also passes on Windows;
isolated native-console MKV/Opus playback checks pass. Native Linux channel
policy preserves surround output. Reports stay under ignored `.obj/`.

The conversion buffer grows to fit decoded frames instead of reserving ten
seconds of PCM per open track. Matching packed PCM bypasses conversion, and
resampler tails are drained at EOF and reset when seeking. FFmpeg uses runtime
CPU dispatch with NASM SIMD routines enabled. The portable C implementation
remains available through the explicit `--disable-x86asm` build option.
`scripts/check_decoder_efficiency.py` compares saved baseline/candidate checkers
using alternating runs on generated audio; results describe decoding throughput,
not hardware playback latency or power consumption.

The user confirmed folder import, audible Opus playback, playback/repeat modes,
library persistence, seeking, pause/stop, and volume on Windows.

`scripts/check_reliability.py` runs address/undefined-behavior sanitizer checks
against production code: queue prepend/FIFO, a 50,000-track sorted index,
ReplayGain selection/fallback/clipping, 16/24/32-bit scaling, deep-directory
scanning, recursion-link detection, and malformed WAV rejection. Its WinMM
model exercises the actual WaveOut backend for gapless same-format queuing,
complete track endings, stop/seek flushing, paused drains, buffer-option safety,
device errors, and stalled drivers. Reports are retained in `.obj/reliability/`.
LeakSanitizer is disabled because this execution environment uses tracing;
address and undefined-behavior sanitizers remain enabled.

The native path layer uses dynamic UTF-8 names, extended-length Win32 paths,
and Win32 directory enumeration. It preserves drive/UNC roots, avoids following
directory junction loops, and no longer silently skips paths above 1024 bytes.
Terminal resize handling reads the active PDCurses buffer rather than resizing
back to the original console buffer. Legacy 8/16-bit mono PCM is converted to stereo
so software volume and ReplayGain apply to mono Opus as well.

Windows display output uses `windows/curses_vt.c` with PDCurses' input and
screen model. It emits explicitly positioned VT spans instead of mixing
`WriteConsoleOutput` rectangles and cursor writes. The old display backend
could leave a diagonal divider, missing rows, and stale content at startup,
resize, or tmux reattachment through nested ConPTY hosts. The VT adapter fixes
those earlier empty-screen failures against the unchanged Windows tmux executable. Colors
configured as `default` retain the terminal's foreground/background (VT 39/49),
including custom and light terminal themes; explicit cmus theme colors still
use the terminal palette.

The native build uses ANSI color indexes (`PDC_RGB`) so named and numeric theme
colors have the same meaning as upstream cmus. Its private static PDCurses ABI
stores Unicode scalars, combining clusters, and wide-character continuation
cells instead of treating every UTF-16 value as one screen column. The display
adapter emits bold, reverse, italic and underline attributes. Canonical Unicode
composition prevents decomposed accents from consuming extra legacy console
cells; user theme/configuration files are not rewritten as a workaround.

Populated album-heading text and visible color/style cells match an isolated
build of the pinned upstream revision in 30 direct system/editor ConPTY checks,
including CJK, supplementary symbols, combining accents, theme changes,
selection, shortening headings, switching views and resizing. The previous
binary fails all 20 baseline checks. These checks use the terminal parser's
Unicode 11 width provider. Additional tests inside the existing Windows tmux
still show wide-character partial-redraw differences; full populated tmux
rendering parity is not established. The tmux binary/settings remain unchanged.
Reusable capture/comparison helpers are `windows/check_album_render.js`,
`scripts/capture_linux_albums.py`, and `scripts/check_album_captures.js`.

The executable also has a read-only `--check-runtime` diagnostic: drive/UNC
canonicalization, long-path and Unicode conversion, a system DLL read/map,
and system-directory enumeration. It bypasses normal initialization and writes
no configuration, fixtures, or reports. Windows diagnostics can run against the
existing synced installation without uploading test files. The installed 0.5
Windows batch launcher and these diagnostics passed, with its binary SHA-256
matching the local build.

The executable imports only Windows system/UCRT DLLs. Physical audio-device
unplugging/reconnection and audible gapless transitions have not been observed
on hardware; driver failures and sample continuity are checked using the model.
Gapless checks cover matching output formats; format changes drain and reopen
the output device. Actual Windows native-console and ConPTY tests import 50,000 distinct generated
WAV files, export the complete library, search and queue tracks, reshuffle,
navigate and resize, restart and export again, then update after a file is
removed. These are generated fixtures, not a user music collection. A previous
stress-harness failure was caused by deleting each asynchronous export before
reading it; the corrected harness invalidates the old export once and polls
the fresh result. No player import defect was inferred from that harness bug.

The harness lives in `windows/check_console.c`. It creates a fresh configuration
directory beside its report, sends real console key events, and captures the
application's console buffer. It accepts executable, WAV path, and report path
as its three arguments. It is not part of the distributed player.
It also checks divider alignment while shrinking and enlarging the console.
`windows/check_terminal_surfaces.js` runs an isolated startup/resize/reattach
matrix through the editor's bundled node-pty, using both system and bundled
ConPTY, directly and inside tmux. Its JSONL captures can be checked by
`scripts/check_terminal_captures.js` with `@xterm/headless`. These development
harnesses and parser dependencies are not runtime requirements.

## Remote control

Windows now builds the shared `cmus-remote` client. Its cooked playback/volume/
seek controls, raw commands, stdin commands, status queries, searching, option
queries and stdout playlist exports use the same handlers as Linux. Command-line
arguments and local paths use UTF-8, including non-ASCII Windows arguments.

The default endpoint follows `CMUS_SOCKET`, otherwise the normal configuration
socket path. Windows maps local endpoint paths to same-user named pipes; it does
not create a filesystem socket. Both player `--listen` and client `--server`
accept a custom local path or an explicit local named-pipe path. Distinct
configuration paths can run independently; a second player on the same endpoint
is rejected. Pipe access is restricted to the current user and local machine,
and the client verifies the server's user identity.

Both platforms also accept a TCP host/port endpoint. TCP retains upstream's
`passwd` configuration option, client `--passwd` authentication and restrictions
on unsafe commands, including the exception permitting stdout playlist exports.
The protocol and password are not encrypted; local endpoints are the default.

Native-console checks cover actual remote playback, pause, seek, resume, stop,
volume, UTF-8 imports, exports, keyboard equivalence, restart/reconnect, duplicate
instances, fragmented commands, long raw/stdin commands and TCP authentication.
Large responses are queued so an unread export does not stall the UI or other
clients. Input commands are limited to 1 MiB and queued replies to 32 MiB per
client, with up to ten connected clients.

## Current limits

* WaveOut output accepts mono and native multichannel PCM; the selected device
  and Windows audio engine determine the final hardware rendering.
  Native output now retains representable WAVE speaker layouts up to 18 channels.
* HTTP(S) radio uses asynchronous decoding and cancellable requests; HLS and
  stream seeking remain unsupported. Remote control is implemented.
* Device failures now report an error instead of silently spinning. The player
  attempts to reopen output; if no device is available, playback can be retried
  after reconnecting. Physical hotplug behavior remains hardware-dependent.
* The Win32 process launcher is basic; Unix shell commands and scripts are not
  translated. Non-UTF-8 tag conversion needs a Windows implementation.
* Terminal behavior across hosts and mouse interactions are not exhaustively
  covered. No additional manual testing is requested for this milestone.

## Build

`scripts/build_windows_release.py` is the single build entry point. Run it with
Python 3.11+ on Linux x64 with GCC and GNU make installed. It downloads pinned
archives, verifies every SHA-256, extracts safely under ignored `.obj/deps/`,
builds the audio libraries and player, and installs the executable and wrapper.
`--offline` requires the archive cache and performs no network access;
`--prepare-only` verifies/extracts dependencies and builds the local NASM tool. Dependency
pins are in `scripts/windows_dependencies.json`. A checksum mismatch stops the
build; unverified downloads are never extracted.

`scripts/build_native_windows.py` is the lower-level cross-build using a supplied
LLVM-MinGW toolchain and PDCurses source. The compiler defaults to
`.obj/deps/llvm_mingw` and can be selected with `--toolchain`; PDCurses defaults to
`.obj/deps/pdcurses` and can be selected with `--pdcurses`.
FFmpeg source belongs in the ignored `.obj/deps/ffmpeg`. Build the audio-only
static libraries with `scripts/build_audio_deps.py` before building the player.
They install under ignored `.obj/deps/audio_windows`; `--audio-libs` selects
another library prefix. The dependency build includes only local-file audio
decoders, parsers, and containers, without networking, video decoders, or tools.
Python 3 and a working LLVM-MinGW installation are build-time
requirements only. All compilation, dependencies, and intermediate outputs stay
in this local project. The completed executable is moved to the local
`monochrome/app/cmus.exe`; `--install-dir` can select another local destination.
The build also writes `monochrome/bin/cmus.bat`; `--launcher-dir` can select
another launcher directory. The wrapper quotes the executable path, forwards
all arguments, preserves its exit code, and keeps the caller's working directory.
The lower-level build does not download dependencies. None of the build scripts
accesses a remote machine. PE timestamps are suppressed and source paths are
remapped; repeated native builds in the same workspace were checked for an
identical SHA-256. This does not claim bit-identical builds across different
workspace paths or arbitrary host toolchains.
Build outputs and dependencies under `.obj/`, and compiled binary file types,
are ignored by Git. Do not copy files to a Windows machine without explicit
instructions from the user.

Current pinned build dependencies:

* [LLVM-MinGW 20260908](https://github.com/mstorsjo/llvm-mingw/releases/tag/20260908),
  asset `llvm-mingw-20260908-ucrt-ubuntu-22.04-x86_64.tar.xz`, SHA-256
  `2258c745e3155870c80793f3e8c80b28fbde11b9ff73c4c78783635b3440b092`.
* [PDCurses 3.9](https://github.com/wmcbrine/PDCurses/tree/3.9), revision
  `6c1f95c4fa9f9f105879c2d99dd72a5bf335c046`, built statically with wide
  character and forced UTF-8 output support.
* [curl 8.23.0](https://curl.se/download/curl-8.23.0.tar.xz), HTTP(S)-only static
  library using native Schannel TLS, SHA-256
  `8dadd7d2536a314f982ad05a651b713f426bf7d02cc2c76cffb9796f2218e6c5`.
  Its license is in `windows/licenses/curl.txt`.
* [FFmpeg 8.0.3](https://ffmpeg.org/releases/ffmpeg-8.0.3.tar.xz), source archive
  SHA-256 `6136812ea6d4e68bdba27e33c2a94382711cdf4f8602ffef056ff792bd6f9818`.
  Downloaded sources and all compiled libraries remain under ignored `.obj/`;
  only the required notices are included under `windows/licenses/`.

For local decoder checks, `scripts/build_audio_deps.py --host` creates host
libraries from the same FFmpeg source. `scripts/check_audio_decoders.py` builds
the portable checker under `tests/` and generates fixtures with the local FFmpeg
command; reference PCM comes from the pinned host FFmpeg. `--native-channels`
checks the Linux channel policy, `--sanitize` enables address/undefined-behavior
checks, and `--windows` cross-compiles the checker for actual Windows execution.
These are
development checks, not runtime dependencies of the player.

The original `scripts/build_windows.sh` Cygwin recipe remains as a reference;
it is **not** the native build path and has not been exercised.

## Source and licensing

Based on https://github.com/cmus/cmus at revision
`521444c83beda6f13feb2eebcd27d12f3f23d70d` (upstream version 2.13.0).
Upstream copyright notices and GPL licensing are preserved; see `COPYING`.
Third-party notices are in `windows/licenses/`. Keep the notices and matching
source available when distributing a binary. The source repository is required
to build, not to run, the player. See [the release review](RELEASE.md) for the
validation gate, source-kit contents and outstanding distribution-source work.

The port reuses the upstream interface and playback engine. Native platform
services are in `windows/`; conditional changes in the upstream files register
static plugins, embed defaults, handle UTF-16 keyboard events, and adapt paths.
The red-black tree's pointer storage uses `uintptr_t` for Windows's LLP64 model.

## Feedback checkpoints

1. **Accepted:** interface, local folder import, Opus playback, playback/repeat
   modes, persistence, seeking, pause/stop, and volume.
2. **Reliability/build milestone:** automated checks and hardening described
   above; no further manual checks are requested from the user.
3. **Future distribution:** review packaging/source notices before publication.

## Remaining finalized-build checkpoints

HTTP(S) radio and remote control are implemented. Streaming uses the shared
bounded worker and HTTP(S)-only curl library with native Schannel TLS and
certificate validation. See the main README for stream behavior and limits.

The finalized build remains in progress: external commands/status hooks and
other platform integrations,
terminal/input completion, endurance checks and the complete source kit remain.

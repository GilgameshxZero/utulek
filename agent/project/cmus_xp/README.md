# cmus_xp (cross-platform cmus)

Native Linux and Windows builds share cmus's interface, playback engine and
expanded FFmpeg decoder. See [the Windows README](README_WINDOWS.md) for codec
coverage and platform-specific limits. Current version: **1.0.0**.
[Release validation and source review](RELEASE.md) describes the completed
reliability checkpoints and the remaining requirements before distribution.

Version 0.5 adds file-backed playlists on Linux and Windows. Add an M3U, M3U8,
PLS or cmus `.pl` file with `:add` (or `:add -p`); it appears as a separate
named entry in playlist view `3`. Adding a directory discovers playlist files
recursively while retaining the existing audio-directory import behavior.
`:pl-import` also accepts a file or directory and imports only playlists.
In browser view `5`, pressing `a` over a playlist file or folder uses the same
import logic; playlist entries appear in view `3`.

`:update` reloads all linked playlists, discovers new files in imported
directories, and removes playlist entries whose source files disappeared.
It works from any view and also updates the music library. `u` on a linked
playlist refreshes linked sources too. Source associations survive restarts;
different files with the same basename get distinct names, and re-adding the
same source does not duplicate it. Relative tracks resolve beside their source
file; UTF-8 BOMs, CRLF, local `file://` URIs, and indexed PLS order are supported.

Source files remain authoritative: imports, refreshes, normal playlist saves,
and removal of an entry never write to or delete its external source. Edits made
inside cmus are replaced on the next refresh. A deleted entry in a tracked
directory is rediscovered on the next update if its file still exists. Native
playlists created inside cmus remain independent. Scanning and loading run on
the worker thread; read failures retain the last successfully loaded contents.
HTTP(S) radio URLs now work on both platforms through a shared asynchronous
streaming decoder. MP3, Opus and FLAC, redirects, chunked responses, ICY titles,
and bounded M3U/PLS playlists are checked on actual Linux and Windows players.
Stop and Quit cancel pending network requests. TLS certificates are verified;
Windows uses its native certificate store and Linux prefers the host's CA
bundle, with public Mozilla roots bundled for otherwise empty systems.
`SSL_CERT_FILE` and `CURL_CA_BUNDLE` can select an explicit CA file.
Radio negotiates its decoded rate, depth and channels asynchronously and follows
upstream's Pause behavior of stopping the radio. Seeking and HLS adaptive playlists are unsupported.
The input plugin ABI is now 4; external input plugins must be rebuilt.

CUE-sheet tracks now work on both platforms. Import a `.cue` file or its
containing directory into the library, playlist or queue; browser `a` and typed
commands share the same import logic. CUE tracks retain their titles, album
metadata, relative/absolute backing-file paths and track-relative seeking.
Directory imports suppress duplicate backing audio files. INDEX 00/01 boundaries
follow upstream behavior. WAV and FLAC tracks, multiple-file sheets, pregaps,
fractional frame boundaries, Unicode paths and restart persistence are checked.
The shared fixes also correct 8-bit PCM conversion and sample rounding on seek.

The shared FFmpeg decoder and playback engine also build on Linux. Windows
adapters stay under `windows/`; Linux uses the existing curses, plugin and audio
output interfaces. Linux includes PulseAudio (also used with PipeWire) and ALSA
plugins. Opus and MKV playback has been checked through a running PipeWire/ALSA
device: stream-specific monitor measurements verify signal frequency, silence
while paused, seeking, resume, software volume and stop. This tests the live
audio stack, not microphone recordings of physical speakers. Native-channel
decoder and output tests preserve surround audio on both platforms.

Windows WaveOut accepts native speaker layouts and 8/16/24/32-bit PCM, with
speaker-order correction when required. Software volume and ReplayGain cover all
channels. Windows may remix the submitted PCM to the selected device's speaker
configuration. `cmus --list-output-devices` lists Windows output choices;
`dsp.waveout.device` accepts `default`, `#index`, a unique friendly name, or an
`id:` endpoint identifier. Stop playback before changing an open device. Missing
saved devices do not block startup; selecting an available device allows playback
to resume. Remote status exposes `output_sample_rate`, `output_bits` and
`output_channels` for PCM sent to the backend, rather than physical speaker count.

`scripts/build_linux.py` builds and installs `monochrome/app/cmus` and its
remote helper, with executable shell launchers in `monochrome/bin/`. Each program
is a portable single-file executable. All 13 input plugins, four output plugins,
defaults and themes are linked or embedded into the player. Third-party shared
libraries, common ALSA/PipeWire modules and terminal data are bundled using
[StaticX](https://staticx.readthedocs.io/en/stable/introduction.html). Its static
launcher extracts a private temporary directory while the program runs and
removes it on exit. No companion `cmus_xp-runtime/`, FUSE, Python, or installed
codec libraries are required. Configuration still uses the normal user paths;
user themes override embedded themes and both support tab completion. The
installer removes known resources from the former runtime directory after
checking the replacement binaries, preserving any unrecognized user files.

`scripts/prepare_linux.py` downloads hash-pinned FFmpeg/libao/CDDB sources,
builds private static libraries and prepares the pinned packaging environment
under `.obj/`. It also builds pinned NASM when an assembler is unavailable.
Libao's Pulse, ALSA and OSS drivers are built into its archive. The builder
uses the resulting audio-only FFmpeg archives by default. System development
headers/libraries are also needed for ncurses,
PulseAudio, ALSA, libao, JACK, libsamplerate, libsystemd (MPRIS), FAAD,
libcdio/CDDB/libdiscid, FLAC, libmad, libmodplug, Musepack, Opusfile,
Vorbis, WavPack and OpenSSL. HTTP(S)-only static libcurl is built privately by
`scripts/build_http_deps.py --host` (Schannel on Windows). These native plugins preserve existing Linux cmus options,
streaming and decoder priorities; FFmpeg also handles the additional formats.
Configuration fails when a required feature is missing rather than silently
producing a smaller build. The ordinary configure/make workflow still supports
custom feature selections for development. `--audio-libs` selects another static
FFmpeg prefix; `PKG_CONFIG_PATH` can supply other local development libraries.
Neither preparation nor the builder installs system packages. The portable
programs are relocatable. Uncompressed packaging favors startup speed;
`--compress` trades slower startup for smaller files. Build intermediates stay
under `.obj/linux_unbundled/`, finished portable programs under `.bin/`.
The portable build currently targets the build machine's Linux architecture.
Normal host audio services, device permissions and user device configuration
still apply; custom third-party ALSA plugins may require an explicit bundle
addition. StaticX uses ordinary files/DNS for name resolution rather than
site-specific NSS modules such as enterprise directory services.

`scripts/check_single_file.py` tests relocation, fresh defaults, all themes,
keyboard completion, user overrides and restart. `scripts/check_portable_runtime.py`
runs real terminal/remote interactions and ALSA-null playback inside an empty
Bubblewrap root with no host libraries or terminal data, then checks extraction
cleanup. CI's Linux archives contain the portable programs and script launchers.

The Windows player/client are `app/cmus.exe` and `app/cmus-remote.exe`, with
`bin/cmus.bat` and `bin/cmus-remote.bat` launchers. Both platforms share remote
commands and CLI controls; Windows uses same-user local named pipes or upstream
TCP authentication. See the Windows README for endpoint details.
The project is named cmus_xp; its executables remain `cmus` and `cmus-remote`
(`cmus.exe` on Windows). Existing configuration paths (`~/.config/cmus` on Linux and `%APPDATA%/cmus-win`
on Windows) and the `CMUS_HOME` override remain compatible.

`scripts/check_linux_audio.py` runs the live audio check with an isolated config
and quiet generated fixtures. It requires access to the user's running audio
server and device; sandbox-only decoder tests cannot establish live playback.

`scripts/check_linux_parity.py` compares plugins, startup using a copy of saved
settings, library/playlist/queue handling, persistence across restart, remote
commands, MPRIS and local HTTP MP3 streaming against system cmus. Run it with
an isolated D-Bus session and access to the audio server. Playback tests use
quiet fixtures and leave user configuration untouched. JACK, libao and CD
plugins are included. Separate libao/Pulse checks now pass Opus and MKV
play/pause/seek/resume/software-volume/stop with measured 440/880 Hz output.
The extra consumer sleep that caused periodic libao gaps has been fixed.
Physical JACK/CD devices and acoustic gapless transitions remain unverified.

The normal `configure`/`make` workflow below stages intermediates under `.obj/`
and finished programs under `.bin/`. Use system FFmpeg development libraries on
Linux, or the optional pinned host libraries from `scripts/build_audio_deps.py`.
The host helper builds a reference FFmpeg executable for decoder checks too.
For those static libraries, set `FFMPEG_PKG_CONFIG` to `pkg-config --static`,
with `PKG_CONFIG_PATH` pointing to their `lib/pkgconfig`. This selects static
FFmpeg dependencies while allowing native plugins to use shared libraries.

The original upstream README follows.

---

# cmus — C\* Music Player

https://cmus.github.io/

[![Build Status](https://github.com/cmus/cmus/actions/workflows/build.yml/badge.svg)](https://github.com/cmus/cmus/actions/workflows/build.yml)

Copyright © 2004-2008 Timo Hirvonen <tihirvon@gmail.com>

Copyright © 2008-2017 Various Authors


## Configuration

    $ ./configure

By default, features are auto-detected. To list all configuration options, run
`./configure --help`. Some common autoconf-style options like `--prefix` are
also available.

After running configure you can see from the generated `config.mk` file
what features have been configured in (see the `CONFIG_*` options).

The packages containing dependencies on common distributions are listed below. All dependencies other than pkg-config and ncurses, iconv, and elogind/systemd are for optional input/output plugins. It is assumed that libc headers, a C compiler, git, and GNU Make are available.

| Distro            | Dependencies |
| :--               | :--          |
| **Debian/Ubuntu** | apt install pkg-config libncursesw5-dev libfaad-dev libao-dev libasound2-dev libcddb2-dev libcdio-cdda-dev libdiscid-dev libavformat-dev libavcodec-dev libswresample-dev libflac-dev libjack-dev libmad0-dev libmodplug-dev libmpcdec-dev libsystemd-dev libopusfile-dev libpulse-dev libsamplerate0-dev libsndio-dev libvorbis-dev libwavpack-dev |
| **Fedora/RHEL**   | dnf install 'pkgconfig(ncursesw)' 'pkgconfig(alsa)' 'pkgconfig(ao)' 'pkgconfig(libcddb)' 'pkgconfig(libcdio_cdda)' 'pkgconfig(libdiscid)' 'pkgconfig(libavformat)' 'pkgconfig(libavcodec)' 'pkgconfig(libswresample)' 'pkgconfig(flac)' 'pkgconfig(jack)' 'pkgconfig(mad)' 'pkgconfig(libmodplug)' libmpcdec-devel 'pkgconfig(libsystemd)' 'pkgconfig(opusfile)' 'pkgconfig(libpulse)' 'pkgconfig(samplerate)' 'pkgconfig(vorbisfile)' 'pkgconfig(wavpack)' |
| **+ RPMFusion**   | dnf install faad2-devel libmp4v2-devel |
| **Arch Linux**    | pacman -S pkg-config ncurses libiconv faad2 alsa-lib libao libcddb libcdio-paranoia libdiscid ffmpeg flac jack libmad libmodplug libmp4v2 libmpcdec systemd opusfile libpulse libsamplerate libvorbis wavpack |
| **Alpine**        | apk add pkgconf ncurses-dev gnu-libiconv-dev alsa-lib-dev libao-dev libcddb-dev ffmpeg-dev flac-dev jack-dev libmad-dev libmodplug-dev elogind-dev opus-dev opusfile-dev pulseaudio-dev libsamplerate-dev libvorbis-dev wavpack-dev |
| **Termux**        | apt install libandroid-support ncurses libiconv ffmpeg libmad libmodplug opusfile pulseaudio libflac libvorbis libwavpack |
| **Homebrew**      | brew install pkg-config ncurses faad2 libao libcddb libcdio libdiscid ffmpeg flac jack mad libmodplug mp4v2 musepack opusfile libsamplerate libvorbis wavpack |


## Building

    $ make

Or on some BSD systems you need to explicitly use GNU make:

    $ gmake


## Installation

    $ make install

Or to install to a temporary directory:

    $ make install DESTDIR=~/tmp/cmus

This is useful when creating binary packages.

Remember to replace `make` with `gmake` if needed.


## Manuals

    $ man cmus-tutorial

And

    $ man cmus


## IRC Channel

Feel free to join IRC channel #cmus on Libera.chat and share you experience,
problems and issues. Note: This is an unofficial channel and all people hanging
around there are for the love of cmus.


## Reporting Bugs

Bugs should be reported using the GitHub [issue
tracker](https://github.com/cmus/cmus/issues). When creating a new issue, a
template will be shown containing instructions on how to collect the necessary
information.

Additional debug information can be found in `~/cmus-debug.txt` if you
configured cmus with maximum debug level (`./configure DEBUG=2`). In case of a
crash the last lines may be helpful.


## Git Repository

https://github.com/cmus/cmus

    $ git clone https://github.com/cmus/cmus.git


## Hacking

cmus uses the [Linux kernel coding
style](https://www.kernel.org/doc/html/latest/process/coding-style.html). Use
hard tabs. Tabs are _always_ 8 characters wide. Keep the style consistent with
rest of the code.

Bug fixes and implementations of new features should be suggested as a
[pull request](https://github.com/cmus/cmus/pulls) directly on GitHub.


## Local build output

Intermediates, build artifacts, dependencies, caches, and generated test reports belong in `.obj/`. Finished binaries belong in `.bin/`. Both directories are local to this project and ignored by Git. `make clean` removes both directories.

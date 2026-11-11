# cmus_xp 1.0.0 — final cross-platform build

This is the finalized local build of cmus_xp for Linux x86_64 and native
Windows x64. It is an installed, validated local release candidate; it has not
been published or uploaded to a package repository.

## Scope

The player keeps the upstream terminal interface, command language, bindings,
library, playlists, queue, filters, search, persistence and status formats. In
addition to ordinary local files it supports:

* FFmpeg-based local audio and video-container audio, native WAV, FLAC, Vorbis,
  Opus, MAD/MP3, Musepack, WavPack, AAC and tracker modules.
* HTTP(S) streaming with bounded playlists, ICY metadata, TLS, redirects and
  cancellable Stop/Quit.
* Remote control over Unix sockets, same-user Windows named pipes, or
  authenticated upstream TCP.
* CUE sheets, native multichannel output up to 18 WAV channels, WaveOut device
  selection, radio-format handoff and ReplayGain-aware PCM scaling.
* File-backed playlists and `:update` synchronization.
* External commands and hooks through native `CreateProcessW` on Windows,
  including UTF-16 paths, argument quoting, exit status and `.bat`/`.cmd` files.
* Windows System Media Transport Controls and Linux MPRIS.
* Legacy tag encodings through iconv (libc on Linux, Windows NLS on Windows).
* Bracketed paste on both platforms, with markers consumed rather than inserted.
* Native Windows CD-DA input (`cdda://`) using CD-ROM TOC/raw-read IOCTLs.

## Validation summary

Checkpoints 06–13 are saved under `.obj/checkpoints/` with source archives,
exact binaries, evidence hashes and, for each installed checkpoint, actual
Windows sync/version/runtime verification.

* **Compatibility:** all 88 pinned upstream command declarations plus the added
  `pl-import`; 15 feature groups audited. Declarations are not behavioral proof;
  behavioral evidence is separate.
* **Remote:** Linux CLI/protocol/concurrency/restart and real Windows console
  checks, including large exports and backpressure.
* **Streaming:** Linux and Windows HTTP(S), playlist, ICY, TLS, rejection and
  cancellation suites; ASan/UBSan variants on Linux.
* **CUE and decoders:** CUE sanitizer/UI suites and native decoder regressions on
  both platforms.
* **Multichannel:** PCM/WaveOut model tests, 512 PCM format cases, native
  Linux/Windows device and format-transition tests, live Linux Pulse/PipeWire
  Opus/MKV controls.
* **Platform parity:** external command, legacy tag, tracker-module, CD URL/TOC
  and SMTC tests; SMTC verified through a real console session.
* **Daily use:** 60-cycle mixed-format endurance runs on Linux and Windows with
  bounded memory/descriptors, playing-file truncation recovery, command storms
  and clean exits.
* **Packaging:** portable Linux single-file relocation, empty-root execution,
  themes/completion and installed launcher checks; Windows runtime diagnostics
  pass 5,005 checks with 0 failures.

## Installed artifacts

Linux binaries are in `monochrome/app/cmus` and `cmus-remote`, with POSIX shell
launchers in `monochrome/bin`. Windows binaries are `cmus.exe` and
`cmus-remote.exe` in `monochrome/app`, with batch launchers in `monochrome/bin`.
The same `app`/`bin` layout is used on both platforms, and Windows receives the
files through Syncthing.

## Known limits

These are explicit limits of this build, not untested claims presented as
successes:

* No physical output-device hotplug/disconnect or hardware failure test.
* Live JACK devices, physical CD hardware, acoustic gapless playback and
  acoustic speaker output are not validated.
* CD-TEXT and online CDDB metadata are not implemented; CD entries show the
  track number.
* Hardware media-key delivery and the Windows shell media flyout were not
  manually exercised; SMTC registration, metadata and status calls were.
* Populated wide-character partial redraws inside an existing Windows tmux
  session still differ from direct terminals. tmux is intentionally unmodified.
* HTTP radio seeking and adaptive HLS playlists are unsupported.
* There is no manual audio-track selector for containers with multiple tracks.
* LeakSanitizer was disabled in the traced test environment; ASan and UBSan were
  enabled where stated.
* Not every enabled codec or container has a dedicated fixture.

## Release gate and source kit

`scripts/check_final_release.py` verifies the checkpoint chain, final installed
artifact hashes, required plugins, the pinned dependency archives and a fresh
Linux smoke interaction. `scripts/prepare_release_review.py` can assemble an
offline review kit from the passing gate, cached archives, notices and Linux
provenance.

The review kit is **not a complete distribution source kit** and this build is
**not publication-ready**. Distribution blockers remain:

* Exact corresponding sources, downstream patches and build recipes for every
  bundled Linux distribution library and runtime data file are not assembled.
  The available Linux inventory is a post-build package-ownership record.
* The pinned LLVM-MinGW binary toolchain and notices are included, but the exact
  corresponding MinGW/winpthreads runtime source revision is not assembled.
* StaticX modifies ELF files after packaging, so post-build ownership records do
  not prove byte-for-byte provenance. Future packaging should record input
  hashes and package versions before transformation.
* The bundled Linux components include GPL-3.0-or-later libcdio code. Retain all
  component notices and review combined distribution terms; do not describe the
  whole bundle as GPLv2-only.

No binary, kit or source archive has been published. Remote CI has not been run.
Historical 0.5.x review notes and kits are obsolete for this version.

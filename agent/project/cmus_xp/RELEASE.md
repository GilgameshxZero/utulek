# 0.6.0-dev: remote-control checkpoint

The finalized cross-platform build is progressing through the compatibility
checkpoints. The audit found all 88 pinned upstream command declarations present,
plus the added playlist-import command. Source declarations alone do not establish
behavioral parity; the audit inventory and implementation evidence are separate.

Windows now includes `cmus-remote.exe` and its batch launcher, using shared
command handlers over same-user local named pipes or upstream authenticated TCP.
Linux retains Unix sockets/TCP. Both benefit from preserving fragmented commands,
dynamic raw/stdin command buffers, queued nonblocking replies and native Windows
UTF-8 argument handling. Shared stdout saves use the same reply queue as status
and error messages. Stalled clients cannot busy-wait inside a shared write loop.

Validation includes ten Linux CLI/protocol/concurrency/restart checks, 30 actual
Windows native-console checks (including real playback/seek controls and TCP),
live Linux Opus/MKV transport, full 49,999-file stdout export from retained
fixtures, and keyboard/other-client responsiveness during unread large exports
on both platforms. The installed 0.5.2 baseline fails the retained fragmented and
long-command regression checks. Portable packaging and persistence checks are
repeated for the new artifacts. The earlier 50,000-file missing-track test had
removed one fixture; this checkpoint reports the retained count accurately.

Streaming, CUE, multichannel, remaining Windows integration, terminal completion,
endurance/hardware checks and complete distribution-source assembly remain in the
subsequent finalized-build checkpoints. This checkpoint does not claim those
features are complete or approve publication. The 0.5.2 evidence and source-review
kit below remain historical records for that binary version.

---

# 0.5.2-dev reliability and release review

This milestone covers the existing local-file player on Linux x64 and native
Windows x64. Streaming, remote control, CUE, native multichannel output and fuller
external-command integration on Windows remain deferred. This review does not
expand that scope or approve publication.

## Validation

The installed 0.5.2-dev binaries retain the fixes validated in checkpoints 02–04:

* Live Linux Opus/MKV transport through Pulse and libao, including measured
  frequency, pause silence, seeking, resume, software volume and stop. Removing
  an extra consumer sleep eliminated the measured periodic libao gaps.
* Atomic state saves preserve previous files on failed writes. Long Unicode
  configuration paths, restart, truncated-cache recovery and Linux interruption
  before replacement have been exercised. This is not a power-loss guarantee.
* Actual Linux and Windows 50,000-file import/export, search, queue, reshuffle,
  navigation, resize, restart and missing-file updates pass. Windows evidence
  includes native console and ConPTY. The large-library export polling bug was
  in the stress harness and was corrected without speculative player changes.
* 56 native-channel Linux decoder cases under ASan/UBSan, 56 Windows stereo-policy
  cases, 400 deterministic malformed mutations and four model regression groups
  pass. Decoder checks include PCM comparisons, tags, seeking, restart and
  malformed rejection. Enabled codecs are not all covered by fixtures.
* Portable Linux relocation, embedded defaults/themes, real keyboard completion,
  user overrides, persistence, empty-root execution and Alpine 3.22.2 execution
  pass. Windows runtime diagnostics passed 5,005 checks with zero failures.

Physical hotplug/reconnect, live JACK/CD hardware, acoustic gapless transitions,
other OS/hardware combinations and populated Windows tmux wide-character redraw
remain unverified or deferred. LeakSanitizer was disabled in the traced test
environment; AddressSanitizer and UndefinedBehaviorSanitizer were enabled.

## Repeatable gate

`scripts/check_release.py` requires the candidate Linux pair and Windows executable,
the actual Windows platform report and an `--evidence-root` containing checkpoints
02–04 and their referenced reports. The root can be a recovered `.obj` snapshot;
recorded absolute evidence paths are relocated beneath that root with traversal
checks. The script verifies:

* Candidate hashes match all three checkpoints, and saved source/evidence hashes
  are intact. Windows installation verification is present.
* Production C/header files and embedded data match checkpoint 04. Changes to
  test harnesses, release tooling or documentation alone do not require rebuilding
  identical player binaries.
* Both 56-case decoder reports, all 400 malformed cases, four model groups and
  the five-check alternate-distribution report are present and passing.
* Windows evidence names the candidate hash and records native large-library,
  decoder and runtime coverage.
* Fresh isolated Linux relocation/theme, failed-save/restart and empty-root
  interactions pass. Supplying `--rootfs` repeats execution in that distribution.

Results and per-check logs go into ignored `.obj/release_gate/`. Gate failures
write an incomplete report with `passed: false`. Historical evidence is reused
for unchanged binaries; it is not represented as a new Windows or hardware run.
The checkpoint hashes provide integrity/traceability, not independently signed
attestation. `tests/check_release_gate.py` exercises rejection of missing or
modified evidence, wrong binaries, incomplete coverage and changed runtime source.

`scripts/prepare_windows_decoder_checks.py` prepares all fixtures and stereo PCM
references for `windows/check_decoders.ps1`. Explicit `--checker` and `--ffmpeg`
paths allow recovered artifacts and a locally runnable same-release reference
tool to be selected without replacing build caches. The package is installed
atomically after successful preparation. Generated payloads have been compared
byte for byte with the fixtures, executable and corrected references used in the
passing Windows decoder run.
The generated package and standalone PowerShell runner also pass all 56 cases
on actual Windows. The installed batch launcher's version and all 5,005 runtime
checks pass, its executable hash matches the checkpoint, and the isolated test
directory and upload have been removed after report retrieval.

`scripts/checkpoint_release.py` saves the current project source, exact binaries
and evidence hashes under ignored `.obj/checkpoints/`. Its installation option
checks already-installed hashes and avoids rewriting identical binaries.

## Provenance and local source-review kit

`scripts/collect_linux_provenance.py` reads the original Arch/Manjaro build host's
portable manifests, package owners/versions, package notices and applicable SPDX
license texts. The reviewed pair contains 94 library names (including soname
aliases) belonging to 36 installed packages; ALSA, PipeWire and terminfo data
owners were also identified. This is a post-build inventory. StaticX modifies ELF
files, so package ownership alone is not proof of exact bundled-file provenance.

`scripts/prepare_release_review.py` prepares an offline local kit from that
inventory, passing gate evidence and verified cached archives. It includes:

* Current cmus_xp source and build scripts, with generated/private trees excluded.
* Hash-checked FFmpeg 8.0.3, PDCurses, NASM, libao and libcddb source archives,
  plus the pinned LLVM-MinGW binary toolchain archive identified as a build tool.
* Windows runtime notices, StaticX's license/bootloader exception, Linux package
  metadata and recovered package/SPDX notices, including GPLv3 terms.
* Candidate hashes, validation report, kit file hashes and explicit distribution
  blockers in `review.json`.

The kit is **a local review artifact, not a complete distribution source kit**.
Before publishing binaries:

* Assemble the exact source packages, downstream patches and build recipes for
  bundled Linux libraries and runtime data. Upstream URLs alone are insufficient.
* Assemble the precise MinGW/winpthreads runtime source corresponding to the
  pinned binary toolchain; the compiler binary archive is not its source.
* Record original library/data hashes and package provenance during packaging,
  rather than relying only on a post-build package inventory.
* Account for GPL-3.0-or-later libcdio components in the Linux bundle; the complete
  bundle must not be described as GPLv2-only merely because `COPYING` contains
  the upstream GPLv2 text. Retain all component notices and runtime exceptions.

The bundled FFmpeg configuration disables GPL/nonfree optional components and
networking. It is statically linked into the GPL player; the exact source and
build recipe are retained. Consult [FFmpeg's licensing page](https://ffmpeg.org/legal.html)
alongside the component license texts when preparing distribution.

Older 0.5.0 review packages are obsolete and must not be used for distribution.
No release kit or binary has been published as part of this milestone. Remote CI
has not been run; local validation does not establish a clean hosted-CI build.

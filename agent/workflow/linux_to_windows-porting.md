# Porting Linux applications to native Windows

Maintain this workflow as ports reveal reusable lessons. Keep project-specific status in an ignored context handoff, and source under project/.

## Establish scope

* Read the earlier project handoff and workflow/project_binaries-workflow.md.
* Distinguish an upstream source port from a compatible reimplementation. Establish required features and native Windows requirements before choosing architecture.
* Inspect current upstream platform assumptions and existing Windows implementations using primary sources. Existing projects are candidates to evaluate, not evidence of compatibility or quality.
* Estimate platform adaptation separately from feature parity and runtime hardening. State estimates as provisional engineering effort rather than promised delivery dates.

## Build and packaging

* Prefer reusable cross-compilation infrastructure where suitable. cmus-xp uses LLVM-MinGW and static dependencies; project documentation describes both Windows and Linux build entry points.
* Keep dependencies, downloads, build outputs, fixtures, and generated reports in the project's ignored out/ directory. Track source, reproducible build scripts, dependency pins/hashes, and required license notices.
* Follow project_binaries-workflow.md for app binaries and launchers. Sandbox permissions still apply to installation destinations.
* Do not create Git commits or publish/upload artifacts without authorization.
* Do not place files on the Windows host unless explicitly authorized. Existing SSH access does not imply upload permission.

## Platform audit

* Inventory POSIX dependencies: PTYs, fork/exec, signals, process groups, Unix sockets, descriptor polling, termios, filesystem paths, dynamic loading, and shell commands.
* Preserve portable application logic and isolate Windows implementations behind clear interfaces where practical.
* For terminal multiplexers, assess ConPTY process hosting, synchronous pipe I/O/backpressure, persistent server lifetime and IPC, terminal parsing/rendering, keyboard/mouse encoding, Unicode widths, resizing, and process cleanup.
* Separate command/config compatibility from Unix shell/plugin compatibility; a native executable cannot automatically supply Unix shell tools or semantics.

## Verification and handoff

* Cross-compilation establishes buildability, not Windows runtime behavior.
* A sandbox may hide audio devices and block the desktop audio socket. Inspect the host's actual audio server/devices before concluding that playback validation is unavailable. For live Linux playback, use an isolated player configuration and monitor only its stream; preserve the user's sink, volume and other streams. Distinguish delivery through a live audio device from an acoustic listening test.
* Run checks appropriate to the change under applicable instructions. For Windows support, the project binaries workflow requires actual Windows execution and launcher argument checks; obtain explicit host-write authorization before staging files there.
* Retain concise, queryable results for costly grouped checks; do not repeat dependency builds or broad checks without a relevant change, failure, or unresolved concern.
* Report exactly what was built, what ran on Windows, what remains unverified, and material feature limitations.
* Save a concise ignored handoff only when needed for continuity. Update this workflow with reusable findings, not routine activity.

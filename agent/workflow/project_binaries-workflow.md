# Project binaries and launchers

The user's installation convention (updated November 1, 2026):

* Keep software source projects under `project/` in the agent workspace.
* Install compiled binaries under `~/main.syncthing/monochrome/app/`, rather than `monochrome/bin/`. A project subdirectory is appropriate when multiple runtime files belong together.
* Put executable shell launchers and Windows `.bat` launchers in `~/main.syncthing/monochrome/bin/`. Follow existing launchers in that directory.
* Launchers must quote the executable path and forward every argument: shell `exec ".../app/program" "$@"`; batch `@"%~dp0..\app\program.exe" %*`. Preserve the program's exit code and do not change the caller's working directory unnecessarily.
* For SMSGate, install `app/smsgatec` and `app/smsgatec.exe`; expose `bin/smsgatec` and `bin/smsgatec.bat`. Remove obsolete raw binaries from `bin/` after installing their replacements.
* Keep only current runtime binaries and callers in `monochrome/app/` and `monochrome/bin/`. After successful replacement or rename, delete obsolete binaries and shell/batch callers, including `.old`, `.previous`, backup copies, retired names, and temporary test executables. Do not retain old versions in either installation directory unless the user explicitly requests it. Perform shared-file cleanup locally only; Syncthing propagates deletions to other machines. Do not repeat these deletions over SSH unless explicitly requested. Verify both local directories after cleanup. Do not delete active Syncthing transfer files.
* Put all project-local intermediates (including build artifacts, downloaded build dependencies, caches, and test reports) in `.obj/`, and finished binaries in `.bin/`. Ignore both directories per project. Build and clean targets must follow this convention; `make clean` removes both. Installed copies still follow the app/bin convention below.
* Keep a project-specific `.gitignore` in every source project covering its binaries, intermediates, build directories, caches, and generated test output. Only source and supporting build scripts, documentation, licenses, and assets belong in Git.
* Build outputs and private configuration remain ignored; never force-add generated binaries or credentials to the public repository. Do not create Git commits.
* When Windows support is part of the task, run the compiled Windows regression executable on an actual Windows machine and check the installed batch launcher with arguments. Cross-compilation alone is not a Windows runtime check. The private Windows test host and established procedure are recorded in ignored `context/projects/smsgate/sms-windows_port.md`.

## Syncthing scope

`~/main.syncthing/` is synchronized between machines. When asked to remove files
there, remove them locally only and let Syncthing propagate the deletions.
Files outside `main.syncthing` are not synchronized. In particular, live configs
under `~/.config/` need explicit copying to other machines when requested; an
archive copy under `main.syncthing/monochrome/module/` does not update those live
configs automatically.

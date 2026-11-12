# neomutt_xp: cross-platform NeoMutt

Linux and native Windows builds of one shared NeoMutt codebase, based on the
pinned upstream source plus reviewable patches and Windows platform services.
The configuration compatibility helpers are installed and accepted. The native
application is an early development candidate, not a production replacement.
Detailed verification history is in [docs/verification.md](docs/verification.md).

## Build

`dependencies.json` is the single source of truth for pinned versions, hashes,
per-platform dependency lists and source-verification coverage. `scripts/deps.py`
turns that manifest into paths, library names, toolchain settings and build
stamps; no script carries its own copy of a version string.

`scripts/prepare.py --platform {linux,windows}` verifies (or downloads) the
pinned archives, extracts a fresh source tree once, and applies `patches/`. It
records the applied patch hashes and refuses to build a tree whose patches have
changed since extraction; pass `--refresh` to re-extract. Extraction happens in
a sibling `.partial` directory and is renamed into place, so an interrupted
prepare never leaves a half-written tree.

`scripts/build.py` builds the `linux` or `windows` target. Windows also requires
`--toolchain` (LLVM-MinGW). Native Windows dependencies are built by
`scripts/build_deps.py` (ncurses, libiconv, PCRE2), `scripts/build_openssl.py`,
`scripts/build_lmdb.py` and `scripts/build_crypto.py` (libgpg-error, libassuan,
GPGME, SQLite). Each step records a stamp under `.obj/stamps/` keyed by version,
options, toolchain and source fingerprint; an unchanged no-op build skips
configure, compile and install. `make verify-sources` re-checks every original
file against the pinned archive using chunked comparison.

`make linux`, `make windows` and `make toolchain` are the supported entry
points; `JOBS` selects parallelism (default: all CPUs). The Windows target links
all libraries statically and embeds its system defaults and MIME table in
`neomutt.exe`, so a single executable is enough. OpenPGP uses the standard
external-command backend with the `gpg` already on `PATH` (`crypt_use_gpgme` is
unset in the embedded defaults); a local `%R` expando expands one `-r` option per
recipient, replacing the external `pgpewrap` helper. GPGME stays linked but is
not the default crypto backend on Windows. Calendar/OAuth scripts and the GPG
installation remain external in `module/email` and `PATH`; Linux uses stock
NeoMutt unchanged.

Build outputs stay in `.obj/`, finished executables in `.bin/{linux,windows}/`.
`scripts/checkpoint.py` saves source, binaries, SHA256 metadata and retained
verification results under `.obj/checkpoints/`; artifacts are stored once as
content-addressed blobs and linked into each checkpoint. No Git commits are
needed. Checkpoints are developmental snapshots with explicit limits, not
installations.

## Runtime design

Keep commands inline when the tools exist on both platforms: ls, rm, iconv,
xdg-open, mpv and Lynx. The configuration resolves my_active_dir using $HOME
before invoking commands, quotes helper/token paths and sources GPG stdout
directly. Password loading needs neither a script pair nor a plaintext scratch
file. The calendar request generator uses `uuidgen` (with a PowerShell fallback
on Windows) and writes `.request.tmp.ics` directly.

| Source | Purpose |
| --- | --- |
| runtime/neomutt-html-open and .bat | Shared HTML wrapper; absolute browser path, atomic page publication, guarded detached cleanup, and Windows batch argument handling. Publication, timer start and browser launch use one Python interpreter. |
| runtime/neomutt-ical-request and .bat | Shared interactive request generator; identities come from the current serialized draft. |
| runtime/neomutt-query and .bat | Preserve echo/xargs/khard query semantics; Windows selects khard.exe because xargs cannot execute khard.bat. |
| python/mutt-ical.py | Display invitations and generate replies with the existing prompts and response behavior. |
| python/ical_support.py and timezones.zip | Standard-library calendar support and bundled IANA timezone data. |
| python/mutt_oauth2.py | Existing OAuth provider/token flow with cross-platform command and private-storage handling. |
| python/command_args.py and private_storage.py | Platform command splitting, Unix permission checks and Windows ACL checks. Windows verifies and tightens ACLs through the native security API and keeps the reviewed PowerShell implementation as a fail-closed fallback. |
| runtime/lynx.bat | Launcher for native Windows Lynx under app/lynx. |

The .bat launchers call the shared scripts through the existing Git for Windows
Bash. They retain the working directory, inherit environment variables and
forward exit status. NEOMUTT_FROM and BROWSER cross that boundary; setlocal keeps
wrapper changes local without removing inherited values. The query batch wrapper
selects the native khard executable through NEOMUTT_QUERY_KHARD. There are no
separate Windows implementations of the HTML/request/query workflows.

Both platforms invoke the existing .py files explicitly through python3 with
UTF-8 mode. Python runtime code uses only the standard library and bundled local
modules/data; no pip packages are required. The calendar implementation requires
Python 3.9 or newer. Native Windows Python 3.11.9 was used for verification.

The static mailcap and vault lookup use paths relative to the configured
snowfall working directory. HTML viewing preserves the original wrapper bytes,
stable scratch filename, BROWSER override and 64-second cleanup. Calendar macros
create NeoMutt-owned attachments before generation, use a no-op composer and
refresh encoding afterward; discarding a draft removes .reply.tmp.ics and
.request.tmp.ics. Continued quoted strings in neomuttrc have no indentation
inside the quoted value.

## Dependencies and installed layout

Windows uses its existing Bash, coreutils, iconv and GPG from Git for Windows,
plus Python, PowerShell, khard and the existing viewers. Linux retains its
existing tools. GPG settings are streamed directly to NeoMutt rather than
buffered until successful decryption. The extensionless xdg-open script retains
its Linux behavior; Windows uses the native START-based xdg-open.bat, quoting
its single file/URL argument.

Email runtime files reside in monochrome/module/email. Native Windows Lynx
runtime and notices reside in monochrome/app/lynx; monochrome/bin/lynx.bat
exposes it on PATH. dependencies.json pins Lynx 2.9.2 and IANA timezone data
2026f. scripts/build_timezone_data.py rebuilds the bundled data from the pinned system
database. Upstream notices and licenses are retained under licenses/ and in
runtime files.

## Verification and limits

Validation compares helper behavior on Linux and actual Windows using synthetic
inputs and disposable homes/mailboxes, plus real PTY/ConPTY NeoMutt workflows
and fixture SMTP/IMAP servers. The coverage record is maintained in
[docs/verification.md](docs/verification.md).

Production account authentication, GUI pinentry, external attachment
applications, IMAP, synchronization races and clean-machine packaging still
need validation. Fixed scratch filenames retain their concurrency limitations.
The calendar parser is event-focused, not a general recurrence engine;
unsupported custom VTIMEZONE rules fail. Generated timezone serialization can
differ while preserving the tested calendar properties. Small native Lynx
layout differences were accepted. Fixture tests establish the tested cases
rather than universal behavioral equivalence.

Google Calendar's email RSVP ingestion is asynchronous: a submitted reply can
take minutes (occasionally much longer) to change the organizer-visible guest
state. Verification must poll for the calendar notification or the guest summary
instead of treating a short delay as a failed submission.

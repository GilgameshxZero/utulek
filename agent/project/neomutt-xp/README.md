# neomutt-xp: cross-platform NeoMutt

Linux and native Windows builds of one shared NeoMutt codebase, based on the
pinned upstream source plus reviewable patches and Windows platform services.
The configuration compatibility helpers are installed and accepted. The native
application is an early development candidate, not a production replacement.

## Native application development

Both platforms build the same patched `.obj/upstream` source. `scripts/verify_sources.py`
reconstructs that tree and patched dependencies from pinned archives and checks
all original source files against the working trees. Windows services
under `native/` provide UTF-8 filesystem operations, full file identities,
locking, child processes, and Winsock transport. The Windows executable imports
Windows system/UCRT libraries. Windows shell commands use CMD with delayed
expansion disabled; paired batch launchers invoke Git Bash for the shared helper
scripts. The UTF-8
application manifest requires Windows 10 1903 or newer.

Dependency versions and SHA256 hashes are pinned in `dependencies.json`.
`scripts/prepare.py` verifies/downloads archives and applies `patches/` to fresh
sources. `scripts/build_deps.py` builds native ncurses, libiconv and PCRE2;
`scripts/build_openssl.py` builds native TLS libraries. These Windows dependency scripts accept a
`--toolchain` directory containing LLVM-MinGW. `scripts/build.py` builds the
`linux` or `windows` target; Windows also requires `--toolchain`. The Makefile exposes platform build targets, source verification and optional
toolchain preparation; `TOOLCHAIN` can select an existing LLVM-MinGW installation.
`scripts/build_crypto.py` builds pinned static libgpg-error, libassuan, GPGME
and SQLite for both platforms. The platform Makefile targets prepare and build
these dependencies too. Windows packaging extracts the pinned GnuPG installer
without executing it and stages private engines, MIME types, the portable system
configuration and local manual under `.bin/windows/neomutt/`. Install that resource
directory alongside `neomutt.exe`; the executable alone is insufficient for GPGME.
The private engine does not replace the user's GPG installation or keyring.

Build outputs stay in `.obj/`, finished executables in `.bin/{linux,windows}/`.

`scripts/checkpoint.py` saves source, binaries, SHA256 metadata and retained
verification results under `.obj/checkpoints/`. No Git commits are needed.
Checkpoints are developmental snapshots with explicit limits, not installations.
The current UI candidate passed actual Windows system/editor ConPTY navigation,
reading, search, Unicode subject serialization, editor handoff, discard, flags,
purge, tagged moves, postponement, restart/recall and clean exit using synthetic
mail. Parsed terminal screens validate resize and editor-return layouts on both
Windows terminal implementations; Linux screen checks also pass. Native Windows
HTML/Lynx, calendar mailcap display and synthetic contact queries pass.
Native calendar macros also exercise invitation responses, request generation and
owned-attachment cleanup. The cancellation regression covers acknowledging an
external helper failure; Windows uses console events for that single-key prompt
and flushes pending input without a blocking CRT drain read. Linux stock and patched builds passed the basic PTY workflows.
Native file/process/cache regression sources and terminal harnesses are in `tests/`.
Mailbox/workflow parity remains under development. Actual SMTP/TLS fixture tests
cover password/OAuth authentication, encrypted fixture passwords/tokens, trust
rejection and interrupted submissions on both platforms. Native sendmail process
execution passes capture-only success/failure/background checks on both platforms.
The configuration parser also consumes real GPG-encrypted synthetic settings.
Interactive Windows GPG loopback pinentry passes through both ConPTY implementations:
a typed passphrase stays hidden and decryption succeeds. Git's MSYS GPG requires
an MSYS-style path for an explicit `--homedir` so its agent can create a socket;
the tests use isolated homes and leave user GPG settings unchanged.
PGP/MIME reader tests also pass on Linux and both Windows terminal implementations:
the correct passphrase decrypts the synthetic message into the pager, and wrong
or cancelled input reveals no plaintext. Native anonymous temporary files use
exclusive creation, private ACLs and deletion sharing, preserving NeoMutt's
open-then-unlink behavior.
No production account or credential is used by the fixture tests; configuration
compatibility checks reproduce the installed configuration structure with fake data.

The Windows baseline opts credential/token GPG pipes into loopback pinentry;
the shared configuration defaults that option to empty on Linux. Synthetic
console tests include decrypting an expired token, a mocked provider refresh,
re-encryption, and verification with the original password. This does not change
global GPG settings or replace GPGME's message-crypto pinentry.

Windows reverse-video cells preserve terminal default colors rather than
swapping fixed palette entries. Default-colored bold text keeps the theme's
foreground color; the legacy console cell API cannot represent a separate bold
font weight for that default color. Explicit palette colors retain intensity.

The attachment tests cover production mailcap invocation, base64/Unicode HTML,
spaced paths, actual isolated Edge/Chromium rendering, failure status, and
overlapping cleanup timers. HTML keeps its existing stable filename and 64-second
lifetime; reloading a browser tab after cleanup still requires reopening the
attachment. Calendar workflow fixtures mirror the shared configuration with fake
credentials and isolated mailboxes/calendar storage, and exercise both reply
bindings, response choices, save, display and discard cleanup. Reply-address
drafts are compared against stock Linux behavior, including self-replies and
Reply-To. These checks do not certify every mailbox failure or synchronization
race.

## Runtime design

Keep commands inline when the tools exist on both platforms: ls, rm, iconv,
xdg-open, mpv and Lynx. The configuration resolves my_active_dir using $HOME
before invoking commands, quotes helper/token paths and sources GPG stdout
directly. Password loading needs neither a script pair nor a plaintext scratch
file. Calendar saving uses inline Python standard-library UUID generation and
shutil.move.

| Source | Purpose |
| --- | --- |
| runtime/neomutt-html-open and .bat | Shared HTML wrapper; absolute browser path, atomic page publication, guarded detached cleanup, and Windows batch argument handling. |
| runtime/neomutt-ical-request and .bat | One shared interactive request generator; identities come from the current serialized draft. |
| runtime/neomutt-query and .bat | Preserve echo/xargs/khard query semantics; Windows selects khard.exe because xargs cannot execute khard.bat. |
| python/mutt-ical.py | Display invitations and generate replies with the existing prompts and response behavior. |
| python/ical_support.py and timezones.zip | Standard-library calendar support and bundled IANA timezone data. |
| python/mutt_oauth2.py | Existing OAuth provider/token flow with cross-platform command and private-storage handling. |
| python/command_args.py and private_storage.py | Platform command splitting, Unix permission checks and Windows ACL checks. |
| lynx.bat | Launcher for native Windows Lynx under app/lynx. |

The .bat launchers call the shared scripts through the existing Git for Windows
Bash. They retain the working directory, inherit environment variables and
forward exit status. NEOMUTT_FROM and BROWSER cross that boundary; setlocal keeps
wrapper changes local without removing inherited values. The query batch wrapper
selects the native khard executable through NEOMUTT_QUERY_KHARD. There are no separate
Windows implementations of the HTML/request/query workflows.

Both platforms invoke the existing .py files explicitly through python3 with
UTF-8 mode. Python runtime code uses only the standard library and bundled local
modules/data; no pip packages are required. The calendar implementation requires
Python 3.9 or newer. Native Windows Python 3.11.9 was used for verification.

The static mailcap and vault lookup use paths relative to the configured snowfall
working directory. HTML viewing preserves the original wrapper bytes, stable
scratch filename, BROWSER override and 64-second cleanup. Calendar macros create
NeoMutt-owned attachments before generation, use a no-op composer and refresh
encoding afterward; discarding a draft removes .reply.tmp.ics and
.request.tmp.ics. Continued quoted strings in neomuttrc have no indentation
inside the quoted value.

## Dependencies and installed layout

Windows uses its existing Bash, coreutils, iconv and GPG from Git for Windows,
plus Python, PowerShell, khard and the existing viewers. Linux retains its
existing tools. GPG settings are streamed directly to NeoMutt rather than buffered
until successful decryption. The extensionless xdg-open script retains its Linux behavior; Windows uses the
native START-based xdg-open.bat, quoting its single file/URL argument.

Email runtime files reside in monochrome/module/email. Native Windows Lynx
runtime and notices reside in monochrome/app/lynx; monochrome/bin/lynx.bat exposes
it on PATH. dependencies.json pins Lynx 2.9.2 and IANA timezone data 2026f.
build_timezone_data.py rebuilds the bundled data from the pinned system database.
Upstream notices and licenses are retained under licenses/ and in runtime files.

## Verification and limits

Validation compared helper behavior on Linux and actual Windows using synthetic
inputs and disposable homes/mailboxes. Coverage included arguments and Unicode,
working directories, environment inheritance, exit codes, inline capture/save,
query behavior, calendar display/replies/requests, OAuth with mocked providers
and real fixture GPG encryption, and HTML rendering. Python helpers were tested
without site packages.

Actual Linux NeoMutt UI checks covered saving invitations, reply/group reply
acceptance/decline/tentative/cancellation, request creation, discard, postpone and
capture-send. Parsed configuration settings were compared with fake credentials.
No real credentials were decrypted, real mail sent or actual attachment viewers
opened during automated verification. The earlier configuration acceptance cleanup removed its temporary artifacts.
Native-port test harnesses and ignored development evidence are retained separately.

The earlier helper verification predates the native Windows application.
Production account authentication, GUI pinentry, external attachment applications,
IMAP, synchronization races and clean-machine packaging still need validation. Fixed scratch filenames retain their
concurrency limitations. The calendar parser is event-focused, not a general
recurrence engine; unsupported custom VTIMEZONE rules fail. Generated timezone
serialization can differ while preserving the tested calendar properties. Small
native Lynx layout differences were accepted. Fixture tests establish the tested
cases rather than universal behavioral equivalence.

## Submission reliability checks

The shared SMTP implementation distinguishes a lost final DATA acknowledgment
from a definite rejection. If the terminator may have reached the server but no
valid response arrives, it reports that delivery status is unknown and the server
may have accepted the message. It retains the draft and does not automatically
resend. A received acceptance remains successful if the connection closes during
QUIT. Recording follows acknowledged submission.

`tests/check_smtp.py` covers isolated authentication/TLS and archive consistency,
including Unicode calendar/image attachments. The Windows and Linux SMTP outcome
UI checks exercise compose/send, retained draft content, no automatic resend and
archive counts. Native GPGME and classic GPG reader checks use disposable keyrings
and genuinely encrypted messages.

A same-message Maildir flag conflict remains an upstream limitation: an external
rename can replace a pending local flag change. This was reproduced in stock Linux
NeoMutt, the shared Linux build and Windows. Message contents remained intact;
concurrent changes to different messages passed the tested cases.

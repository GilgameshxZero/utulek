# neomutt-xp: configuration compatibility

Shared configuration helpers for Linux and a future native Windows NeoMutt.
The configuration compatibility changes are installed and accepted. This project
does not yet contain a Windows NeoMutt executable.

## Runtime design

Keep commands inline when the tools exist on both platforms: ls, rm, iconv,
xdg-open, mpv and Lynx. The configuration resolves my_active_dir using $HOME
before invoking commands, quotes helper/token paths and sources GPG stdout
directly. Password loading needs neither a script pair nor a plaintext scratch
file. Calendar saving uses inline Python standard-library UUID generation and
shutil.move.

| Source | Purpose |
| --- | --- |
| runtime/neomutt-html-open and .bat | One shared Bash HTML wrapper; Windows defaults BROWSER to xdg-open.bat and respects an explicit override. |
| runtime/neomutt-ical-request and .bat | One shared interactive request generator; Windows UUID generation uses PowerShell. |
| runtime/neomutt-query and .bat | Preserve echo/xargs/khard query semantics; Windows selects khard.exe because xargs cannot execute khard.bat. |
| python/mutt-ical.py | Display invitations and generate replies with the existing prompts and response behavior. |
| python/ical_support.py and timezones.zip | Standard-library calendar support and bundled IANA timezone data. |
| python/mutt_oauth2.py | Existing OAuth provider/token flow with cross-platform command and private-storage handling. |
| python/command_args.py and private_storage.py | Platform command splitting, Unix permission checks and Windows ACL checks. |
| lynx.bat | Launcher for native Windows Lynx under app/lynx. |

The .bat launchers call the shared scripts through the existing Git for Windows
Bash. They retain the working directory, inherit environment variables and
forward exit status. NEOMUTT_FROM and BROWSER cross that boundary; setlocal keeps
wrapper changes local without removing inherited values. There are no separate
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
until successful decryption. The shared xdg-open launcher is unchanged.

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
opened during automated verification. Temporary fixtures, test harnesses,
migration scripts, downloads and rollback files were removed after acceptance;
only runtime source, dependency metadata, licenses and documentation remain.

There is no Windows NeoMutt TUI/config parser to test yet. Live authentication,
pinentry, real sending/viewers and the future native application's process
launching still require validation. Fixed scratch filenames retain their
concurrency limitations. The calendar parser is event-focused, not a general
recurrence engine; unsupported custom VTIMEZONE rules fail. Generated timezone
serialization can differ while preserving the tested calendar properties. Small
native Lynx layout differences were accepted. Fixture tests establish the tested
cases rather than universal behavioral equivalence.

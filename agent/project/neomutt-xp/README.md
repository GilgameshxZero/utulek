# neomutt-xp: configuration adapters

This first stage adapts the existing email configuration for a future native
Windows NeoMutt. It does not contain a NeoMutt executable. The candidate is
staged; the production configuration is not installed or executed by these tools.
Keep testing isolated. The user explicitly deferred live installation: testing
does not require installing the candidate into the active email module.

The current scope is command launching, shell flows, and configuration glue.
Portability and removal of third-party dependencies inside the existing Python
scripts are explicitly deferred. Do not rewrite those scripts in this stage.
Preserve the existing workflow, including prompts, timezone interpretation,
scratch filenames, output formats, failure behavior, and attachment actions.
Test against the originals rather than inferring behavior from their comments.
Do not introduce new Python packages.

## Runtime design

The executable `runtime/neomutt-command` and its `.bat` counterpart share an
extensionless entry point. Unix launches the executable script; Windows cmd
selects the batch launcher, which runs the same script with the installed Bash.
The Windows test machine already supplies Bash, coreutils, iconv, GPG and
cygpath through Git for Windows, and PowerShell through Windows.

| Flow | Adapter behavior |
| --- | --- |
| Helper paths | Resolve the email directory through NeoMutt's `cd` and emit an absolute native path; quote command paths. Restore the original snowfall working directory before other operations. |
| Vault mailbox | Keep the directory-existence check with `ls`; emit a native Windows path rather than an MSYS path. |
| OAuth | Explicitly invoke the existing Python script; retain its token and GPG arguments. Python internals are deferred. |
| Encrypted settings | Run existing GPG and feed successful output to NeoMutt's source-pipe interface. No plaintext `/tmp` file or timed deletion. A GPG failure emits no partial settings. |
| Address queries | Preserve the existing echo/xargs/khard `--parsable` behavior, including its status-line behavior and errors. Windows xargs invokes the installed native khard executable because it cannot execute the batch launcher directly. No migration to `--mutt` in this parity pass. |
| Calendar capture and removal | Run the same iconv and rm commands inside Bash. |
| Calendar save | Keep mv and UUID filenames; Windows UUIDs use PowerShell `([guid]::NewGuid()).ToString()` with CR removed. No clipboard operation. |
| Calendar requests | Run the original Bash script unchanged, supplying the Windows UUID implementation as an exported Bash function. The original `date -u -d` behavior is retained. |
| Calendar display/reply | Explicitly invoke the original Python script, preserving argv and current directory. |
| HTML browser | Run the original script unchanged, preserving its wrapper bytes, stable scratch filename, BROWSER override and 64-second cleanup. When no BROWSER override exists, Windows bridges to the existing xdg-open.bat. |
| Other attachments | Keep xdg-open and mpv routes. |
| HTML pager | Keep the exact Lynx command. A compatible Windows Lynx executable is still required; it was absent from the test host's PATH. |
| Mailcap | Expand a source template into a per-user `~/.neomutt/mailcap` containing quoted native helper paths. |

The original Python and `.sh` helpers are copied into an ignored staging tree,
not tracked again here. `stage_config.py` reads the current configuration and
fails if expected command patterns have changed. It does not copy tokens,
encrypted credentials, mailboxes or signatures. The generated configuration
retains the source's account/path settings and therefore belongs in ignored
`.obj/`, never in the public source tree.

## Validation

`test_adapters.py` compares adapter behavior against the original commands with
disposable fixtures. It records command arguments instead of accessing contacts,
decrypting credentials, contacting providers, or opening actual attachments.
Tests cover extensionless dispatch, spaces/Unicode/shell punctuation, native
paths, UTF-8 capture including invalid input and exit status, password output and
failure, Python argv/cwd, UUID moves, query argument/error parity, calendar
request output and failures, and HTML wrapper/opener parity. Random UUID and
DTSTAMP values are normalized only for request comparisons. HTML tests substitute
the timer and deletion commands; they do not measure the actual 64-second delay.

`check_config.py` compares all parsed settings with Linux NeoMutt, synthetic
Maildirs, and a fake GPG command. Only the intended external-command settings may
differ; neither configuration may produce parser diagnostics. It also checks
that all copied Python and original shell scripts remain byte-identical.

`argv_observer.go` is a test fixture compiled to a Windows executable for query
argument recording; it adds no runtime dependency. `run_windows_tests.py` uploads
only the fixtures and original shell scripts to an isolated temporary directory
and removes it afterward. Host writes require explicit authorization.

Private results are in `.obj/linux_results.json`, `.obj/windows_results.json`,
and `.obj/config_check/results.json`.

`test_tui.py` exercises the actual attachment-save macro in disposable Linux
Maildirs. Indentation tabs inside the original macro trigger completion in the
command-entry field; there it falls back to filename completion, not an address
book lookup. This is directory-dependent: an initially empty working directory
and a directory with one ordinary file caused malformed commands. With two
ordinary filenames having no shared prefix, the unchanged original and candidate
both saved byte-identical ICS files and removed the capture scratch file. See
`.obj/tui_multiple_files/results.json`. This is not evidence that the user's live
workflow is broken. The user initially requested preserving that whitespace, then explicitly
authorized fixing the live neomuttrc to use unindented continued strings. That
formatting-only fix is now applied; layout spaces between macro actions were
also removed while command argument separators were preserved. Both original
and adapted versions pass the real save-macro TUI check in an empty directory;
see `.obj/formatting_tui_check/results.json`. The Windows adapters remain
uninstalled. All future continued multiline strings in neomuttrc should start
at column one inside the quoted value.

## Limits before declaring full Windows parity

There is no native Windows NeoMutt yet. The port must preserve configuration,
macro, mailcap and source-pipe behavior, expand home directories, launch cmd
command strings correctly, and implement Windows escaping for queries and MIME
filenames. Adapter tests cannot establish those core behaviors. In particular,
arbitrary percent-variable strings and embedded quotes require testing through
the eventual native launcher, not merely through a batch wrapper.

The installed Windows Python lacks vobject; this is an existing dependency of
the original calendar helper and belongs to the deferred Python work. OAuth's
POSIX permission checks likewise remain deferred. Live GPG/pinentry, account
authentication, message sending, actual default viewers, full TUI macro execution
and concurrent scratch-file behavior have not been validated. The unchanged
scripts retain their existing fixed-filename/concurrency limitations.

Passing fixture tests is evidence for the tested behavior, not a proof of exact
equivalence for every input, environment, or future Windows NeoMutt build.

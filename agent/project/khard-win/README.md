# khard-win

A standalone Windows command-line address book using khard's configuration
format and local vCard files. This is a Go reimplementation, not an upstream
khard source port. No Python, WSL, Go installation or third-party runtime DLL
is needed on Windows. An external editor is needed for interactive editing.

Version 0.1.1-preview adds an explicit NeoMutt email-query protocol to the
initial user-feedback milestone.

## Configuration shared with Linux

Copy the same `khard.conf` to `~/.config/khard/khard.conf` on either platform.
On Windows, `~` is the current user's home directory, normally
`C:\Users\<user>`. Configuration lookup, in order:

1. `-c FILE` / `--config FILE`.
2. `KHARD_CONFIG`.
3. `XDG_CONFIG_HOME/khard/khard.conf` if that environment variable is set.
4. `~/.config/khard/khard.conf`.

The parser accepts ConfigObj sections, nested `[[address book]]` sections,
quoted values, comments, boolean values, comma lists, and trailing commas.
Windows backslashes are preserved. Home-relative paths with `/` work on both
platforms. Relative paths resolve from the caller's working directory, as in
upstream khard; use `~/...` to avoid that dependency. Absolute Linux paths
and editor executables still need to exist on the platform using the file.
Use a comma list such as `editor = vim, -i, NONE` to avoid command-line
quoting differences. Single-string editor commands support quoted tokens;
shell expansions, pipelines and escape sequences are not executed.

Recognized settings include all settings in `khard.conf.example`, plus
`preferred_phone_number_type`, `preferred_email_address_type`, `debug`, and
configured `private_objects`. `merge_editor` is accepted for interchange but
unused until merge is implemented. `localize_dates` is accepted, but this
preview displays dates in their stored ISO form. `search_in_source_files`
is accepted; searches always use the parsed contact. Debug is currently
accepted without verbose logging. ConfigObj multiline quoted values are
not implemented. The app never rewrites your configuration.

## Commands

* `list` / `ls`: contacts, optional search, address-book filter, sorting and
  display options. `-p` prints UID, name and book separated by tabs.
* `show` / `details`: select one match and display it. `--format yaml` and
  `--format vcard` export editable YAML or the exact source vCard. `-o FILE`
  writes a new file and refuses to overwrite an existing one.
* `phone`, `email`: list all matching phone numbers or emails. `-p` prints
  value, name and label separated by tabs. Untyped properties retain an empty
  label instead of upstream's implicit voice/internet labels.
* `postaddress`, `birthdays`, `filename`, `addressbooks`: basic listings.
* `template`: print a YAML contact template.
* `new` / `add`: create through the configured editor, `-i FILE`, or piped YAML.
* `edit` / `modify` / `ed`: edit one selected contact, using the editor,
  `-i FILE`, or piped YAML. `--format vcard` edits the original vCard directly.
* `remove` / `rm` / `delete`: remove one selected contact after confirmation.
  `--force` skips confirmation but still refuses ambiguous matches without
  an interactive selection.

Search terms are case-insensitive and ANDed. Field searches include
`name:`, `formatted_name:`, `phone_numbers:`, `emails:`, `addresses:`,
`notes:`, `uid:`, and the YAML top-level field names with underscores.
An unrecognized field prefix is treated as a literal general search term,
as in khard. Arbitrary query operators and nested `-F` expressions are not
implemented. A literal `--` ends option processing.

Multiple matches prompt for selection when input comes from a terminal.
Piped operations must identify exactly one contact; `uid:<UID>` is useful.
When multiple books are configured, `new` requires `-a BOOK`.

## NeoMutt query integration

Use the explicit `email --mutt` mode for NeoMutt, rather than `--parsable`
alone. Success always starts with a blank, newline-terminated status line,
including when there are no matches. Subsequent UTF-8 lines contain exactly
`email<TAB>display-name<TAB>description`; the description is the existing
email label. This preserves the first result when NeoMutt consumes the
status line. Tabs, newlines and other ASCII control characters inside fields
are replaced with spaces, as in the existing parsable output. Failures emit
one useful sanitized status line on stdout; the executable also writes the
full diagnostic to stderr and exits with code 1. Results are buffered until
the query succeeds so a failure cannot leave partial results on stdout.

The calling integration should construct these arguments directly:

```text
email --mutt -d formatted_name -- <complete query as one argv argument>
```

Only `--mutt` mode splits whitespace inside each query argument into the
existing ANDed search terms. Empty/whitespace-only queries match all contacts.
Apostrophes, quotation marks, backslashes, Unicode and shell metacharacters
are literal characters, never shell syntax to khard-win. Quotation marks
inside the delivered query do not group phrases or disappear. Multiple query
arguments are also supported. `--` protects queries that resemble options.
Configured address books, `-a`, display/sort options and field restrictions
work as for ordinary email searches. `--parsable` without `--mutt` remains
headerless and does not gain whitespace splitting. If both flags are supplied,
the explicit NeoMutt protocol takes precedence.

Windows executable: `monochrome/app/khard.exe`; Windows launcher:
`monochrome/bin/khard.bat` (callable as `khard` on PATH). Prefer the `.exe`
for programmatic invocation to avoid an additional batch shell. The Linux
build of this implementation is `project/khard-win/.bin/khard`; the existing
system Linux khard is a different program and does not acquire this flag.

The NeoMutt integration owns `%s` substitution, platform-specific quoting,
and `query_command`. The argv above is an interface contract, not a
copy-paste shell/config expression. Passing the query as one argument removes
the need for the old `echo`/`xargs` pipeline; it does not make arbitrary raw
`%s` shell substitution safe. Use a direct process API where available or
the correct platform shell-escaping implementation. No NeoMutt configuration
is changed by this feature.

## Contact editing

YAML uses the familiar khard fields, including structured names, phone,
email, postal addresses, organization, dates, categories, webpages, private
objects and notes. Strings remain strings, including unquoted phone values
and dates. Unknown fields, duplicate keys, aliases, extra YAML documents and
unsupported structures are rejected. This preview does not validate every
vCard date or property syntax rule; inspect unusual imported data carefully.

An empty label (`""`) represents an unlabeled phone or email:

```yaml
Formatted name: Example Person
Phone:
    "": '+15550100001'
Email:
    work: example@example.invalid
Note: |
    First line
    Second line
```

No `CELL` label is inferred. Phone labels already present are preserved.
YAML input files may contain only fields to update: missing fields are left
alone, while explicit null/empty values clear fields. This partial-update
behavior differs from upstream's replacement-template behavior. `Private`
is treated as a complete mapping of the configured private objects when
supplied. `--open-editor` opens input for review before saving.

Unchanged properties keep their exact original physical bytes, including
unknown properties, photos, extension parameters and folded lines. Unchanged
YAML round trips perform no write. Changed phone/email/address entries reuse
their original property headers when their labels match, preserving extra
parameters and group associations. Exotic custom labels that cannot be
represented as type tokens can remain untouched in YAML; use `--format vcard`
to change those fields directly. Arbitrary unknown properties are available
for direct vCard editing. New properties use UTF-8, CRLF and 75-octet folding.

Only vCard 3.0 and 4.0 are editable. New contacts receive a random UUID and
UTC revision. Existing UIDs remain unchanged. Only nonhidden `.vcf` files
with one vCard each are loaded; the separate combined compatibility export
is not a configured address book and is not regenerated.

Saves stage data in the contact directory and replace the destination
atomically. Windows uses `MoveFileExW` with write-through. A cooperating
writer lock and an original-content comparison detect concurrent edits;
external sync tools do not share the lock, so a small race remains between
the last comparison and replacement. Symlinks and nonregular contact files
are not modified. Stale `.khard-win.lock` files from a crashed process cause
a clear error rather than an automatic lock override. Failed interactive
edits leave the private temporary input file available for recovery and
print its path.

## Build and install

Go 1.24+ is needed to build. GNU Make targets `all`, `windows`, `test`, and
`test-windows` build the Linux reference executable, Windows x64 executable,
local tests, and Windows test executable. Dependencies and build caches stay
under the ignored `.obj/` directory. YAML is pinned to
`gopkg.in/yaml.v3 v3.0.1`, authenticated by `go.sum`; its license is in
`licenses/yaml_v3.txt`.

`scripts/install.py` installs `.bin/khard.exe` to
`~/main.syncthing/monochrome/app/khard.exe` and an argument-forwarding
`monochrome/bin/khard.bat`. `--root` selects a different monochrome root.
It does not replace the existing Linux khard executable.

`scripts/windows_smoke.ps1` runs the compiled app against a disposable
address book, including config lookup, create/edit/remove, Unicode, paths
with spaces, and optional batch-launcher argument and exit-code checks.
The Windows regression executable additionally checks editor invocation,
conflict detection, no-op preservation and vCard edge cases. Neither test
suite writes production contacts. `scripts/compare_readonly.py` compares
list/phone/email identities against an installed upstream khard and retains
only aggregate results in `.obj/readonly_comparison.json`.

## Current limitations / feedback

Merge, copy, move and add-email explicitly return an unimplemented error.
Full upstream command/output parity, locale-specific name sorting, date
localization, nested/custom `-F` fields, and translated Unix editor/shell
commands are outside this first preview. This program manages local files;
Syncthing or another existing sync process remains responsible for syncing.

The feedback milestone is actual Windows terminal use: list readability,
ambiguous-contact selection, search behavior and a create/edit/remove cycle
for a disposable contact using your normal Vim setup.

## Local build output

Intermediates, build artifacts, dependencies, caches, and generated test reports belong in `.obj/`. Finished binaries belong in `.bin/`. Both directories are local to this project and ignored by Git. `make clean` removes both directories.

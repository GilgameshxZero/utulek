# khard-xp

A standalone Linux and Windows command-line address book using khard's configuration
format and local vCard files. This is a Go reimplementation, not an upstream
khard source port. No Python, WSL, Go installation or third-party runtime DLL
is needed on Windows. On Windows, interactive editing opens the default text-file application unless
an editor is explicitly configured. Save the file, then press Enter in khard.
Temporary files end in `.yaml.txt` or `.vcf.txt` so they open as text rather
than being imported by a contacts application. With no configured editor,
Linux uses `EDITOR`, then Vim.

Version 0.1.5-preview completes the staged compatibility work against khard
0.22.0, including native locale handling, Chinese collation/display support,
replacement YAML editing and the current-upstream command audit.
See [UPSTREAM_COMPATIBILITY.md](UPSTREAM_COMPATIBILITY.md) for stages and exceptions.

## Configuration shared with Linux

Copy the same `khard.conf` to `~/.config/khard/khard.conf` on either platform.
On Windows, `~` is the current user's home directory, normally
`C:\Users\<user>`. Configuration lookup, in order:

1. `-c FILE` / `--config FILE`.
2. `KHARD_CONFIG`.
3. `XDG_CONFIG_HOME/khard/khard.conf` if that environment variable is set.
4. `~/.config/khard/khard.conf`.

The parser accepts ConfigObj sections, nested `[[address book]]` sections,
quoted values (including triple-quoted multiline scalars), comments, boolean
values, comma lists, and trailing commas. Quoted whitespace and backslashes
are literal.
Windows backslashes are preserved. Address-book paths expand environment
variables (`$NAME`, `${NAME}`, and Windows `%NAME%`), retaining unknown references.
Explicit `type = vdir` is accepted. Home-relative paths with `/` work on both
platforms. Relative paths resolve from the caller's working directory, as in
upstream khard; use `~/...` to avoid that dependency. Absolute Linux paths
and editor executables still need to exist on the platform using the file.
Use a comma list such as `editor = vim, -i, NONE` to avoid command-line
quoting differences. Single-string editor commands support quoted tokens;
shell expansions, pipelines and escape sequences are not executed.

Recognized settings include all settings in `khard.conf.example`, plus
`preferred_phone_number_type`, `preferred_email_address_type`, `debug`, and
configured `private_objects`. `merge_editor` receives the source YAML file
first and the editable target YAML file last. When unset, `MERGE_EDITOR` is
used if defined; otherwise the normal editor opens the target with the source
included as a commented reference. `localize_dates` is accepted, but localized
presentation uses native locale data when enabled. YAML dates remain ISO.
Discovered address books support recursive globbing, leaf directories and
collision suffixes. Interpolation stays disabled, matching upstream 0.22.0. The app never rewrites your configuration.

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
as in khard. Public upstream fields such as `post_addresses:`, `organisations:`,
`roles:`, `titles:`, `kind:`, and `version:` are supported too. `name:` checks
both first-name and last-name ordering. `phone_numbers:` ignores punctuation
and handles upstream's local/international number matching rules.
General searches cover upstream-visible names, headings, address-book names,
known values and configured private properties. Unknown vCard properties,
photos, raw syntax, REV, and unconfigured private values are excluded.
An unknown prefix, including its colon, stays part of the general query.
Public field queries match values and mapping labels; date-valued `birthday:`
and `anniversary:` require an exact ISO date, with invalid queries reported as
errors when a date-valued contact is examined. Reduced dates use year 1900,
as upstream does. Date-times only match a date at naive midnight; text-valued
dates use substring matching. vCard 3 X-ANNIVERSARY is supported.

`-f` / `--search-in-source-files`, or `search_in_source_files = yes`, first
matches query values against raw file text, skips rejected files before parsing,
and then checks parsed contacts. This is faster but can miss folded/escaped
values, display labels and normalized dates. Malformed rejected files are not
parsed; disable this option for complete parsed-contact search/error reporting.
`--debug`, or `debug = yes`, writes book loading counts and prefilter/parse/match
counts to stderr without changing stdout protocols. Debug output is specific
to khard-xp rather than an upstream logging transcript.

Existing YAML field aliases (`phone:`, `email:`, `addresses:`, `private:`, and
others) and case-insensitive field prefixes remain extensions. All nicknames
in a comma-separated property are retained, unlike the installed upstream
0.19.1/vobject stack, which exposes only the first. Search uses upstream's
implicit voice/internet/home labels without adding those labels to stored data
or to otherwise untyped TSV output.

A literal `--` ends option processing.

`list -F` accepts upstream property names and nested paths such as
`emails.work.0`, `post_addresses.home.0.city`, and `organisations.0.1`.
Indices start at zero; missing entries produce an empty column. Custom columns
replace the default columns in both readable and parsable output. An empty
`list` exits with status 1, with no output in parsable mode.

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
are literal characters, never shell syntax to khard-xp. Quotation marks
inside the delivered query do not group phrases or disappear. Multiple query
arguments are also supported. `--` protects queries that resemble options.
Configured address books, `-a`, display/sort options and field restrictions
work as for ordinary email searches. `--parsable` without `--mutt` remains
headerless and does not gain whitespace splitting. If both flags are supplied,
the explicit NeoMutt protocol takes precedence.

Windows executable: `monochrome/app/khard.exe`; Windows launcher:
`monochrome/bin/khard.bat` (callable as `khard` on PATH). Prefer the `.exe`
for programmatic invocation to avoid an additional batch shell. The Linux
build of this implementation is `project/khard-xp/.bin/khard`; the existing
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
and dates. Unknown fields, duplicate keys, extra YAML documents and unsupported
structures are rejected. Anchors/aliases are supported with bounded expansion
and nesting; cycles are rejected. Dates are validated on changes. Tagged
values, nested categories, Kind and Impp are supported.

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
YAML edits now use replacement semantics: omitted managed fields are cleared.
Omitted Formatted name retains the existing FN, as upstream does; explicitly
empty FN requests automatic generation. Replacement input requires a name or
organisation. Use `edit --patch` for partial updates that preserve omitted
fields. Explicit null/empty values clear fields in either mode. `Private`
is treated as a complete mapping of the configured private objects when
supplied. `--open-editor` opens input for review before saving.

Unchanged properties keep their exact original physical bytes, including
unknown properties, photos, extension parameters and folded lines. Unchanged
YAML round trips perform no write. Changed phone/email/address entries reuse
their original property headers when their labels match, preserving extra
parameters and group associations. Unicode custom labels, including Chinese labels, are supported. Unknown
fields and unchanged metadata remain intact. Arbitrary unknown properties are available
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

Go 1.24+ is needed to build. Linux native locale support requires cgo and a C
compiler, and uses the host libc and installed locale data. Windows uses native
NLS APIs with no additional DLLs. A cgo-disabled Linux build supports only C
locales and explicitly rejects unsupported locales. GNU Make targets `all`, `windows`, `test`, and
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
The Windows regression executable additionally checks source-search/debug
stdout/stderr behavior, multiline config, field/date queries, editor invocation,
conflict detection, no-op preservation and vCard edge cases. Neither test
suite writes production contacts. `scripts/compare_readonly.py` compares
list/phone/email identities against an installed upstream khard and retains
only aggregate results in `.obj/readonly_comparison.json`.

## Contact operations

`copy` and `move` select a source using ordinary search terms and `-a`, and
select a destination using `-A` / `--target-addressbook`. Copies receive a new
UID; moves retain the existing UID. Both update REV and preserve other vCard
properties. An existing destination filename is never silently replaced.
Same-name destinations require an interactive choice: add another entry,
merge, overwrite, or cancel. Overwrite installs the transfer before removing
the old destination. A failed later removal keeps the saved data and reports
partial completion; inspect the named files before retrying.

`merge` selects a target using `-t` / `--target-contact` / `--target` and optional
`-A`. Edit the target template to incorporate the desired source values;
merging is manual, as in upstream. An unchanged target leaves both files
untouched. Confirmation saves the target with its UID and removes the source.
Unknown target properties remain intact. Source-only fields must be considered
before confirming deletion. Editor, parsing, conflict and save failures preserve
the target draft and report its location. Invalid YAML exits with recovery
rather than repeatedly reopening the editor.

`add-email` reads a message or header block from `-i FILE` or stdin. `-H` /
`--headers` selects comma-separated address headers (default `from`, or `all`).
Encoded names, folded headers and address groups are supported. Confirm each
address, select or search for an existing contact, or create a contact with
name/organisation prompts. New contacts may use any configured book, as in
upstream. Choose the email label (default `internet`); custom labels use grouped
X-ABLabel properties and preferences follow the vCard version. Existing phone
and email labels remain untouched. `--skip-already-added` skips known addresses
without prompting or writing. A batch loads the selected books once and indexes
known emails once; repeated addresses in the same input are handled once.
Use `-i FILE` when the terminal must remain available for answers. Prompts and
cancellation status are not byte-for-byte upstream equivalents: cancellation
returns nonzero. Confirmation-dependent operations refuse unattended execution.

`scripts/compare_operations.py` compares disposable-book mutations against
installed upstream khard on Linux, using a PTY for interactive input. It retains
semantic vCard snapshots and command results in `.obj/operation_compatibility.json`.
Generated UIDs, REV timestamps and property order are excluded from equality;
upstream's inferred voice label in a rewritten merge is an explicit exception.

`scripts/compare_search.py` compares general/public/date/private queries in
both parsed and raw-prefilter modes, plus config/debug cases, against upstream
using disposable books. Detailed results and named compatibility extensions
are retained in `.obj/search_compatibility.json`. Queries are compiled once per
load; simple terms check visible scalars and labels directly, and only complex
phrases render the complete search text. An invariant regression check compares
the simple-term fast path with that full representation.

## Locales and current-upstream audit

Locale selection follows LC_ALL, then LC_COLLATE/LC_TIME, then LANG. Windows
uses the user's native locale when no override is set. POSIX Chinese names
`zh_CN.UTF-8` / `zh_TW.UTF-8` map to Windows `zh-CN` / `zh-TW` where appropriate.
Linux uses glibc collation/date data; Windows uses NLS and honors user date
preferences. Sort keys are computed once per contact and once per book.
CJK table cells count wide characters correctly; combining marks add no width.
YAML always uses ISO dates; localized dates appear in readable output/search.

The release audit uses isolated khard 0.22.0 and dependencies pinned in
`scripts/upstream_requirements.txt`, installed under ignored `.obj/upstream-current/`.
`scripts/audit_current.py` covers commands, aliases, option inventory, output,
collections, discovery and seven locales, including Simplified/Traditional
Chinese. `scripts/audit_editing.py` verifies saved YAML/new-contact semantics.
`scripts/windows_locale_smoke.ps1` independently compares Chinese sorting and
dates with Windows/.NET NLS and edits Chinese labels through the executable.
Detailed reports, native Windows logs and benchmarks remain under `.obj/`.

This remains a compatible reimplementation with deliberate differences, listed
in [UPSTREAM_COMPATIBILITY.md](UPSTREAM_COMPATIBILITY.md). It preserves valid
nickname lists, tagged private values, precise date offsets and yearless leap
days rather than reproducing upstream data loss/rejection bugs. Empty labels
and headerless ordinary email TSV remain supported; `--mutt` has its separate
status-line contract. Editor prompts and help/log wording differ. Unix shell
commands are not translated for Windows. The default Windows GUI editor's
manual save/confirmation flow still needs an interactive user check; configured
editor and recovery paths are automated-test covered. Syncthing remains
responsible for synchronization.

## Local build output

Intermediates, build artifacts, dependencies, caches, and generated test reports belong in `.obj/`. Finished binaries belong in `.bin/`. Both directories are local to this project and ignored by Git. `make clean` removes both directories.

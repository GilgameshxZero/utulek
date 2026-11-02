# Upstream compatibility

This is a staged compatibility effort, not a claim of full upstream parity.
Executable comparisons currently use installed khard 0.19.1; the current
upstream documentation is also checked for changed behavior. The supported
interface should be tested against a newer upstream release before calling
this a complete compatible replacement.

## Stage 1: search and list columns — implemented

* Phone queries normalize punctuation and support local/international matching.
* Names use upstream's first/additional/family order, or family followed by a
  comma and first/additional names. Prefixes and suffixes are excluded from
  these display orders; the formatted name retains the original FN value.
* `name:` checks both orders, formatted names and nicknames.
* Public search fields include organisations, post_addresses, roles, titles,
  kind and version, in addition to the previously supported fields.
* `list -F` replaces default columns in parsable output too, supports upstream
  public properties and nested dictionary/list paths, and leaves missing
  entries empty. A per-contact cache avoids rebuilding shared column values.
* Empty lists exit with status 1; parsable output stays empty.

`scripts/compare_compatibility.py` exercises disposable synthetic contacts
against upstream and the native build, retaining expected and actual rows,
exit codes and known exceptions in the ignored JSON report. Go regression
coverage runs on Linux and Windows, including subprocess checks when
KHARD_XP_CHECK_EXECUTABLE is set.

## Stage 2: missing contact operations — implemented

Copy and move support source selection and `-A` destination selection. Copies
receive a new UID; moves retain it. Shared staged installation validates the
book, updates REV, refuses filename collisions and rechecks the source under a
lock. Duplicate-name destinations support add, merge, overwrite and cancellation.
Overwrite installs first; partial removal failures retain saved data and report
what remains. Unsafe UID filenames are rejected.

Merge supports `-t` / `--target-contact` / `--target` and `-A`, two-file
`merge_editor` invocation, and a normal-editor fallback with a commented source
reference. The target UID and unknown target properties are preserved. No-op
merges write nothing. Confirmation saves the target before removing the source;
failures preserve the draft and detect stale source and target data.

Add-email supports `-i`, `-H` / `--headers`, `all`, and
`--skip-already-added`, encoded names, folded headers, address groups, existing
contact selection/search, new contacts, custom labels and vCard preferences.
A batch loads its books and indexes known email addresses once.

Regression tests exercise these operations on both platforms. The Linux
`scripts/compare_operations.py` compares semantic results against upstream on
disposable books, excluding generated UID/REV and serialization order. The
Windows smoke script exercises copy/move and add-email skip through the binary.

Interactive prompts are not exact upstream transcripts. Invalid merge YAML
exits with a recovery draft instead of reopening in a loop; cancellation is
nonzero. Repeated mail addresses are processed once. Unattended confirmation
is refused. Both implementations require manually editing the target to select
source values; source-only properties are not automatically unioned. Newer
upstream auditing and the general YAML semantics decision remain in stage 4.

## Stage 3: configuration and search semantics — pending

Honor source-file search and debug options rather than merely accepting them;
add ConfigObj multiline quoted values. Compare exact public/unknown field
semantics, date queries, configured private objects and general searches
against upstream. Existing extra YAML field aliases remain extensions for
compatibility with this preview's callers. Case-insensitive extensions should
be documented rather than confused with upstream guarantees.

## Stage 4: presentation, editing and release audit — pending

Finish locale collation and date localization, pretty/show/address/birthday
output, preferred-label/value selection, tagged properties, collection order
and quoting, short UID formatting and human-readable table parity. Compare
all commands, options and YAML shapes with a supported current upstream
release. Editing currently accepts partial YAML updates; upstream replacement
semantics need a deliberate compatibility decision and preservation tests.

## Deliberate platform and data-preservation differences

* Windows defaults to the associated text application with explicit completion
  confirmation. Explicit editor commands remain supported. Unix shell syntax
  is not translated into Windows shell commands.
* Untyped phone/email properties keep an empty label. Never infer CELL.
* ASCII controls inside TSV columns are replaced with spaces. Upstream may
  emit embedded newlines/tabs; the differential report names this exception.
* Unknown/unchanged vCard bytes are preserved; unchanged edits do not write.
* The explicit `email --mutt` protocol is a supported extension. End-to-end
  NeoMutt integration belongs to neomutt/neomuttwin.

Sources: [upstream command line documentation](https://khard.readthedocs.io/en/latest/commandline.html),
[upstream subcommand reference](https://khard.readthedocs.io/en/stable/man/khard-subcommands.html).

# Upstream compatibility

The four staged implementation scopes are complete in **0.1.5-preview**.
The current release audit targets **khard 0.22.0**, with pinned comparison
requirements in `scripts/upstream_requirements.txt`. This is a compatible Go
reimplementation with the intentional differences below, not byte-for-byte
identity with every upstream behavior.

## Completed stages

* Stage 1: names, phone queries, public search fields and custom/nested columns.
* Stage 2: copy/move/merge/add-email, UID/REV behavior, conflicts and recovery.
* Stage 3: source-file prefiltering, debug output, multiline config, field/date/
  private/general query semantics and native book-path expansion.
* Stage 4: native locale collation and dates; Chinese names/addresses/labels and
  display-cell widths; shared pretty/search rendering; readable tables, preferred
  labels/values, row refinement, birthdays, postal output, collection order and
  quoting, short UID scope, Kind/Impp, tagged YAML and recursive book discovery.
  Command aliases, options, combined short flags and attached values are audited.

## YAML editing contract

Edits replace the managed YAML fields by default: omitted fields clear their
managed vCard properties. Omitted FN retains the current formatted name;
explicitly empty FN requests automatic generation. Replacement input requires
a name/organisation (explicit formatted names are also accepted).
`edit --patch` preserves omitted fields. Null/empty values clear fields.
A supplied Private mapping replaces the configured private values.

Unknown vCard properties, photos, metadata on unchanged values, UID and physical
bytes of unchanged fields remain preserved. Unedited round trips do not write.
Invalid updates are transactional in memory and on disk; editor/save failures
retain a recovery draft. Tagged properties, nested categories, Impp, validated
ISO/text dates and bounded YAML aliases are supported. Cycles, duplicate keys,
unknown YAML field names and multiple documents are rejected.

## Locale and verification

Linux uses per-locale glibc handles (cgo), without changing global process locale.
Windows uses native NLS collation/date APIs and user date preferences. Locale
precedence is LC_ALL, category-specific LC_COLLATE/LC_TIME, then LANG; Windows
uses the user locale if unset. Linux requires installed locale data. A cgo-free
Linux build supports C locales only and reports other locales as unavailable.

Linux differential coverage includes C, US English, German, Swedish, Icelandic,
Simplified Chinese and Traditional Chinese. Native Windows Chinese ordering and
dates are checked independently against .NET/NLS, including an executable edit
with Chinese labels. Each platform uses its own native collation data; platform
and OS-version differences are expected, not a promise of identical ordering.

Queryable reports: `.obj/current_upstream_audit.json` (presentation, parser
inventory and discovery), `.obj/current_edit_audit.json`, and the
`.obj/current_{compatibility,search_compatibility,operation_compatibility}.json`
reports. Actual Windows regression/locale/installed launcher results are in
`.obj/windows_stage4_validation.txt`. Read-only comparisons/audits cover the
production books without writing any contact files. Performance records are
under `.obj/performance_stage4_after.txt`.

## Deliberate differences and boundaries

* Untyped phone/email properties retain empty labels in stored data and TSV;
  never infer CELL. Human pretty/search text may use upstream's implied labels.
* Ordinary email TSV stays headerless; `email --mutt` has its explicit status
  protocol. `--remove-first-line` is accepted. Zero-match mutt queries succeed.
* ASCII controls are sanitized in TSV. Unicode/CJK table padding uses display
  cells, fixing upstream's one-character/one-column assumption.
* Existing YAML query aliases and case-insensitive field names remain extensions.
* Nickname lists are preserved. The audited vobject 0.9.9 still reads only the
  first comma-separated nickname. Upstream 0.22.0 also prints Kind twice and
  cannot re-import certain tagged private exports; these bugs are not copied.
* Valid reduced leap-day birthdays are retained using a leap-year placeholder
  internally, rather than rejected because upstream's placeholder is 1900.
  Source date offsets/precision are preserved; text date exports retain text=
  so they can be imported safely. Explicit null clearing is supported.
* ConfigObj interpolation remains disabled, as in upstream 0.22.0. Discovery
  order/collision naming is deterministic; recursive symlink traversal is avoided.
* Unknown/unchanged vCard bytes are preserved. Default-kind serialization is
  avoided when it would only rewrite an implicit individual default.
* Output files are not overwritten silently. Raw vCard edits are staged and
  checked rather than editing production files directly. Invalid YAML exits
  with recovery instead of reopening in a loop; cancellation is nonzero.
* Windows defaults to the associated text application with explicit completion
  confirmation. Configured editors work on both platforms. Unix shell syntax is
  not translated. Prompts, help, logs and error wording are not exact transcripts.
* Manual default-Windows-GUI save/confirmation remains unverified; automated
  editor invocation, recovery, lifecycle and launcher checks pass.
* NeoMutt end-to-end integration belongs to neomutt/neomuttwin.

Sources: [khard 0.22.0](https://pypi.org/project/khard/0.22.0/),
[configuration](https://khard.readthedocs.io/en/stable/man/khard.conf.html),
[vCard nickname lists](https://www.rfc-editor.org/rfc/rfc2426.html#section-3.1.3),
[Windows sort orders](https://learn.microsoft.com/en-us/windows/win32/intl/sort-order-identifiers).

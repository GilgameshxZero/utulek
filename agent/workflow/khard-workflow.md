# Khard contacts

Contacts live in `~/main.syncthing/monochrome/module/contact/khard/`, with one vCard per file. Configuration is in the adjacent `khard.conf`. The adjacent `contacts.vcf` is a combined compatibility export, not khard's configured source, and can be stale.

When adding contacts, follow the existing convention: use plain `TEL:` without a TYPE label unless the user explicitly requests a label. Do not infer CELL from an SMS conversation. Preserve existing labels when editing unrelated fields.

## Regenerate the merged export

Use the original procedure documented in `monochrome/module/contact.md`: from `~/main.syncthing/monochrome/module/contact/`, run `cat khard/* > contacts.vcf`. This overwrites the combined export with the individual contact files in shell glob order. Do not use khard export or reformat the vCards; concatenate their existing contents. Regenerate when the user requests an updated merged file.

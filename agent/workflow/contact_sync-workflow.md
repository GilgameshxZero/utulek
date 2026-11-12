# Sync contacts to Android devices

Reusable procedure for pushing the vCard collection in
`~/main.syncthing/monochrome/module/contact/` to Android phones over adb.
Device-specific state and current results live in the private context note,
not here.

## Source and merged export

* Individual cards: `monochrome/module/contact/khard/*.vcf`.
* Merged export: `monochrome/module/contact/contacts.vcf`, regenerated from
  the contact directory with `cat khard/* > contacts.vcf` (documented in
  `monochrome/module/contact.md`). It is a plain concatenation, not a khard
  export.
* Treat the cards as read-only unless the user explicitly asks to edit them.

## Dump device contacts

```bash
adb -s "$D" shell 'content query --uri content://com.android.contacts/raw_contacts --projection _id:display_name:account_name:account_type:deleted' > rc.txt
adb -s "$D" shell 'content query --uri content://com.android.contacts/data --projection raw_contact_id:mimetype:data1:data2:data3:data4:data5:data6:data7:data8:data9' > data.txt
```

Android 9's `content query` does not accept `--sort` or `--limit`; projections
are colon-separated. Values can contain commas and newlines (postal
addresses), so parse the output by splitting on `^Row: \d+ ` chunks rather
than by lines.

## Diff rules

* Match a card to a raw contact by email (lowercase), then phone (last 10
  digits), then normalised name.
* Compare: name components (family/given/middle/prefix/suffix), email set,
  phone set, birthday, nickname, note, organisation, URL.
* Do not treat a different stored display-name string as a change when the
  name components are identical. Android aggregates the components (including
  middle names) into the visible name, so `Kevin Fang` and `Kevin A Fang` are
  the same data.
* vCard values need unescaping: `\:` `\,` `\;` `\n`.
* Birthday formats: vCard `2007-01-27`, `--1206`, `19990214`; provider stores
  `2007-01-27` or `--12-06`.

## Update an existing contact

```bash
adb -s "$D" shell 'content update --uri content://com.android.contacts/data --bind "data1:s:New Name" --bind "data2:s:Given" --bind "data3:s:Family" --where "raw_contact_id=RID AND mimetype=\"vnd.android.cursor.item/name\""'
adb -s "$D" shell 'content insert --uri content://com.android.contacts/data --bind raw_contact_id:i:RID --bind mimetype:s:MIME --bind data1:s:VALUE --bind data2:i:TYPE'
```

Follow the device's own conventions:

* email: `data2=1`
* phone: `data2=0`, plus `data4=` with the normalised number
* birthday: `contact_event`, `data2=3`, value `--MM-DD`
* nickname: `data2=1`

Inserted/updated rows are marked dirty and sync to the account provider
automatically.

## Add missing contacts to a local account

Insert a raw contact, then its data rows:

```bash
adb -s "$D" shell 'content insert --uri content://com.android.contacts/raw_contacts --bind "account_name:s:Phone" --bind "account_type:s:Local Phone Account"'
adb -s "$D" shell 'content query --uri content://com.android.contacts/raw_contacts --projection _id:display_name:account_type'
```

`content insert` prints nothing on success; take the last `_id` from the
query. Then insert `name`, `email_v2`, `phone_v2`, `nickname` and `note` rows.
Do not modify USIM/SIM contacts (read-only).

## Import into a Google account (device has no local account)

1. Concatenate only the missing cards into `/sdcard/Download/missing.vcf`.
2. Grant storage to the contacts app, otherwise the import fails with
   `FileNotFoundException: Permission denied`:
   `adb shell pm grant com.android.contacts android.permission.READ_EXTERNAL_STORAGE`
3. Launch the system importer:
   `adb shell am start -a android.intent.action.VIEW -d file:///sdcard/Download/missing.vcf -t text/x-vcard -n com.android.contacts/.vcard.ImportVCardActivity`
4. Dump the UI (`uiautomator dump`), pick the target account row with
   `input tap`, then wait for `VCardImport: Successfully finished importing`
   in logcat.
5. Revoke the permission and delete the pushed file.

Importing into a Google account syncs the new contacts to Google Contacts.

## Dedupe

Delete duplicate raw contacts by URI:

```bash
adb -s "$D" shell 'content delete --uri content://com.android.contacts/raw_contacts/RID'
```

The `--where` form silently did nothing in testing; use the per-row URI. On a
Google account the row becomes `deleted=1` and the deletion syncs to Google;
on a local account it is removed. Verify via the aggregated
`content://com.android.contacts/contacts` view.

## Caveats

* A source card can match several raw contacts; Google accounts can hold the
  same person as separate raw contacts that share only one identifier.
* Never blanket-import the full collection into a device that already has most
  contacts; it duplicates them and can upload duplicates to Google. Add only
  missing cards or update in place.

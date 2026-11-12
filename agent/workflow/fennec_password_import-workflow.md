# Fennec password import workflow

Use when importing a browser password export (Chrome-style CSV) into Firefox for Android / Fennec F-Droid. Verified against Fennec 159.0.0 on Android 12.

## The critical fact: there are two stores

Fenix/Fennec keeps two unrelated login stores:

| Store | Path | Used by |
|---|---|---|
| Gecko profile | `files/mozilla/<profile>.default/logins.json` + `key4.db` | **Not** Fenix's password UI or autofill |
| App-services | `databases/logins2.sqlite` | Passwords screen, autofill, Sync |

Writing a valid Gecko `logins.json` (NSS SDR-encrypted, `encType: 1`) does **not** populate the Fennec password manager. Confirmed by logcat (`SyncableLoginsStorage: Warming up storage`) and an empty UI afterwards. Do not use the Gecko route for Android; it is only useful for priming a desktop Firefox profile for Sync.

## Preconditions

* Root (`su` works) or a debuggable build. Release Fennec is not debuggable, so `run-as` fails.
* Stop the app before reading or replacing the DB: `adb shell am force-stop org.mozilla.fennec_fdroid`.
* Locate the UID for `chown` (e.g. `u0_a226`) with `su -c 'ls -ld /data/data/org.mozilla.fennec_fdroid'`.
* The app-services key is plaintext only on release builds. Fenix sets `forceInsecure = !Config.channel.isNightlyOrDebug`; on Nightly/Debug the key in `core_prefs_kp_post_m.xml` is Android-Keystore-encrypted and direct injection is blocked.

## Key and crypto details

* Key: prefs file `shared_prefs/core_prefs_kp_pre_m.xml`, entry `loginsKey`, an octet JWK (32 bytes base64url).
* Canary: `shared_prefs/loginsCrypto.xml`, entry `canaryPhrase`, JWE of `a string for checking validity of the key`. Decrypting it validates the key.
* `secFields` = JWE compact, `alg=dir`, `enc=A256GCM`, AAD = the base64url header string, 12-byte IV, 16-byte tag.
* Plaintext = `{"u":"<username>","p":"<password>"}` (no spaces).
* Header is exactly `eyJhbGciOiJkaXIiLCJlbmMiOiJBMjU2R0NNIn0`.

## DB schema

SQLite, `PRAGMA user_version = 5`. Relevant tables: `loginsL` (local), `loginsM` (sync mirror), `breachesL`, `loginsSyncMeta`. Reads are the union of L and M, with L overriding M.

For an imported local row:

* `origin` / `formActionOrigin` = `scheme://host[:port]`
* `httpRealm` = NULL
* `timesUsed` = 0, `timeCreated` / `timeLastUsed` / `timePasswordChanged` = now in ms
* `secFields` = JWE above, `guid` = 12-char random URL-safe string
* `local_modified` = now in ms, `is_deleted` = 0, `sync_status` = 2 (New)

## Procedure

1. Back up the DB and key prefs. Copy with root, make the temp files readable, then `adb pull` (do not use `exec-out cat`: it corrupted a binary in testing):

   ```
   su -c 'T=/data/local/tmp/fenback; mkdir -p $T
     cp /data/data/org.mozilla.fennec_fdroid/databases/logins2.sqlite* $T/
     cp /data/data/org.mozilla.fennec_fdroid/shared_prefs/core_prefs_kp_pre_m.xml $T/
     cp /data/data/org.mozilla.fennec_fdroid/shared_prefs/loginsCrypto.xml $T/
     chmod 755 $T; chmod 644 $T/*'
   adb pull /data/local/tmp/fenback/<each file> <local dir>
   ```

2. Run the tool on a copy of the DB (never edit the device DB in place):

   ```
   python3 project/fennec_password_import/inject_appservices.py \
     --csv export.csv --prefs core_prefs_kp_pre_m.xml \
     --canary loginsCrypto.xml --db logins2.sqlite \
     [--overwrite] [--dry-run]
   ```

   Default merges and skips duplicate `(origin, username, password)` triples. `--overwrite` clears `loginsL`, `loginsM`, and `breachesL` first. Every row is decrypted back as a self-test before writing.

3. Push the result, fix ownership, drop stale WAL/SHM:

   ```
   adb push logins2.sqlite /data/local/tmp/l2.db
   su -c 'D=/data/data/org.mozilla.fennec_fdroid/databases
     cp /data/local/tmp/l2.db $D/logins2.sqlite
     chown <uid>:<uid> $D/logins2.sqlite
     chmod 600 $D/logins2.sqlite
     rm -f $D/logins2.sqlite-wal $D/logins2.sqlite-shm /data/local/tmp/l2.db'
   ```

4. Verify by pulling the DB back and checking `SELECT count(*) FROM loginsL;` and that all `secFields` decrypt. Hash-compare the pushed file. Do not launch the app just to verify if Sync is configured; starting it may trigger a sync.

## Sync caveats

* `loginsM` holds entries downloaded from the server. Overwriting it removes them locally.
* Keep `loginsSyncMeta`; clearing it forces a full sync download. Leaving it means the next sync may upload the new local rows and may not immediately re-fetch untouched server records. Either way, server-side records can reappear depending on sync state.
* Imported rows are `sync_status = 2` (New), so Sync will treat them as local additions.

## Verification limits

The Passwords screen requires device unlock even when entries exist. DB-level verification (row counts, decrypting all `secFields`, hash match) is reliable. UI verification requires the device to be unlocked; skip it if the user is not present.

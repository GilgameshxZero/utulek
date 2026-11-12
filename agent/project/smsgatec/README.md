# smsgatec

A shared Linux/Windows terminal client for SMSGate, plus an Android client for e-ink phones. The terminal application logic, gateway protocol, compose/review state machine, attachment handling, configuration, and direct vCard contact reader are shared Go code under `native/`. The Android app under `android/` is a Kotlin/Compose client that implements the same gateway protocol, message model, merge rules, incremental sync cursor, and account semantics.

## Terminal client

Linux: executable `smsgatec`. Windows 10/11: executable `smsgatec.exe`, run in Windows Terminal or the VS Code integrated terminal. Windows uses the system `winsqlite3.dll`, so no extra runtime DLLs are distributed. Linux requires ncursesw, tinfo and SQLite shared libraries.

Configuration is `~/.config/smsgatec/config.json` on both systems for compatibility. `--config` and `SMSGATEC_CONFIG` override it; `SMSGATEC_SERVER`, `SMSGATEC_USERNAME`, and `SMSGATEC_PASSWORD` override connection values. Never put credentials in the project tree. The JSON accepts:

```json
{
  "server": "http://phone-address:8080",
  "username": "your-username",
  "password": "your-password",
  "update_mode": "push",
  "reconcile_seconds": 3600,
  "poll_seconds": 60,
  "timeout_seconds": 20,
  "cache_enabled": true,
  "cache_dir": "",
  "theme": "auto",
  "vcf_dir": "~/main.syncthing/monochrome/module/contact/khard"
}
```

`vcf_dir` points to a directory of `.vcf` files; concatenated cards are also accepted within those files. No khard installation is required. An empty directory setting disables contacts. On Windows an absolute path can use forward slashes, e.g. `C:/Users/0/main.syncthing/monochrome/module/contact/khard`. Names, nicknames, notes and phone values are searchable. Phone numbers without country codes require confirmation of an international number before sending.

Credentials, cached messages and saved attachments use mode 600 on Linux and a protected DACL granting only the current user full access on Windows. Drafts and navigation are never persisted.

## Keys

j/k or arrows navigate. Enter opens a thread, starts compose, finishes compose into review, then sends from review. Escape finishes compose into review or cancels search. Shift+Enter (where mapped) or F4 inserts a newline; bracketed paste never sends. q backs through filters and views before quitting. + opens the new-conversation contact picker; + again enters a phone number. The status line always shows the last known server connection state. / searches. e edits a reviewed draft; a attaches; s edits the subject in review or saves existing media in a conversation; d removes the last draft attachment. o opens media. E invokes the external editor ($VISUAL/$EDITOR, otherwise vi on Linux or Notepad on Windows). r/F5 refreshes; I/F6 imports history without export webhooks; c/F2 configures the gateway; t cycles appearance; ? shows help.

Terminal/editor shortcuts remain owned by the terminal: the app does not request Kitty keyboard mode or xterm modifyOtherKeys.

## Efficient updates and installation

Push is the default. The client listens on port 8766 of the private network address used to reach the phone, registers its own SMSGate webhooks, and removes them on a clean exit. Its HTTPS certificate is issued by SMSGate's CA and renewed when needed; the certificate, private key, and random callback-path secret stay under the private configuration directory's `push/` subdirectory. The phone must be able to reach that address. If listener setup fails, the app reports it and polls at the configured interval, with a 60-second minimum in push mode and backoff on failures. Set `update_mode` to `poll` to explicitly use polling.

Incoming SMS uses the webhook text directly without a follow-up GET. Outgoing status events fetch only the affected message to preserve authoritative multipart-delivery states. Incoming MMS uses the small arrival webhook, followed by a few bounded metadata checks until download completes. The large `mms:downloaded` webhook is deliberately not registered because it includes base64 media. Incoming attachments are downloaded only when opened or saved, and reused afterward. An MMS arriving without an arrival notification is recovered by reconciliation or manual refresh.

Startup and hourly reconciliation use a gateway-scoped saved cursor with a five-minute overlap; they do not repeatedly fetch all history. The cursor advances only after both endpoints succeed and cache writes commit. Initial cache population, explicit `r`/F5, and history import can read full history. Cached outgoing bodies and media are reused, unchanged records avoid SQLite writes, contacts are indexed in memory with file-change checks, and idle views are not rebuilt or repainted. HTTP connections are reused; sends are never automatically retried.

Install binaries as `~/main.syncthing/monochrome/app/smsgatec` and `smsgatec.exe`. Put the executable shell launcher `smsgatec` and Windows launcher `smsgatec.bat` in `monochrome/bin`, quoting paths and forwarding all arguments. See `workflow/project_binaries-workflow.md` in the agent workspace for the general convention.

## Android client

Source: `android/`, Gradle project, package `com.gilgamesh.smsgatec`, version `0.3.1`, minSdk 28, targetSdk 35. Release APK: `android/.bin/smsgatec-release.apk` (about 2.8 MB after R8). The UI is Kotlin/Compose; the terminal's Go code is not linked.

Capabilities matching the terminal client:

* SMS, MMS, and group conversations with contact names, subjects, delivery states, and monochrome media rows. Group threads are keyed by the gateway group ID; the title comes from the participants and each incoming message shows its sender.
* SMS and MMS sending, including an optional subject and multiple attachments (combined limit 20 MiB). Sends are not retried automatically; a failed send stays visible as `Uncertain` or `Rejected`.
* New group conversations by adding 2–10 recipients in the inline To field (contact suggestions or an explicit Add recipient row). Replies to an existing group go to its participants. Group sends use the gateway's multi-recipient MMS path; a locally keyed group is adopted into the gateway group thread when its group ID arrives.
* Multiple accounts with one selected account connecting at a time and its own message cache, media directory, and sync cursor.
* Android system contacts for names and search; there is no vCard directory setting.
* Inline search in the top bar for conversations and thread messages, new-conversation autocomplete, full refresh, and incremental sync with a saved cursor.
* Attachments are downloaded only when opened or saved, cached by content hash, and can be opened in another app or saved through the system file picker.
* One-level Settings with account rows (edit, move up/down, remove via the row menu), server/username/password, update mode, poll/reconcile intervals, timeout, theme (auto/light/dark), and a Paged/Smooth scrolling toggle (paged by default). Settings changes commit when you press Back; there is no confirm button. The account editor also saves on Back, and removing an account lives in the Settings row kebab.
* Account switching from the conversation top bar; Refresh and Settings live in the top-bar kebab menu, anchored under the kebab. Connection status, last sync time, and message/contact counts are shown in the status line.
* Message times display as `yyyy/MM/dd HH:mm` in the device timezone, without a timezone suffix.
* Outgoing messages are right-aligned with a left margin; incoming messages are left-aligned with a right margin.

Platform differences from the terminal client:

* Push is the default update mode. The Android app does not host the HTTPS webhook listener. With an account whose `/health` advertises `backend: google_voice` and the `event-stream` capability (the `smsgate_gv` bridge), push mode consumes the authenticated `/events` stream and reconnects with backoff. Phone gateways fall back to polling at `poll_seconds` (minimum 15) with hourly reconciliation. This avoids a background server and CA-certificate provisioning on a mobile device; updates run while the app process is alive.
* Passwords are never written into the app's `config.json`. They are encrypted with an AES/GCM key in the Android Keystore (`smsgatec-credentials`) and stored in app-private `credentials.enc`. Keystore keys are device-bound, so credentials are entered on each device.
* The terminal's compose/review step maps to the inline composer at the bottom of a conversation: a single SMS/MMS toggle reveals subject and attachment fields, and Send validates the recipient and content. The external editor maps to the system keyboard.

Design: strict black and white with no dialogs (modal work is a fullscreen view), a fixed short top bar on every screen, low padding, no list dividers, no ripple, minimal motion, and compact layouts that fit a 480x800 e-ink screen (verified at 320x533 dp). Scrolling is swipe-driven: Paged (default) turns one page per vertical swipe with no page buttons and no fling; Smooth adds ordinary scrolling. The mode is a Settings toggle and applies to message lists, the settings page, and the account form.

### Build

JDK 21, Gradle wrapper 8.9, Android SDK platform 35 and build-tools 35.0.0. From `android/`:

```
JAVA_HOME=/usr/lib/jvm/java-21-openjdk-amd64 \
ANDROID_HOME=/home/gilgamesh/main.syncthing/utulek/agent/.obj/toolchain/android-sdk \
GRADLE_USER_HOME=/home/gilgamesh/main.syncthing/utulek/agent/.obj/gradle-home \
./gradlew --project-cache-dir .obj/gradle-project-cache :app:publishDebugApk --no-daemon
```

Release builds use `:app:publishReleaseApk` with
`SMSGATEC_SIGNING_PROPERTIES="$PWD/.signing/signing.properties"`. The release
keystore is `android/.signing/release.jks` (ignored by Git); passwords live only
in the ignored `signing.properties`. JVM unit tests: `:app:testDebugUnitTest`
(covers recipient validation, message merging, API
mapping, Java hashes, configuration validation, and ID generation).

From the project root, `make android-debug`, `make android-test`, and
`make android-release` run the same commands with the workspace toolchain paths;
override `JAVA_HOME`, `ANDROID_HOME`, `GRADLE_USER_HOME`, or
`SMSGATEC_SIGNING_PROPERTIES` as needed.

### First-run setup on a device

1. Open the app. `No account` appears until an account is configured.
2. Settings (gear icon) → + opens the account form. Enter a name, the SMSGate server
   (for the phone hosting SMSGate itself, `http://10.8.58.57:8080` or
   `http://127.0.0.1:8080`; for the Google Voice bridge,
   `http://10.8.47.42:8081`), username, and password. Use Test connection, then
   Save.
3. Allow the contacts permission when prompted; names come only from the
   Android system contacts provider.
4. Choose update mode. `Push` uses the Google Voice bridge event stream when
   available; phone gateways poll. Poll mode always polls.

`android/.obj/` and `android/.bin/` are ignored local build directories. Both
the Android client and the terminal client remain subject to the same rule:
never place credentials in the project tree.

## Build and test (terminal)

The source module is `native/`. Its Makefile builds the Linux binary. A Windows amd64 executable can be cross-compiled from the same directory with GOOS=windows, GOARCH=amd64 and CGO_ENABLED=0 using Go 1.24 or later. Windows regression tests can similarly be compiled with `go test -c` and run on Windows. Ordinary tests use fake HTTP gateways and temporary state; live tests require explicit SMS_LIVE_CONFIG and SMS_LIVE_NUMBER (the international recipient for sends and sender for inbox checks), with a durable journal preventing automatic repeated sends. `SMS_LIVE_JOURNAL` and `SMS_LIVE_CACHE` select isolated live-test state without removing earlier journals. Opt-in callback checks use `SMS_PUSH_CONFIG`; incoming-MMS checks additionally require `SMS_MMS_CAPTION`. Ordinary tests never send texts or change phone settings. Compiled executables remain local and are ignored by Git.

`--demo` is offline and never sends. `--check` reads gateway history and reports counts without sending. `--version` prints build information.

### Google Voice bridge event streams

The native client source now detects an authenticated `smsgate_gv` event-stream
capability on gateway URLs. In push mode it consumes changed API message
records over `/events` directly, reconnects with backoff, and reconciles the
bridge's local journal on connection reset. Phone gateways continue using their
existing HTTPS webhook listener. This feature does not add multi-server support.
The Linux candidate was verified with the bridge; installation is still pending.
The Android client implements the same stream detection and reconnection.

## Accounts and push reconnects

`accounts` contains entries with `id`, `name`, `server`, `username`, and `password`;
`active_account` selects the startup account. Legacy single-server configurations
still work. Press `y` in the recipient list to open Accounts, then Enter to select.
Esc returns to the previous list. Each selected account retains the existing
interface and its own server-scoped history/cache/cursor. Only the selected
account connects; switching cancels and drains its old workers and closes its
push listener. Background updates for inactive accounts are not collected until
that account is opened again. `c` edits the selected account's connection.

In push mode, `push_retry_seconds` defaults to 300. Connection failures retry push
without fallback history polling. Initial loading, explicit refresh, and scheduled
reconciliation (`reconcile_seconds`, default 3600) still read history. The optional
explicit poll mode remains available.

## Local build output

Intermediates, build artifacts, dependencies, caches, and generated test reports belong in `.obj/`. Finished binaries belong in `.bin/`. Both directories are local to this project and ignored by Git. `make clean` removes both directories.

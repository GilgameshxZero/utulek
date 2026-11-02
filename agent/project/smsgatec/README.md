# smsgatec

A shared Linux/Windows terminal client for SMSGate. The application logic, gateway protocol, compose/review state machine, attachment handling, configuration, and direct vCard contact reader are shared Go code. Platform files supply Linux ncurses/SQLite bindings or Windows VT console/system SQLite integration. Windows does not require Go installed to run. The current implementation is not a C++ rewrite.

## Run

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

j/k or arrows navigate. Enter opens a thread, starts compose, finishes compose into review, then sends from review. Escape finishes compose into review or cancels search. Shift+Enter (where mapped) or F4 inserts a newline; bracketed paste never sends. q backs through filters and views before quitting. n opens the new-conversation contact picker. / searches. e edits a reviewed draft; a attaches; s edits the subject in review or saves existing media in a conversation; d removes the last draft attachment. o opens media. E invokes the external editor ($VISUAL/$EDITOR, otherwise vi on Linux or Notepad on Windows). r/F5 refreshes; I/F6 imports history without export webhooks; c/F2 configures the gateway; t cycles appearance; ? shows help.

Terminal/editor shortcuts remain owned by the terminal: the app does not request Kitty keyboard mode or xterm modifyOtherKeys.

## Efficient updates and installation

Push is the default. The client listens on port 8766 of the private network address used to reach the phone, registers its own SMSGate webhooks, and removes them on a clean exit. Its HTTPS certificate is issued by SMSGate's CA and renewed when needed; the certificate, private key, and random callback-path secret stay under the private configuration directory's `push/` subdirectory. The phone must be able to reach that address. If listener setup fails, the app reports it and polls at the configured interval, with a 60-second minimum in push mode and backoff on failures. Set `update_mode` to `poll` to explicitly use polling.

Incoming SMS uses the webhook text directly without a follow-up GET. Outgoing status events fetch only the affected message to preserve authoritative multipart-delivery states. Incoming MMS uses the small arrival webhook, followed by a few bounded metadata checks until download completes. The large `mms:downloaded` webhook is deliberately not registered because it includes base64 media. Incoming attachments are downloaded only when opened or saved, and reused afterward. An MMS arriving without an arrival notification is recovered by reconciliation or manual refresh.

Startup and hourly reconciliation use a gateway-scoped saved cursor with a five-minute overlap; they do not repeatedly fetch all history. The cursor advances only after both endpoints succeed and cache writes commit. Initial cache population, explicit `r`/F5, and history import can read full history. Cached outgoing bodies and media are reused, unchanged records avoid SQLite writes, contacts are indexed in memory with file-change checks, and idle views are not rebuilt or repainted. HTTP connections are reused; sends are never automatically retried.

Install binaries as `~/main.syncthing/monochrome/app/smsgatec` and `smsgatec.exe`. Put the executable shell launcher `smsgatec` and Windows launcher `smsgatec.bat` in `monochrome/bin`, quoting paths and forwarding all arguments. See `workflow/project_binaries-workflow.md` in the agent workspace for the general convention.

## Build and test

The source module is `native/`. Its Makefile builds the Linux binary. A Windows amd64 executable can be cross-compiled from the same directory with GOOS=windows, GOARCH=amd64 and CGO_ENABLED=0 using Go 1.24 or later. Windows regression tests can similarly be compiled with `go test -c` and run on Windows. Ordinary tests use fake HTTP gateways and temporary state; live tests require explicit SMS_LIVE_CONFIG and SMS_LIVE_NUMBER (the international recipient for sends and sender for inbox checks), with a durable journal preventing automatic repeated sends. `SMS_LIVE_JOURNAL` and `SMS_LIVE_CACHE` select isolated live-test state without removing earlier journals. Opt-in callback checks use `SMS_PUSH_CONFIG`; incoming-MMS checks additionally require `SMS_MMS_CAPTION`. Ordinary tests never send texts or change phone settings. Compiled executables remain local and are ignored by Git.

`--demo` is offline and never sends. `--check` reads gateway history and reports counts without sending. `--version` prints build information.

### Google Voice bridge event streams

The native client source now detects an authenticated `smsgate-gv` event-stream
capability on gateway URLs. In push mode it consumes changed API message
records over `/events` directly, reconnects with backoff, and reconciles the
bridge's local journal on connection reset. Phone gateways continue using their
existing HTTPS webhook listener. This feature does not add multi-server support.
The Linux candidate was verified with the bridge; installation is still pending.

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

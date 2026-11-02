# smsgate-gv

Linux Google Voice bridge exposing a subset of the SMSGate local API to the
existing SMSGate terminal client. Current preview supports reading recent
one-to-one and short-code conversation history with native message identity and UTC timestamps.
One-to-one SMS sending and interrupted-send reconciliation are verified with the
existing Android conversation. Full historical synchronization, attachment
downloads, groups, webhooks and unattended systemd deployment remain unfinished.

The executable is built with Go and pure-Go SQLite: no Go, Python or Node runtime
is needed. Installed Chromium or Chrome is required. Build with `make`.

## Authentication and private state

Initialize with `init --number`, then use `auth-check` or `serve`. Valid existing
Chromium cookies are imported without a separate bridge login. `auth-check` shows
stages and has a 90-second overall deadline. The optional graphical `login`
command remains available for manual authentication.

State defaults to `~/.local/state/smsgate-gv`; `SMSGATE_GV_HOME` overrides it.
Configuration contains:

* `number`: the expected Google Voice sender, checked before use.
* `listen`: an IP and port; defaults to `0.0.0.0:8081`.
* `username` and `password`: HTTP Basic authentication credentials.
* `cookie_source`: source profile directory, normally `~/.config/chromium/Default`.
* `auth_retry_seconds`: authentication recovery interval, default 300.
* `poll_seconds`: background history reconciliation; new configurations use 300.
* `max_threads`: maximum supported threads imported from a history response.
* `sync_numbers`: optional list of international peer numbers or short codes; omit
  to include supported recent one-to-one and short-code threads.

The bridge first injects its saved Google session into a disposable headless
browser, verifies the configured number, and persists refreshed cookies. If that
fails, it snapshots the source cookie database consistently, including committed
WAL data. Unrelated cookies are removed before Chromium loads the copy; the
source profile is never launched or modified. If source import fails too, new
sends pause and authentication recovery retries periodically.

`session.json` contains usable Google session cookies, mode 0600 inside a 0700
state directory. Treat it like a password: Google cookies can grant access beyond
Voice. Saved cookies can be injected without the desktop wallet at service
startup. Importing newly refreshed source cookies may still require that wallet
to be unlocked. Google can revoke sessions or require manual sign-in.

Temporary browser profiles are deleted on normal shutdown. SIGKILL or power loss
can leave a private `browser-*` directory; it is not used for recovery.

## History, incremental updates and notifications

The initial page bootstrap identifies the authenticated Google Voice API
transport. A cold inbox snapshot imports up to 100 threads with 50 messages per
thread, restricted by `max_threads` and optional `sync_numbers`. Subsequent
refreshes call `inbox/warmsync` with the saved cursor and fetch only changes.
Cursor files include a decoder version; upgrading from older cursor files triggers
a fresh snapshot to backfill previously skipped short-code messages.
The cursor advances only after message records have been durably committed;
failed cursor writes replay safely. Cursor state is scoped to the own GV number.
Continuation batches are committed individually, and a server cursor reset
triggers a fresh snapshot. Full historical pagination is still unfinished.

The bridge observes Google Voice WebChannel data and app-side sync requests as
coalesced invalidation hints. It then reads authoritative deltas; the hints are
not interpreted as message content. Small observed channel keepalives are ignored.
The periodic reconciliation interval remains a fallback for missed hints or
reconnections. The private web protocol can change.

Ordinary updates do not navigate to Settings, reload the interface or reopen
conversations. Browser asset caching is enabled. Transient endpoint/storage
failures preserve the browser; authentication loss or a dead browser can require
a new bootstrap. Browser startup/background traffic is additional to bridge
request measurements. A measured empty warm response had a 65-byte body and
119 encoded response bytes, compared with the earlier 8,777-byte recent-page
response. These are individual response measurements, not daily carrier totals.

Native IDs are scoped by own GV number. Imports preserve UTC millisecond times,
direction, text, subjects and known client-assigned IDs. Unchanged imports skip
journal writes. Local `/inbox` and `/messages` support pagination and RFC3339
`from` filters. An empty initial journal returns 503 until synchronization is
complete. Numeric short codes retain their original sender identity. Group and
unsupported thread IDs are skipped; historical server-side deletions are not mirrored into the retained local message journal.
MMS text is readable, with explicit open-GV media placeholders; attachment
sending/downloading is not implemented.

Authenticated `/events` emits changed message records as a local SSE stream.
Subscriptions start with a reset indication so reconnecting clients reconcile
against the local journal. Slow consumers disconnect instead of silently losing
updates, and the client reconnects with backoff. This does not trigger a full GV
page reload or transfer media bytes.

The updated native SMSGate client recognizes the bridge's event-stream capability
on configured connections and consumes native message records directly, avoiding a
history pull per notification. The Android phone webhook path is unchanged.
The verified Linux candidate is `.obj/smsgate_candidate`; the installed client has
not been replaced yet. Separate server URLs/client configurations per number
remain necessary until multi-server client support is implemented.

`/health` reports readiness, authentication and the last successful sync. Cached
history remains readable during outages. No systemd service is installed yet.

## Verification

Local `go test ./...` and `go vet ./...` cover decoder validation, account-scoped
identity, imports/restarts, failed commits, timestamp filtering/order, cookie
snapshots, and authentication pause/cancellation behavior.

Opt-in live tests inspect only an explicitly supplied `GV_REMOTE_NUMBER`:
`TestLiveNetworkMessageIdentity` compares native records to the UI across reload
and browser restart; `TestLiveEndpointRefresh` checks endpoint-only refresh and
records encoded response size. Incremental decoder/cursor and event publication
tests cover replay and commit boundaries. Native client tests cover stream
reconnection, exact native identity and cancellation. Authentication tests cover source import and
saved-session startup without desktop/keyring/source-profile access. Private
fixtures, test state and results remain ignored under `.obj/`.

An isolated integration harness also exercised the built bridge with the installed
SMSGate client: API details, local filters/pagination, exact cached text/direction/
timestamps, repeated refresh, process restart and the actual terminal conversation
view. It did not send messages or change the normal phone client configuration.
No systemd service is installed and an actual reboot has not been tested.

Google Voice's private response schema may change; unknown kinds or absent
identity/timestamp fields fail explicitly instead of inventing message identities.

## Sending and recovery

SMS uses the normal Google Voice composer with an independently checked recipient
and exact text. The app generates its normal request and fresh protection fields.
Before dispatch the bridge checks the request text/thread and inserts its durable
48-bit correlation token; protection fields are preserved and never logged. The
bridge accepts a single send request and aborts additional app retries. An unknown
outcome closes the disposable browser and enters reconciliation, without sending
again. Confirmed history must match the native ID, saved correlation token,
account, peer and exact text before the bridge reports `Sent`.

A repeated client ID returns the existing journal record; different content with
that ID is rejected. On restart, queued messages that never reached dispatch fail
without being retried. Dispatched messages become uncertain and can be matched to
history by their saved token. Identical text alone never confirms a send. Recovery
is currently limited to the recent history page; a missing match stays uncertain.
`Sent` means confirmed in Google Voice history, not a carrier delivery/read receipt.

The installed client passed a live multiline/Unicode test with a deliberately
lost HTTP acknowledgment, followed by reconciliation without a duplicate. A
simulated missing confirmation in the durable journal recovered after restart;
the Android gateway independently reported one exact receipt. New-message-form
recipient entry/selection was checked using the same Android number; delivery to
a genuinely new number remains unverified because only two test numbers exist.
Private single-use test journals prevent an accidental repeat of live sends.

The latest isolated live check sent one Android-to-GV SMS with both reconciliation
intervals set to an hour: the candidate terminal showed it in 4.429 seconds, with
no extra client inbox/outbox reads. Old-cursor replay and event-stream reconnection
after a bridge restart produced no duplicates. Results are retained privately in
`.obj/incremental_integration/results.json`. Single-use test journals must not be
reused to send the same test again.

## Local build output

Intermediates, build artifacts, dependencies, caches, and generated test reports belong in `.obj/`. Finished binaries belong in `.bin/`. Both directories are local to this project and ignored by Git. `make clean` removes both directories.

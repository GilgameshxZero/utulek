# SMSGate local API and native terminal client

The canonical project is `project/smsgatec/`; see `project/smsgatec/README.md`. Release executables are `project/smsgatec/smsgatec` (Linux) and `project/smsgatec/smsgatec.exe` (Windows), installed in `~/main.syncthing/monochrome/app/`, with argument-forwarding `bin/smsgatec` and `bin/smsgatec.bat` launchers. It supersedes the retired `smsgate_tui/` client. Linux requires common libraries (ncursesw/tinfo, SQLite, libc/libm); neither executable needs Python, Go, Node, packages, or adjacent application files at runtime. The built-in vCard reader supplies names via the `vcf_dir` config setting; xdg-open optionally opens selected media. Native sources/tests are under project/smsgatec/native/ and are not runtime requirements. The run launcher invokes the same binary.

## Interface and private state

* NeoMutt-like index/contact picker and full-width conversation view; no title/sidebar. Search, hints, and errors share one bottom line. Background refresh is silent.
* j/k or arrows navigate, Enter opens a recipient. n opens New conversation with vCard contact search and manual number entry.
* Conversation Enter opens compose. Compose Enter or Esc finishes into review; Shift+Enter inserts a newline (F4 fallback). Review y or Enter sends; e edits, a attaches a file, s sets/clears the MMS subject, d removes the last selected file. E optionally uses the external editor.
* Conversation o chooses an attachment to open; s chooses one to save. Saving never overwrites an existing file.
* / searches. Esc cancels search and restores the previous view position. Enter confirms a filter; q backs through it and then through the originating list. Selection remains pinned to the same recipient across index reordering. q quits only at the unfiltered root index.
* Updates: default `update_mode: "push"`; HTTPS listener on the private route address, TCP 8766. Incoming SMS requires no follow-up GET. MMS arrival triggers bounded metadata checks; image bytes are fetched only on open/save. Do not subscribe to `mms:downloaded` by default: its webhook includes base64 media. `reconcile_seconds` defaults to 3600 and uses a durable incremental cursor with a five-minute overlap; full history only for initial fill, explicit refresh, or import. Push setup and stream failures retry every 300 seconds by default (`push_retry_seconds`); no fallback history polling. Hourly reconciliation remains. Own webhook registrations are removed on clean exit. Certificate/key and callback-path secret stay in the private config directory under `push/`.
* Settings: ~/.config/smsgatec/config.json. Optional compatible SQLite/message/media cache: ~/.cache/smsgatec/. XDG overrides, --config, cache_dir, and --no-cache are supported. Configuration/cache files are private. No drafts, composition subjects, selected draft paths, or navigation state are persisted.
* AI-agent attribution is optional. Include it only when requested for that message. The application sends exactly the user's text.

## Connection and authorization

Read ignored context/projects/smsgate/smsgate-local_api.md for the saved gateway endpoint and credentials, or the user's private standard configuration. Never copy credentials into tracked files, user-facing commands, diagnostic output, or executable source. The phone's Local Server must be running and reachable directly or through the private network/VPN.

Establish recipient (including country code) and content from the current conversation. Stored credentials are not authorization for future messages. If a contact has multiple numbers and the intended one is unclear, ask before sending. Follow actual approval-review requirements for private content.

## Submit once

Use authenticated HTTP through the native client or an equivalent HTTP client. Local mode uses HTTP Basic Auth and Content-Type: application/json.

* Method: POST
* Path: /message (singular)
* SMS: id plus textMessage.text, phoneNumbers, and withDeliveryReport: true.
* MMS: id plus mmsMessage (optional subject/text, attachments), and phoneNumbers. Do not include textMessage alongside mmsMessage. Each MMS part has contentType, optional name, and base64 data. An MMS requires text or an attachment; subject alone is insufficient.

Use bounded timeouts. HTTP 202 means accepted/queued, not delivered. Retain the message ID immediately. The native client saves its generated ID before posting and never automatically repeats a send. If a response is lost, reconcile by ID first. A timeout does not prove failure. Native attachment resource guards are not carrier delivery limits.

## Delivery

Authenticated GET /message/{id} returns state and recipient states. Pending means queued; Processed means handed to Android; Sent means sent without a delivery confirmation; Delivered means a carrier delivery report, not a read receipt. Failed is terminal. Report only the observed status.

For MMS, Android provides Sent as the final successful state, without Delivered receipts. The native live MMS was independently confirmed received in Google Voice.

## Conversation history and attachments

GET /inbox?includeAttachments=true includes incoming SMS/MMS; paginate with limit=500 and offset and check X-Total-Count. Incoming text is contentPreview and may be abbreviated. MMS can appear first as a notification and later as MMS_DOWNLOADED with attachment metadata.

Refresh historical incoming records with POST /inbox/refresh, ISO since/until, messageTypes: [SMS, MMS], webhookDelivery: Disabled, and triggerWebhooks: false. This sends no texts or export webhooks. Then read the inbox again. Epoch timestamps are unknown dates.

List outgoing gateway records at GET /messages; fetch full content at GET /message/{id}. This does not include all outgoing texts sent by other Android apps.

Download an incoming attachment at authenticated GET /inbox/{messageId}/attachments/{partId}. Treat it as bytes, not JSON. The native o/s picker does this on selection. Outgoing attachments cannot be downloaded from the server; the app retains selected outgoing bytes in the optional private media cache. Saves are explicit and non-overwriting.

## Verified behavior

Native regression tests cover search cancellation/restoration, recipient anchoring, mode transitions, paste/newline safety, SMS/MMS payloads, MMS subject encoding, attachment downloads, safe saves, cache compatibility, and ambiguous-send recovery.

On November 1, 2026, a native-client MMS containing a PNG, text, and subject reached Google Voice. An image/caption reply from Google Voice appeared in the native conversation as downloaded MMS. Saving through s produced a valid 32×32 JPEG with mode 600; Google Voice/carrier handling converted PNG to JPEG.

Earlier cloud SSE captures showed HTTP/2 resets after about 285 seconds, followed by fetching pending messages on reconnect. This supports preferring local API access for this setup, but does not prove a general cloud-service defect or a specific VPN/IPv4 cause.

References: https://docs.sms-gate.app/getting-started/local-server/ ; https://docs.sms-gate.app/features/reading-messages/ ; https://docs.sms-gate.app/features/mms/ ; https://docs.sms-gate.app/features/status-tracking/ . Installed API schema: /docs/swagger.json.

## Installed executable and terminal shortcuts

Build optimized releases with `make all windows` in `project/smsgatec/native`; copy (do not move) `project/smsgatec/smsgatec` and `project/smsgatec/smsgatec.exe` to `~/main.syncthing/monochrome/app/smsgatec` and `smsgate.exe`. Use executable `~/main.syncthing/monochrome/bin/smsgatec` and Windows `smsgate.bat` launchers that forward all arguments. Follow `workflow/project_binaries-workflow.md` for future project installations. Linux retains its common ncurses/SQLite shared-library dependencies; Windows uses system console APIs and winsqlite3.dll and needs no Go installation. Leave enhanced keyboard protocols (Kitty and xterm modifyOtherKeys) disabled on both terminal initialization and resume, so VS Code/terminal shortcuts retain normal handling. Accept existing Shift+Enter mappings, with F4 as the newline fallback.


## Windows and end-to-end verification

Use the actual Windows host and private test numbers recorded in ignored `context/projects/smsgate/sms-windows_port.md`; cross-compilation alone is insufficient. Build `make test-windows`, copy the temporary test executable under `monochrome/app/`, and run it over SSH. Also run the installed `monochrome/bin/smsgatec.bat --version` and `--check`, plus the established PTY demo comparison when terminal behavior changes. Keep prior live-send journals; select a fresh `SMS_LIVE_JOURNAL` and `SMS_LIVE_CACHE` for an explicitly authorized new run.

For browser-side receipt/reply checks, use `workflow/google_voice-workflow.md` and the saved headless Chromium/keyring method. Wait for the exact recipient conversation heading to load. A composer can appear before that conversation finishes loading. Receipt text may occur both in the sidebar preview and the conversation: scope to the conversation or select its actual message bubble. After sending, wait and reload to verify persistence before closing the temporary browser; an optimistic bubble alone is insufficient. Confirm arrivals through SMSGate as well. Use SMSGate's supported whole-second RFC3339 timestamps (or exactly three fractional digits) in date filters; arbitrary nanosecond precision is rejected. Invalid date filters on the logs endpoint can silently return all logs, so avoid them.

Do not use inbox reimport as a callback probe: already-imported messages suppress duplicate webhooks. Do not toggle ping as a probe: changing its interval through the API requires an app restart. Use a real authorized Google Voice -> Android incoming event, with both listeners ready, to validate callbacks. Keep ping disabled unless the user separately wants it.

Accounts: y in the list opens the account picker; Enter selects, Esc cancels. Only the selected account connects. Configuration uses accounts plus active_account; legacy flat configs remain supported.

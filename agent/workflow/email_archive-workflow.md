# Email archive workflow

## Location and format

* Archive path: `../../monochrome/archive.syncthing/module/email/archive.maildir/`.
* This is a Maildir archive. Messages are individual files, usually under `cur/`; check `new/` too when relevant.
* Filenames are not descriptive of message topics. They begin with a Unix timestamp, followed by delivery metadata and Maildir flags. Sort by the leading numeric timestamp to find recent messages.
* Use the message `Date` header for the email's stated date and time; the filename timestamp is useful for ordering.
* A filename cutoff filters delivery/archive timestamps, not necessarily message dates; recently delivered files can contain much older messages. State the actual search scope when reporting results.
* This environment has `python3`; do not assume `python` exists. For header-only reads, use `email.parser.BytesHeaderParser(policy=email.policy.default).parse(file)` or `.parsebytes(raw)`. Do not pass `headersonly` to `email.message_from_bytes`, which rejects that argument.

## Efficient search

For an urgent, narrow lookup, bound the delivery-filename date range first and read raw lines only until the blank line ending the headers, then use `BytesHeaderParser.parsebytes`. This avoids MIME/body parsing and large attachment reads. An October 28 lookup parsed 449 recent headers in about 0.2 seconds; only the matching invitation/notifications then needed body extraction. Prefer this to a full raw archive search when identifying a meeting organizer or recruiter firm. Preserve the scope and matching paths in the task index.

1. Recursively enumerate files, parse the leading filename timestamp, and sort newest first.
2. Parse headers with Python's `email` package using `email.message_from_bytes(..., policy=email.policy.default)`. Review `From`, `To`, `Date`, and `Subject` for recent messages before reading bodies.
3. Narrow by sender/recipient, subject, or a distinctive phrase. Broad `rg` searches for common words (company names, “LinkedIn,” etc.) can return many irrelevant matches.
4. Extract `text/plain` MIME parts from likely messages and inspect nearby replies or LinkedIn notification emails. Newer replies may quote the original message.
5. Retain only relevant matches, with paths and message IDs, in a queryable task index in `context/`; avoid saving unrelated family correspondence just because a sender matched.

## Calendar invitations and scheduling evidence

* Inspect `text/calendar` MIME parts, not just the plain-text body. Some Lark invitations have an empty plain-text part and put scheduling details in the calendar part or HTML.
* Check `METHOD`, `UID`, `DTSTART`, `DTEND`, `STATUS`, `SEQUENCE`, organizer, and attendee `PARTSTAT` values. Match replies to the invitation by `UID`. A `METHOD:REPLY` message with the user's `PARTSTAT=ACCEPTED` is evidence of an RSVP, even when the body adds no personal message.
* A trailing `Z` on calendar times means UTC. Convert the actual event date using the relevant timezone, accounting for daylight saving. Do not infer the time from the message delivery date.
* Unfold calendar lines (remove CRLF followed by a space or tab) before extracting wrapped fields and URLs. The description can contain an HTML meeting link plus cancellation and rescheduling links; retain the meeting link without opening action links.
* `STATUS:CONFIRMED` and an organizer attendee marked accepted support treating a meeting as scheduled. Scheduling software can generate these fields; they do not prove a personal reconfirmation or guarantee attendance.
* Check relevant later messages for updates/cancellations before calling an invitation current. Distinguish archive evidence from live LinkedIn, WeChat, or calendar state, which may not have been inspected.

## Large duplicate scans

* Keep each message's Maildir path alongside its content fingerprint and group membership. Subjects and counts alone are not enough to support follow-up inspection.
* Retain the group index in queryable state after the scan. If the analysis process may exit, use a private temporary database outside the Maildir and verify that group members can be queried before reporting results. Do not rely on a process-local in-memory table or an stdin loop unless its lifetime has been confirmed.
* Exclude Google Chat messages from duplicate groups when indicated by Gmail's `Chat` label. Keep dates out of the content fingerprint when comparing email copies, but report when missing dates or parse errors exclude messages.
* Never modify Maildir messages during a duplicate scan unless the user explicitly asks for that change.

## Example: list recent headers

```python
from pathlib import Path
import email
import email.policy
import re

root = Path("../../monochrome/archive.syncthing/module/email/archive.maildir")
messages = []
for path in root.rglob("*"):
    if not path.is_file():
        continue
    match = re.match(r"(\d+)", path.name)
    if match:
        messages.append((int(match.group(1)), path))

for timestamp, path in sorted(messages, reverse=True)[:100]:
    msg = email.message_from_bytes(path.read_bytes(), policy=email.policy.default)
    print(timestamp, msg.get("From"), msg.get("To"), msg.get("Subject"), path)
```

## Example: extract a selected message's plain text

```python
for part in msg.walk():
    if part.get_content_type() == "text/plain":
        print(part.get_content())
```

# Email archive workflow

## Location and format

* Archive path: `../../monochrome/archive.syncthing/module/email/archive.maildir/`.
* This is a Maildir archive. Messages are individual files, usually under `cur/`; check `new/` too when relevant.
* Filenames are not descriptive of message topics. They begin with a Unix timestamp, followed by delivery metadata and Maildir flags. Sort by the leading numeric timestamp to find recent messages.
* Use the message `Date` header for the email's stated date and time; the filename timestamp is useful for ordering.

## Efficient search

1. Recursively enumerate files, parse the leading filename timestamp, and sort newest first.
2. Parse headers with Python's `email` package using `email.message_from_bytes(..., policy=email.policy.default)`. Review `From`, `To`, `Date`, and `Subject` for recent messages before reading bodies.
3. Narrow by sender/recipient, subject, or a distinctive phrase. Broad `rg` searches for common words (company names, “LinkedIn,” etc.) can return many irrelevant matches.
4. Extract `text/plain` MIME parts from likely messages and inspect nearby replies or LinkedIn notification emails. Newer replies may quote the original message.

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

# Conversation monitoring

Follow discord-workflow.md, playwright-workflow.md, smsgate-workflow.md. Establish authorized recipients, UTC deadline, privacy boundaries and disclosure. Announce start/end and deadline changes. Keep private identifiers/state in ignored context.

Poll Discord DOM every 1–2 seconds, return new messages promptly, retain seen IDs. Select main message-content by message ID; generic descendants may read quoted text. Cache author references for grouped messages. Reply naturally without answering every line.

For SMS, verify contacts with khard. Supply selection input when show is ambiguous: EOF can cause a busy loop. Refresh inbox explicitly with messageTypes [SMS,MMS]; default refresh only imports SMS. MMS_DOWNLOADED is accepted as query type but rejected for refresh. Refresh is asynchronous; fetch again afterward. Query includeAttachments=true with a from date; normalize sender numbers and filter authorized recipients before output. Download pictures via /inbox/{id}/attachments/{partId} and view before replying about them. Deduplicate by ID. Submit outgoing messages once, retain IDs and separately check delivery.

Maintain monitoring during lookups. Use concise output. Announce interruptions if possible; never claim a watchdog is installed without success. At deadline send disclosed end notices, close separate browser and delete temporary profiles/scripts/images and browser artifacts.

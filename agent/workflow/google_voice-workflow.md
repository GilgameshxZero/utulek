# Send SMS through Google Voice

Use when the user asks to send or read messages through Google Voice. This was verified using the connected Playwright browser on November 1, 2026.

## Access and scope

* Follow `workflow/playwright-workflow.md`, including cleanup of automatically generated browser snapshots and logs.
* Read ignored `context/accounts/google_voice/google_voice-account.md` for the last verified account and number. Do not store cookies or session tokens in this workflow or task notes.
* Use the connected Playwright browser's existing signed-in session at `https://voice.google.com/`. No cookie extraction or separate login was needed in the verified session. If signed out, report that and ask the user to sign in as needed.
* Establish the authorized recipient and message content from the conversation. Stored account access is not standing authorization to send. If a contact has multiple numbers and the intended one is unclear, ask before sending.
* AI-agent attribution/disclaimers are optional. Include one only when requested for that message; do not carry a prior message's preface forward by default.

## Verify the sender

Navigate to `https://voice.google.com/u/0/settings` and locate **Google Voice number**. Confirm it matches the user's requested sender. Account index `0` is not guaranteed to stay associated with the same account; inspect rather than assume.

Use `browser_find` with a targeted phrase or number to avoid returning unrelated inbox messages. Discover tool argument schemas from tool metadata if needed: this session's `browser_click` and `browser_type` used `target`, not `ref`.

## Compose and send once

1. Open Messages, then an existing conversation with the authorized recipient or **Send new message**. Existing thread links may use `/u/0/messages?itemId=t.%2B<country-code-and-number>`; prefer a link observed in the browser rather than assuming an identifier.
2. Verify the conversation heading/recipient number before filling the **Type a message** textbox.
3. Fill the exact authorized text. Locate the enabled **Send message** button and click once.
4. Confirm the text appears as an outgoing message in the conversation, and that the composer cleared. A click result alone does not prove it was sent. Check for error or retry indicators.

### Browser click timeout

In the verified session, normal Playwright clicks timed out while waiting for a button to be visible, enabled, and stable, even though the page was loaded and text entry worked.

After a timeout, inspect the conversation and composer first to avoid a duplicate. If the message is still in the composer, the correct recipient is verified, and there is no indication it was sent, a DOM click through `browser_evaluate` worked:

```javascript
() => {
  const button = document.querySelector('button[aria-label="Send message"]');
  if (!button || button.disabled) return { clicked: false };
  button.click();
  return { clicked: true };
}
```

This is a fallback for a UI automation timeout, not a way around a permission or approval rejection. Reinspect the conversation immediately afterward. Do not repeat a send merely because confirmation is delayed.

## Verify and report

* Google Voice displaying an outgoing message supports reporting **sent**, not delivery or reading by the recipient.
* If the recipient is the user's own Android phone and verifying arrival is within the authorized task, use `workflow/smsgate-workflow.md` to check recent incoming records for the exact Google Voice sender, text, and timestamp. This confirmed receipt in the November 1 test.
* Report the actual sender, recipient, and result. Use UTC for times.
* Remove only browser snapshots and logs created during this task unless the user requested retention; preserve unrelated pre-existing artifacts. Check `.playwright-mcp/` itself afterward.

## Observed result

On November 1, 2026, the authorized test appeared as an outgoing message in Google Voice and was independently found in the recipient phone's SMSGate inbox at 04:28:43 UTC. Browser audio/call warnings did not prevent sending SMS.

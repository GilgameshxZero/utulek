# Discord workflow

Follow [the Playwright workflow](playwright-workflow.md). Use a separate headless browser by default. The user authorizes reuse of existing cookies and authentication session data without additional approval. Attaching to the user's open browser still requires their explicit request.

## Headless signed-in access

This procedure worked with the local Chromium profile and npm-cached Playwright:

* Locate `playwright` under `~/.npm/_npx/*/node_modules/` with `rg --files`. The connected Playwright MCP tool may be configured with a CDP endpoint pointing at the user's open browser; use the Node Playwright library directly for headless operation instead.
* Create a private temporary directory with `mktemp -d` under `/tmp`. Create a `profile/Default` directory inside it.
* Copy these items from `~/.config/chromium` into the same relative locations in the temporary profile: `Local State`, `Default/Preferences`, `Default/Cookies`, `Default/Cookies-journal` if present, and `Default/Local Storage`. Discord authentication needs local storage; cookies alone may not suffice. Exclude `LOCK`, `LOG`, and `LOG.old` files from the local-storage copy. Do not copy browser history, saved passwords, or unrelated profile directories. The shared local-storage database can contain other sites' data; never inspect or print it, and delete the entire temporary profile afterward.
* Launch `chromium.launchPersistentContext(temporaryProfilePath, { executablePath: '/usr/bin/chromium', headless: true, args: ['--no-sandbox', '--disable-dev-shm-usage'], viewport: { width: 1280, height: 900 } })`. Use the normal sandbox where supported; the listed launch arguments worked in this environment. Network access and browser execution may require a tool sandbox escalation.
* Open `https://discord.com/channels/@me`, let the UI load, and confirm that it is signed in. Avoid printing credentials, cookies, or raw session storage.
* For an interactive Node controller reading commands from stdin, launch with `exec_command` and `tty: true`, then use `write_stdin`. Without a TTY, stdin may close and terminate the browser before follow-up actions.

## Find, send, and verify

* Find the requested conversation using its visible link text and inspect its destination. Confirm the selected conversation, including whether it is an individual or group DM; resolve material recipient ambiguity before sending.
* Honor any request to approve the exact wording before sending. Once approved, send that wording without asking again unless something material changes.
* In this Discord UI, `page.getByRole('textbox')` selects the message `TEXTAREA`. Use `fill(message)` and `inputValue()` to check the draft; `innerText()` can be empty for a textarea. Verify its placeholder and the current conversation URL before sending.
* Send with `press('Enter')`. Verify that the composer is empty and the exact message appears in a matching `[id^="chat-messages-"]` element. If the result is uncertain, inspect the conversation before retrying to avoid duplicate sends.
* Close the headless context and delete the temporary script and profile, including all copied session data. Do not retain private messages, recipient identifiers, or session material in this tracked workflow. Keep routine output minimal and avoid log artifacts.

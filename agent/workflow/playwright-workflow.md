# Playwright workflow

Do not generate Playwright logs or log artifacts by default. Avoid enabling debug logging or saving browser, console, network, trace, video, or test-run output to log files. Keep routine Playwright output minimal. Generate logs or related artifacts only when the user explicitly asks for them or they are necessary to diagnose a specific issue.

The connected Playwright browser tool automatically writes page snapshots under `.playwright-mcp/` during navigation. These are snapshots, not logs, and the tool does not expose a way to disable them. If that tool is used, remove its generated snapshots after the task so they are not left in the workspace.

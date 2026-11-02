# Session handoff workflow

Use when the user asks to move a conversation or task into another session, asks to save progress, or says the work is done (for example, "that's all", "we're done", or "done for now"). The user explicitly wants a resumable save when ending work; do this before the final reply without asking for another confirmation.

## Save when the user is done

1. Stop starting new task work. Finish only the necessary wrap-up and persistence already authorized.
2. Ensure completed file changes are saved on disk. Do not create Git commits; uncommitted work is intentional in this workspace. Preserve unrelated changes.
3. Update the existing relevant private task note, or create a concise handoff if none provides a clear resumption point. Save the final state, adopted preferences, important artifacts and paths, verification and its limits, outstanding work, and the next useful action. A finished task can explicitly have no outstanding work.
4. Keep all task/project-specific information in ignored `context/`, using exactly one context/handoff file per task or project. Update that same file. Reserve `workflow/` for reusable procedures shared across different tasks/projects, without credentials or private account details. Retain queryable results needed for follow-up rather than rerunning expensive work. Do not duplicate transcripts or routine tool output.
5. Check that private notes and generated artifacts are ignored, and read back the handoff. Do not rerun tests or scans just for a save.
6. Briefly confirm the save with a clickable link to the handoff. Link a changed workflow when applicable. Do not claim a commit, upload, or backup occurred unless it actually did.


Keep handoffs concise and limited to what the next session needs. Do not create routine task notes by default; write a note only when requested or necessary to preserve important context. Link existing artifacts instead of duplicating transcripts, correspondence, or tool output.

## Create or update the task note

1. Check `context/` for the single existing context/handoff file covering the task or project and update it. Do not create a separate handoff alongside an existing context file. If none exists, create `<task_name>-session_handoff.md` in the appropriate topic directory under `context/` (see `context/README.md`). Use lowercase snake_case within phrases and hyphens between phrases.
2. Summarize the relevant conversation and completed work. Read only the files needed to confirm paths or current state; do not repeat expensive scans just to create a handoff.
3. Put the latest state and next useful action near the top. Include a timestamp with timezone when timing matters, and distinguish historical state from facts that remain current.
4. Include enough context that the receiving session can continue without access to the previous chat:
   * User objective, scope, preferences, boundaries, and important corrections.
   * Confirmed actions/results, outstanding work, and unresolved questions.
   * Decisions actually adopted, separately from assistant suggestions or interpretations.
   * Relevant absolute file paths, artifact locations, queryable scan results, and source links.
   * Checks already run, limitations, and required checks still outstanding, if relevant.
   * Existing authorization and any external actions that have or have not occurred, if relevant.
5. For support or other sensitive conversations, retain only details useful for continuity. Preserve the user's wording and boundaries; do not convert reported feelings into diagnoses, imply undisclosed risks, or promise privacy or anonymity.
   * User preference for personal-life handoffs: foreground personal history, emotional context, work/school/family concerns, support preferences, and the way they want to continue. Keep incidental shopping transactions and individual appointment logistics in separate task notes, with only brief pointers when useful. The user may return for scheduling help, but an old schedule should not dominate the personal handoff.
6. Do not include hidden reasoning, credentials, unrelated personal material, or wholesale tool logs. Never invent progress or treat an unconfirmed plan as completed.

## Keep procedures separate from task history

Store reusable methods in `workflow/` and task-specific state in `context/`. A handoff note can link to a relevant workflow, but should not duplicate its full instructions. Update a reusable workflow only when there is a useful procedural change or the user requests it.

## Deliver the handoff

1. Read back the written note to check the latest status, paths, unsupported assumptions, and any missing user corrections. For a documentation-only handoff, this review is sufficient; do not run unrelated tests.
2. Give the user a clickable link to the absolute path and a short instruction they can give the receiving session, such as: "Read /absolute/path/to/context/task-session_handoff.md and continue from the latest state."
3. If giving shell commands to run, copy those exact commands to the desktop clipboard according to `workflow/clip-workflow.md`. A ready-to-paste handoff instruction may also be copied for convenience; report clipboard success only after the tool succeeds.
4. If a reusable procedure was also requested, link it in the final response. Keep the response brief; do not reproduce the handoff note in chat.

# Deviation Analysis

Given a Cursor implementation plan, compare it against the local changes and judge whether the plan was delivered exactly as written. Surface every deviation — missing work, partial work, or drift from the agreed approach. If everything in the plan was delivered and nothing extra crept in, tell me it's good to commit and push.

This is a read-only audit. Do not edit code, fix gaps, run `make` targets, or make commits during this command. Your only output is the verdict and the deviation list.

## Step 1: Load the plan

- If I named a plan file, read it in full. Otherwise look under `.cursor/plans/` and, if more than one exists, ask me which plan this analysis is for before going further.
- Extract the plan's commitments into a concrete checklist:
  - Every item in the `Todos` checklist.
  - Every file path the `Parts` section says to create or change, and what each one was supposed to contain (handler method, service method, repo query, mapper, DTO, migration SQL, router registration, wiring in `cmd/main.go`, etc.).
  - Every recorded `Decision` — these encode the agreed approach, so a change that contradicts a decision is drift, not just a gap.
  - Every check named in the `Validation` section (specific tests, `make lint`, `make build`, manual steps).
- Treat the plan as the source of truth for intent, but do not trust its descriptions of the current codebase — verify everything against the actual diff and files in Step 3.

## Step 2: Capture the local changes

Run these in parallel via the `Shell` tool to see the full picture of what actually changed:

- `git status` — untracked files and staged/unstaged changes.
- `git diff` and `git diff --staged` — the actual line-level changes.
- `git diff --stat HEAD` — the set of touched files at a glance.

Read the full contents of new or heavily changed files where the diff alone is ambiguous. For generated code (entities, mocks), confirm it was regenerated rather than hand-edited.

## Step 3: Match changes against the plan

Go through the plan checklist from Step 1 item by item and classify each against the diff:

- **Delivered as planned** — the change exists, lives where the plan said, and does what the plan described (respecting the relevant `Decision`).
- **Missing** — the plan called for it and there is no corresponding change.
- **Partial** — started but incomplete (e.g. service method added but no error mapping in the handler, repo query added but no router registration, migration written but entity not regenerated, new interface method but `make mock` not run so the mock is stale).
- **Drifted** — delivered, but differently from the plan: a different file path, a different schema/column name, a different error semantic, a contradicted decision, or a different approach than the one we agreed on.

Then scan the diff for anything that is **not in the plan at all** — extra files, unrelated refactors, debug code, stray prints/logs, commented-out blocks, TODOs, secrets or `.env` values. Out-of-scope additions are deviations too.

Be specific and evidence-based. For every deviation cite the exact file path (and line range where it sharpens the point) plus the plan section it relates to. Verify each claim against the real diff — never infer delivery from the plan's own wording.

Also sanity-check the repo's own bar (from `AGENTS.md`): respect the layering (handlers → services → repositories), no `zap.Any` on structs that may carry PII, error mappings updated when new error types were added, and `make lint` / `make build` expectations from the plan's `Validation` section.

## Step 4: Report the verdict

Lead with one of two clear verdicts:

### If there are deviations

State plainly that the plan is **not** fully delivered and do **not** tell me to commit. Then list the deviations grouped by type, each as a single actionable line:

- **Missing** — what the plan wanted, where it should live.
- **Partial** — what exists, what's still needed to finish it.
- **Drifted** — what the plan said vs. what the code does, and which `Decision` (if any) it contradicts.
- **Out of scope** — unplanned changes in the diff that should be removed or split out.

Close with the shortest path to alignment: the specific edits needed to finish the plan as written, or — if a drift is actually an improvement — a one-line note that I may want to amend the plan instead. Don't make any of these changes yourself; just name them.

### If everything matches

State plainly that the plan was **fully delivered as planned** with no missing parts, no drift, and no out-of-scope changes. Briefly confirm the high-value checks that back this up (every todo accounted for, every planned file present, decisions honoured, validation steps satisfied per the plan). Then tell me it's **good to commit and push**, and — if I ask — offer to draft the commit message from the plan summary. Do not commit or push yourself unless I explicitly ask.

## Style rules

- Be specific. "Service layer incomplete" is useless; "§Parts said `internal/safety/service.go` gets a `BlockUser` method that maps `ErrAlreadyBlocked`, but the handler at `internal/api/safety/handler.go:NN` never maps that error to a status code" is useful.
- Distinguish "missing" from "drifted" from "out of scope" — they need different fixes.
- Don't manufacture deviations to look thorough. If an item is delivered correctly, mark it delivered and move on.
- Never green-light a commit while any Missing, Partial, or unexplained Drifted item remains.

# Deviation Analysis

Given a Cursor implementation plan, compare it against the local changes, surface every deviation, **fix all fixable gaps automatically**, re-verify, and report whether the work is ready to commit. Do not wait for a follow-up prompt to start fixing — remediation is part of this command.

**Do not edit the plan file itself** unless a drift is clearly an improvement and you record why in the final report. Do not commit or push unless I explicitly ask in the same turn.

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

## Step 4: Fix deviations automatically

If Step 3 found any **Missing**, **Partial**, **Drifted**, or **Out of scope** items, fix them now — do not stop at a report and wait for me to say "fix them".

Work through deviations in this order:

1. **Out of scope** — revert or remove unplanned changes (debug code, stray refactors, accidental files) unless they are clearly harmless test/doc additions that directly support the plan's Validation section.
2. **Missing** — implement what the plan specified, in the file and shape the plan named.
3. **Partial** — finish the incomplete work (wire the handler, regenerate mocks, add the missing test, run the migration, etc.).
4. **Drifted** — align code with the plan and recorded **Decisions**. If the drift is objectively better than the plan *and* changing it would be a product decision, leave the code as-is, flag it in the final report, and note that the plan may need updating — do not silently adopt a different approach.

While fixing:

- Prefer the smallest correct diff. Match existing conventions in the touched files.
- Run the plan's **Validation** commands as you go (`go build`, `go vet`, targeted tests, `npx tsc --noEmit`, migrations against dev DB when applicable).
- If a validation step requires infrastructure you can start locally (e.g. Postgres via docker compose), start it and run the check — do not mark validation Partial and stop if you can resolve it yourself.
- For validation gaps that are purely manual UI smoke tests, close them with the strongest automated substitute available (API/integration tests, repository queries) and note any remaining manual checks in the final report.
- If you are genuinely blocked (ambiguous product choice, missing secret/credential, failing external service you cannot reach), stop fixing, explain the blocker, and ask me — do not guess.

After each fix pass, re-run Step 2 and Step 3 against the updated diff. **Loop** until either:
- no deviations remain and validation passes, or
- you hit a blocker that requires my input.

## Step 5: Report the verdict

Lead with one of two clear verdicts:

### If deviations remain (including blockers)

State plainly that the plan is **not** fully delivered and do **not** tell me to commit. Summarise:
- what you **already fixed** in this pass (bullet list, with file paths),
- what **still remains** grouped by type (Missing / Partial / Drifted / Out of scope / Blocked),
- the specific next action for anything you could not auto-fix.

### If everything matches after fixes

State plainly that the plan was **fully delivered as planned** (note "after auto-fix" if you changed anything). Briefly confirm:
- every todo accounted for,
- every planned file present,
- decisions honoured,
- validation steps run with results.

Then tell me it's **good to commit and push**, and offer to draft a commit message from the plan summary. Do not commit or push yourself unless I explicitly ask.

## Style rules

- Be specific. "Service layer incomplete" is useless; "§Parts said `internal/safety/service.go` gets a `BlockUser` method that maps `ErrAlreadyBlocked`, but the handler at `internal/api/safety/handler.go:NN` never maps that error to a status code" is useful.
- Distinguish "missing" from "drifted" from "out of scope" — they need different fixes.
- Don't manufacture deviations to look thorough. If an item is delivered correctly, mark it delivered and move on.
- Never green-light a commit while any Missing, Partial, unexplained Drifted, or failed Validation item remains.
- When you auto-fix, show both the **before** finding and **what you did** — keep the report concise but auditable.

# Review Plan

Given a plan file (usually under `.cursor/plans/`), review it against the codebase, surface every ambiguous or undecided section, ask me to resolve the real ones, then patch the plan in place. Stay in plan mode until I explicitly switch.

## Step 1: Load context

- Read the plan file in full.
- Re-read [.cursor/commands/create-implementation-plan.md](.cursor/commands/create-implementation-plan.md) to remember the acceptance criteria the plan must meet.
- Cross-check the plan against the codebase. For every concrete claim the plan makes — file path, line number, struct name, interface method, error variable, mock pattern, route registration, mocked dependency — verify it. Use parallel `Grep` / `Read` / `Glob` calls. Do not trust the plan's own descriptions of the codebase.
- Pay special attention to claims about:
  - Exact file paths and line numbers (off-by-many is common).
  - Whether a "dedicated handler" actually exists for a write path, or whether it's bundled into a larger handler.
  - Whether interfaces have generated mocks (look for `go:generate mockgen` directives + sibling `_mock.go` files) — adding interface methods requires `make mock`.
  - Whether validators run unconditionally or only when a payload field is non-nil. Existing `if x != nil { … }` gates are easy to miss and cause production regressions.
  - Whether constants the plan bumps have other callers that change behaviour.

## Step 2: Categorise findings

Sort everything you find into three buckets:

### Bucket A — Real ambiguities (need my decision)

A real ambiguity is anything where:
- Multiple valid choices materially change implementation, schema, API shape, or UX.
- The plan silently introduces a breaking change for existing users / data.
- The plan picks a value (a column name, a category string, a magic constant) without my input and the choice is visible in the API or DB.
- The plan's scope is unclear: would this rule fire on unrelated edits? Would it block a read path?

Don't manufacture ambiguities — if a single answer is clearly correct given the rest of the plan, just fix it (Bucket B). Be conservative: cap real ambiguities at 3-4. Use the `AskQuestion` tool with concrete options (label each option with the tradeoff, not a vague description). After my answers, if anything I said raises a new ambiguity, ask another short round.

### Bucket B — Specification gaps (fix without asking)

Things that are obviously missing or wrong but where the right fix is unambiguous:
- Imprecise file paths → pin to exact paths and line numbers.
- Missing tooling steps that the codebase requires (`make mock`, `make entity`, `make lint`, etc.).
- Missing test cases that follow obviously from the rule being added (e.g., the negative-path / no-op-path that proves a gate works).
- Frontend / cross-team coordination notes when the plan changes an API contract.
- Wiring or mock regeneration that the plan omitted.

Note these in your critique to me, but don't ask permission — patch them in.

### Bucket C — Strengths

Briefly mention 3-5 things the plan got right (correct schema instinct, correct pattern reference, correct dependency direction, no unnecessary wiring, etc.). This calibrates trust.

## Step 3: Present the critique

Before patching, write a single message with:

- **Strengths** — short bulleted list (Bucket C).
- **Issues I found** — split into:
  - **Real ambiguities — need your decision** (Bucket A) numbered list with your concrete recommendation if you have one.
  - **Specification gaps I'll fix without asking** (Bucket B) numbered list. Be explicit so I can object if I disagree.
  - **Minor things** — single-line items (typos, slightly stricter constraints than needed, etc.).
- An `AskQuestion` call covering all Bucket A items in one batch (single-select per question, options labelled with tradeoffs).

Do not start patching the plan file until I answer the questions. While I'm answering, do nothing.

## Step 4: Patch the plan

Once the answers are in:

- Use `StrReplace` against the plan file directly. Do NOT call `CreatePlan` again — the plan is a markdown file and you edit it in place. Plan mode permits markdown edits to plan files.
- Update the YAML frontmatter `todos:` list when adding new actionable tasks. Each new todo gets a unique `id`, a one-line `content`, and `status: pending`.
- Update the `Decisions` section to record every Bucket A choice with the rationale I gave.
- Update the relevant `Parts` subsections to absorb every Bucket B fix. Pin file paths and line numbers; show code structures where they clarify intent.
- Update the `Validation` section if any new test cases were added (typically one new case per Bucket A or B item that affects runtime behaviour).
- Keep the plan proportional — don't bloat sections with restatements of what I just told you.

## Step 5: Self-check and report

Run the plan against the acceptance criteria from [create-implementation-plan.md](.cursor/commands/create-implementation-plan.md):

- All ambiguities resolved and recorded as decisions.
- Every part lists exact file paths and what goes in them.
- Key code structures shown where they clarify intent.
- Validation plan is concrete and risk-based.
- Todos are actionable and ordered (bottom-up — repo → service → handler → wiring → mocks → tests).
- An engineer unfamiliar with this area could implement it without further questions.

Report which criteria pass and which don't, then ask whether to iterate again or treat the plan as approved. If approved, suggest switching to agent mode to begin executing.

## Style rules

- Be specific. "Inaccurate file path" is useless; "§7 says `internal/api/profile/handler.go` has an `UpsertVoicePrompts` method, but voice-prompt writes flow through the general profile-update handler at line N" is useful.
- Distinguish "this is wrong" from "this is missing". Both warrant a fix; only the former warrants an apology in the critique.
- Never invent ambiguities to look thorough. If a section is fine, leave it alone.
- Do not run `make` targets, edit non-plan files, or make commits during this command. Plan mode rules apply throughout.

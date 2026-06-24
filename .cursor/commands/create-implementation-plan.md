# Create Implementation Plan

Given a linear ticket link or task description, produce a concrete, actionable implementation plan for this codebase. Switch to plan mode — no edits until I confirm.

## Step 1: Gather Context

Read the task carefully, then build a picture of the relevant code before planning. Use parallel explore subagents when the surface area is broad.

- Identify which domain(s) the change touches (`internal/{domain}/`) and which API layer(s) (`internal/api/{domain}/`).
- Read the existing handler, service, repository, domain models, and mappers in the affected domain.
- Find a similar feature elsewhere in the repo to use as a reference pattern (e.g. how `interaction`, `profile`, `safety`, or `conversation` does it).
- Check `AGENTS.md` and `internal/README.MD` for the conventions you must follow.
- Identify integration points: router registration in `internal/http/router/router.go`, wiring in `cmd/main.go`, migrations in `migrations/`, generated entities in `internal/entity/`.

Spend real time here. A thorough read upfront prevents bad questions later.

## Step 2: Resolve Ambiguities

Before drafting the plan, surface every decision that would materially change it. Use the `AskQuestion` tool to ask 1–4 targeted questions at a time.

Ask about:

- **Design alternatives** — when there are multiple valid approaches, present the tradeoffs and ask me to pick.
- **Scope boundaries** — what is in vs. out of this change.
- **Schema / API shape** — table columns, indexes, request/response fields, status codes, error semantics.
- **Validation strategy** — what risks need test coverage, what can rely on existing patterns.

Rules:

- All clarifying questions must be asked and answered **before** the first draft of the plan. No questions appear inside or after the first plan.
- Only leave code-level details (exact names, error messages, log lines) to the build phase.
- If my answers raise new ambiguities, ask another round until resolved.
- When I pick an option, record it as an explicit decision in the plan.

## Step 3: Draft the Plan

Write the plan as a markdown document. Keep it proportional to the task — a 2-file change does not need 6 sections.

Structure:

### Summary

Two or three sentences: what this plan does, which domain it touches, and why it's being done now. If I gave any framing during Q&A (priorities, concerns, areas I'm unfamiliar with), capture it here.

### Decisions

List every ambiguity from Step 2 and the chosen option, with my rationale where I gave it. Nothing in the plan is "optional" — every path is decided.

### Parts

Break the work into logical parts following this repo's layering. For each part, give:

- The exact file paths to create or change.
- What goes in each file (handler method, service method, repo query, mapper, DTO, migration SQL, etc.).
- Key code structures where they clarify intent — schemas, interfaces, function signatures, request/response DTO shapes, error variables. Show the shape, not the full implementation.

Default ordering for a new feature in this repo (skip parts that don't apply):

1. Migration (`migrations/`) + regenerated entity (`internal/entity/`).
2. Domain model (`internal/{domain}/domain/domain.go`).
3. Repository (`internal/{domain}/storage/repository.go`).
4. Mappers (`internal/{domain}/mapper/`).
5. Service + service errors (`internal/{domain}/service.go`).
6. DTOs + DTO mappers (`internal/api/{domain}/dto/` and `.../dto/mapper/`).
7. Handler + error mapping (`internal/api/{domain}/handler.go`).
8. Router registration (`internal/http/router/router.go`) and wiring (`cmd/main.go`).

### Validation

For each meaningful risk, name the cheapest check that proves it works. Be concrete — not "test thoroughly". Cover:

- Unit tests for service-layer business logic and error paths.
- Repository tests where SQL is non-trivial.
- Handler tests for status code mapping and validation.
- `make lint` and `make build` must pass.
- Manual verification steps where automated coverage is impractical (e.g. hitting the endpoint locally with a sample payload).

### Todos

A flat checklist of actionable tasks, each mappable to a focused commit. Order them so the repo compiles after each step where possible (bottom-up: repo → service → handler → wiring).

## Step 4: Iterate

The plan is a living document until I approve it.

1. I'll annotate the plan inline with corrections, rejections, or extra constraints.
2. When I signal I've added notes, re-read the plan, address every annotation, and update it.
3. Before asking if another round is needed, check the plan against:
   - All Step 2 ambiguities resolved and recorded as decisions.
   - Every part lists exact file paths and what goes in them.
   - Key code structures shown where they clarify intent.
   - Validation plan is concrete and risk-based.
   - Todos are actionable and ordered.
   - An engineer unfamiliar with this area could implement it without further questions.
4. Report which criteria pass and which don't, then ask whether to iterate again or treat the plan as approved.

Do not start implementing until I explicitly approve the plan.

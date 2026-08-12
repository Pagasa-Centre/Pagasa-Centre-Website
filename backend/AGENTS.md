# AI agent guidelines — Pag-Asa Centre backend

Read this before changing anything under `backend/`. The API uses a **layered Clean Architecture** (same conventions as [Haerd-Limited/dating-api](https://github.com/Haerd-Limited/dating-api)): handlers → services → repositories → Postgres.

## Architecture

```
Handler  (internal/api/{domain}/)     HTTP, JSON DTOs, status codes
    ↓
Service  (internal/{domain}/)         business logic, domain models
    ↓
Storage  (internal/{domain}/storage/) pgx SQL, returns domain models
    ↓
Postgres
```

**Dependency rule:** handlers import services; services import storage/domain. Storage must not import handlers or API DTOs. Register new routes in [`internal/http/router/router.go`](internal/http/router/router.go). Wire dependencies in [`cmd/api/main.go`](cmd/api/main.go) only.

## Directory layout

```
backend/
  cmd/api/main.go                 DI wiring → router.New(...)
  pkg/commonlibrary/
    db/                           pgx pool + golang-migrate
    errors/                       APIError { code, message, fields? }
    render/                       render.Json, HandleServiceErrorResponse
    request/                      request.Decode (JSON body)
  internal/
    http/router/router.go         all chi routes
    middleware/                   CORS, timeouts, admin session auth
    api/{domain}/
      handler.go                  Handler struct, methods → http.HandlerFunc
      dto/request.go              request structs (JSON tags)
      dto/response.go             response structs
      dto/mapper/                 request_to_domain.go, domain_to_response.go
    {domain}/
      service.go                  Service + business errors
      domain/                     domain models + constants
      storage/repository.go       Repository (exported type)
    config/                       env loading
    email/, sheets/, adminlog/    infra collaborators (no api/ layer)
    admin/                        shared admin helpers (actor, audit, csv)
    billing/, payment/            cross-cutting services (no separate api/ yet)
```

Existing domains: `camp`, `accommodation`, `registration`, `consent` (api only), `payment`, `admin`.

## Hard guardrails

1. **Error JSON shape is frozen.** The frontend parses `{ "code", "message", "fields"? }`. Always use `pkg/commonlibrary/errors` (`APIError`, `BadRequest`, `ValidationFailed`, `WriteError`). Never return a simplified `{ "error": "..." }` body.

2. **Logging:** stdlib `log` only. Do not add zap or structured logging libraries unless explicitly requested.

3. **Database:** raw `pgx/v5` SQL in `storage/` repositories. No sqlboiler, no ORM. Migrations stay in `migrations/` with **golang-migrate** (numbered `NNNN_name.up.sql` / `.down.sql`).

4. **Config:** keep using `internal/config.Load()`. Do not switch to viper unless asked.

5. **Repository types stay exported** (`storage.Repository`). Services that need cross-domain DB access (e.g. `billing`, `payment` using registration storage) take `*regstorage.Repository` — do not hide repos behind new interfaces unless refactoring is explicitly requested.

6. **No flat feature packages.** Do not put handlers, services, repos, and domain types in one `internal/foo/` package. Do not reintroduce `Mount()` per feature — add routes in `internal/http/router/router.go`.

7. **SSE `/camp-admin/stream`** must stay **outside** the request-timeout middleware group (see router).

## Model types (no entity layer)

We use **two** model layers (not three — there is no sqlboiler `entity` package):

| Layer | Location | Purpose |
|-------|----------|---------|
| DTO | `internal/api/{domain}/dto/` | JSON request/response contracts |
| Domain | `internal/{domain}/domain/` | business models used by services and storage |

Repositories scan SQL rows into **domain** types. Handlers decode **DTOs**, map to domain, call the service, map results back to DTOs.

Type aliases are fine when DTO and domain are identical:

```go
// internal/api/registration/dto/request.go
type SubmitRequest = domain.SubmitRequest
```

## Adding a new endpoint (existing domain)

1. Add or extend domain types in `internal/{domain}/domain/`.
2. Add service method in `internal/{domain}/service.go` (return domain types or `errors.APIError`).
3. Add storage method in `internal/{domain}/storage/repository.go` if DB access is needed.
4. Add DTOs + mappers in `internal/api/{domain}/dto/`.
5. Add `Handler` method in `internal/api/{domain}/handler.go`.
6. Register route in `internal/http/router/router.go`.
7. Run `go build ./...`, `go vet ./...`, `go test ./...`.

## Adding a new domain

Create the full scaffold:

```
internal/api/myfeature/handler.go
internal/api/myfeature/dto/request.go
internal/api/myfeature/dto/response.go
internal/api/myfeature/dto/mapper/request_to_domain.go
internal/api/myfeature/dto/mapper/domain_to_response.go
internal/myfeature/service.go
internal/myfeature/domain/domain.go
internal/myfeature/storage/repository.go
```

Then wire repo → service → handler in `cmd/api/main.go` and register routes in `router.go`.

## Handler pattern

```go
func (h *Handler) Submit() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req dto.SubmitRequest
		if err := request.Decode(r, &req); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		result, err := h.service.Submit(r.Context(), mapper.RequestToDomain(req))
		if err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		render.Json(w, http.StatusOK, mapper.SubmitToResponse(result))
	}
}
```

Map service errors to HTTP status via `errors.StatusFor` or explicit `APIError` codes. Validation uses `errors.ValidationFailed(fields)` so the frontend can show per-field messages.

## Service pattern

```go
func (s *Service) Submit(ctx context.Context, req domain.SubmitRequest) (*domain.SubmitResponse, error) {
	if err := Validate(req); err != nil {
		return nil, err
	}
	// business logic …
	if err := s.repo.InsertGroup(ctx, tx, req, total, currency); err != nil {
		return nil, commonerrors.Internal(err.Error())
	}
	return resp, nil
}
```

Keep transactions in the service when multiple storage calls must be atomic.

## Storage pattern

```go
func (r *Repository) FindGroupByID(ctx context.Context, groupID string) (*domain.Group, error) {
	const q = `SELECT ` + groupSelectCols + ` FROM registration_groups WHERE id = $1`
	g, err := scanGroup(r.pool.QueryRow(ctx, q, groupID))
	// handle pgx.ErrNoRows → (nil, nil) where appropriate
	return &g, err
}
```

## Shared packages

| Need | Package |
|------|---------|
| JSON response | `pagasacentre/backend/pkg/commonlibrary/render` |
| Decode request body | `pagasacentre/backend/pkg/commonlibrary/request` |
| API errors | `pagasacentre/backend/pkg/commonlibrary/errors` |
| DB pool / migrations | `pagasacentre/backend/pkg/commonlibrary/db` |
| CORS, timeouts | `pagasacentre/backend/internal/middleware` |
| Admin session | `pagasacentre/backend/internal/middleware` (`AuthConfig`, `RequireAdmin`) |

## Tests

- Unit tests live next to the code they test (`service_test.go`, `validate_test.go`, …).
- Integration tests use `internal/testhelper.MaybePool(t)` and require `TEST_DATABASE_URL`.
- After changes: `go test ./...` from `backend/`.

## Do not

- Collapse layers (handler calling storage directly for non-trivial logic).
- Add routes only in `main.go` — use `router.go`.
- Change migration tooling to goose or ORM to sqlboiler without explicit approval.
- Break the admin SSE stream by wrapping it in `WithRequestTimeout`.
- Remove or rename JSON fields on public API responses without checking `frontend/src/lib/api.ts`.

## When unsure

Look at an existing domain as reference — **`registration`** is the fullest example; **`camp`** is the smallest CRUD-style read API.

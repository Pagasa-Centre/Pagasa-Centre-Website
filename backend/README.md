# Pag-Asa Centre Backend

Go API for pagasacentre.org.

## Quick start (Docker — recommended)

You only need Docker Desktop (or the Docker engine + the Compose plugin).

1. `cp .env.example .env` and fill in the Stripe values (placeholders work if you don't need to test payments yet).
2. `make docker-up`

That runs Postgres + the API in containers. Migrations are applied automatically on boot. The API is at <http://localhost:8080> and Postgres is exposed on `localhost:5432` with user/pass/db all set to `pagasa`.

> Port conflicts? Set `POSTGRES_HOST_PORT` and/or `API_HOST_PORT` in `.env` before running `make docker-up`. Containers always talk to each other internally, so only the host-side binding is remapped.

Useful commands:

| Command              | What it does                                            |
| -------------------- | ------------------------------------------------------- |
| `make docker-up`     | Build image, start containers, tail API logs            |
| `make docker-down`   | Stop containers (data persists)                         |
| `make docker-clean`  | Stop containers AND wipe the Postgres volume            |
| `make docker-rebuild`| Rebuild the API image without cache and restart it      |
| `make docker-logs`   | Tail all container logs                                 |
| `make docker-psql`   | Open a `psql` shell against the running Postgres        |

When you change Go code, run `make docker-up` again to rebuild and restart the API (the build is cached so it's quick).

## Quick start (without Docker)

1. **Postgres**: have a local Postgres 14+ running. Create a database (e.g. `createdb pagasa`).
2. **Env**: `cp .env.example .env` and fill in the values.
3. **Run**: `make run` — server listens on `:8080`. Migrations run automatically on boot.

If you'd like to run migrations by hand (e.g. to test a `down`), install [golang-migrate](https://github.com/golang-migrate/migrate) (`brew install golang-migrate`) then use `make migrate-up` / `make migrate-down`.

## Stripe webhooks (either workflow)

For local payment testing, install the [Stripe CLI](https://stripe.com/docs/stripe-cli) and run, in a separate terminal:

```bash
stripe listen --forward-to localhost:8080/api/payments/webhook
```

Copy the `whsec_…` it prints into `STRIPE_WEBHOOK_SECRET` in `.env`, then restart the API (`make docker-up` or `make run`).

## Endpoints

### Public

| Method | Path                       | Purpose                                          |
| ------ | -------------------------- | ------------------------------------------------ |
| GET    | `/health`                  | Health check                                     |
| GET    | `/api/camp`                | Camp info (name, dates, location)                |
| GET    | `/api/prices`              | Current price list                               |
| GET    | `/api/accommodations`      | Accommodation types with live availability       |
| POST   | `/api/registrations`       | Submit registration form, returns Stripe URL     |
| POST   | `/api/payments/webhook`    | Stripe webhook (server-to-server)                |
| GET    | `/api/consent-form`        | Downloads the Parental Consent Form PDF          |

### Camp admin (session cookie — set `ADMIN_PASSWORD` + `ADMIN_SESSION_SECRET`)

| Method | Path                                        | Purpose                          |
| ------ | ------------------------------------------- | -------------------------------- |
| POST   | `/camp-admin/login`                              | Shared-password login            |
| GET    | `/camp-admin/registrations`                      | List groups (`?status=`, `?billing_status=`) |
| GET    | `/camp-admin/accommodations`                     | Tiers + Stripe Price ids         |
| PUT    | `/camp-admin/registrations/{groupID}/allocation` | Save per-camper allocation       |
| POST   | `/camp-admin/registrations/{groupID}/invoice`    | Create & email Stripe Invoice    |
| POST   | `/camp-admin/registrations/invoice-bulk`         | Invoice all `allocated` groups   |
| POST   | `/camp-admin/registrations/{groupID}/release`      | Void invoice & release placement |
| POST   | `/camp-admin/billing/sweep`                      | Release overdue invoiced groups  |

Frontend dashboard: `/camp-admin` (Next.js). Stripe webhooks should also subscribe to `invoice.paid`, `invoice.payment_failed`, `invoice.marked_uncollectible` on the same `/api/payments/webhook` URL.

## Project layout

```
cmd/api          entrypoint
internal/config  env loading
internal/db      pgx pool + migrations
internal/httpx   JSON, errors, middleware
internal/camp    camp config + prices read endpoints
internal/accommodation  capacity tracking
internal/registration   form submit + Stripe session creation
internal/payment        Stripe client + webhook
internal/api/campadmin  camp admin HTTP handlers
internal/admin        audit helpers + session actor context
internal/billing allocation + Stripe Invoices + overdue sweep
internal/consent parental-consent PDF
migrations       golang-migrate SQL files
static           parental-consent-form.pdf (placeholder)
```

## Tests

```bash
go test ./...
```

Integration tests for repositories and webhook concurrency are gated on
`TEST_DATABASE_URL`; without it they are skipped.

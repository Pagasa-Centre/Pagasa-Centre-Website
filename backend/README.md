# Pag-Asa Centre Backend

Go API for pagasacentre.org.

## Quick start

1. **Postgres**: have a local Postgres 14+ running. Create a database (e.g. `createdb pagasa`).
2. **Env**: `cp .env.example .env` and fill in the values.
3. **Migrate**: install [golang-migrate](https://github.com/golang-migrate/migrate) (`brew install golang-migrate`) then `make migrate-up`. Migrations are also run automatically on app boot.
4. **Stripe** (for local payment testing): install the [Stripe CLI](https://stripe.com/docs/stripe-cli), then in a separate terminal:
   ```bash
   stripe listen --forward-to localhost:8080/api/payments/webhook
   ```
   Copy the `whsec_…` it prints into `STRIPE_WEBHOOK_SECRET`.
5. **Run**: `make run` — server listens on `:8080`.

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

### Admin (not yet authenticated — internal use only)

| Method | Path                              | Purpose                              |
| ------ | --------------------------------- | ------------------------------------ |
| GET    | `/admin/registrations`            | List registration groups (JSON)      |
| GET    | `/admin/registrations.csv`        | Flat CSV, one row per camper         |
| PATCH  | `/admin/registrations/{groupID}`  | Update payment_status manually       |
| PUT    | `/admin/accommodations/{code}`    | Adjust capacity                      |
| PUT    | `/admin/prices/{code}`            | Adjust price                         |

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
internal/admin   admin endpoints
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

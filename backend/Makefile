.PHONY: run build test lint vet tidy \
        migrate-up migrate-down migrate-create \
        docker-up docker-down docker-logs docker-rebuild docker-psql docker-clean

DATABASE_URL ?= $(shell grep -E '^DATABASE_URL=' .env 2>/dev/null | cut -d '=' -f2-)

# --- Local Go workflow -------------------------------------------------------

run:
	go run ./cmd/api

build:
	go build ./...

test:
	go test ./...

lint vet:
	go vet ./...

tidy:
	go mod tidy

# --- Migrations (require the golang-migrate CLI on host) ---------------------
# Not needed for normal development: migrations also run automatically on app
# boot. Use these only when iterating on schema files against a non-dockerised
# Postgres.

migrate-up:
	migrate -database "$(DATABASE_URL)" -path migrations up

migrate-down:
	migrate -database "$(DATABASE_URL)" -path migrations down 1

migrate-create:
	@test -n "$(name)" || (echo "Usage: make migrate-create name=add_something"; exit 1)
	migrate create -ext sql -dir migrations -seq $(name)

# --- Docker workflow ---------------------------------------------------------
# `make docker-up` builds the image, starts Postgres + the API, and tails logs.
# Migrations are applied automatically when the API container boots.

docker-up:
	docker compose up --build -d
	docker compose logs -f api

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

docker-rebuild:
	docker compose build --no-cache api
	docker compose up -d api
	docker compose logs -f api

# Drop into a psql shell against the running Postgres container.
docker-psql:
	docker compose exec postgres psql -U pagasa -d pagasa

# Tear down containers AND the Postgres volume (clean slate).
docker-clean:
	docker compose down -v

.PHONY: run build test lint vet migrate-up migrate-down migrate-create tidy

DATABASE_URL ?= $(shell grep -E '^DATABASE_URL=' .env 2>/dev/null | cut -d '=' -f2-)

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

# Requires golang-migrate CLI: brew install golang-migrate
migrate-up:
	migrate -database "$(DATABASE_URL)" -path migrations up

migrate-down:
	migrate -database "$(DATABASE_URL)" -path migrations down 1

migrate-create:
	@test -n "$(name)" || (echo "Usage: make migrate-create name=add_something"; exit 1)
	migrate create -ext sql -dir migrations -seq $(name)

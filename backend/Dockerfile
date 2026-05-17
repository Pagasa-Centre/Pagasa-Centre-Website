# syntax=docker/dockerfile:1

# ---- Builder ----------------------------------------------------------------
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Download modules (cached as its own layer via go.mod/go.sum)
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# ---- Runtime ----------------------------------------------------------------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata wget \
    && adduser -D -u 10001 app

WORKDIR /app

COPY --from=builder /out/api /app/api
COPY migrations /app/migrations
COPY static /app/static

USER app

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=5 \
    CMD wget -qO- http://localhost:8080/health >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/app/api"]

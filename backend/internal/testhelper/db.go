// Package testhelper provides shared utilities for integration tests.
package testhelper

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	commondb "pagasacentre/backend/pkg/commonlibrary/db"
)

// MaybePool opens a connection pool against $TEST_DATABASE_URL, or calls
// t.Skip() if the env var is not set. The schema is migrated before returning
// and tables are truncated to give each test a clean slate.
func MaybePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

	migrationsPath := "file://" + filepath.Join(repoRoot(t), "migrations")
	if err := commondb.RunMigrations(url, migrationsPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	pool, err := commondb.New(context.Background(), url)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(context.Background(),
		`TRUNCATE TABLE registrations, registration_groups RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return pool
}

// repoRoot finds the backend/ directory regardless of which test package the
// caller lives in.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// .../backend/internal/testhelper/db.go -> .../backend
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

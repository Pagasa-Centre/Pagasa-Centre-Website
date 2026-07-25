// Package testhelper provides shared utilities for integration tests.
package testhelper

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	commondb "pagasacentre/backend/pkg/commonlibrary/db"
)

func acquireTestDBLock(t *testing.T) {
	t.Helper()
	lockPath := filepath.Join(os.TempDir(), "pagasacentre-integration-db.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatalf("open test db lock: %v", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatalf("flock test db: %v", err)
	}
	t.Cleanup(func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	})
}

// MaybePool opens a connection pool against $TEST_DATABASE_URL, or calls
// t.Skip() if the env var is not set. The schema is migrated before returning
// and tables are truncated to give each test a clean slate.
func MaybePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	acquireTestDBLock(t)

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
		`TRUNCATE TABLE registrations, registration_groups, free_codes RESTART IDENTITY CASCADE`); err != nil {
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

package db_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	commondb "pagasacentre/backend/pkg/commonlibrary/db"
)

type testDB struct {
	pool   *pgxpool.Pool
	runner *commondb.TxRunner
}

func openTestDB(t *testing.T) testDB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := commondb.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS tx_runner_test (
			id serial PRIMARY KEY,
			val text NOT NULL UNIQUE
		)`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `TRUNCATE tx_runner_test`)
	})
	return testDB{pool: pool, runner: commondb.NewTxRunner(pool)}
}

func TestWithTx_commitAndRollback(t *testing.T) {
	td := openTestDB(t)
	ctx := context.Background()

	if err := td.runner.WithTx(ctx, func(ctx context.Context) error {
		_, err := commondb.Conn(ctx, td.pool).Exec(ctx, `INSERT INTO tx_runner_test (val) VALUES ('committed')`)
		return err
	}); err != nil {
		t.Fatalf("commit: %v", err)
	}
	var n int
	if err := td.pool.QueryRow(ctx, `SELECT count(*) FROM tx_runner_test WHERE val = 'committed'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("expected committed row, count=%d err=%v", n, err)
	}

	boom := errors.New("boom")
	err := td.runner.WithTx(ctx, func(ctx context.Context) error {
		_, e := commondb.Conn(ctx, td.pool).Exec(ctx, `INSERT INTO tx_runner_test (val) VALUES ('rolled')`)
		if e != nil {
			return e
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("expected boom, got %v", err)
	}
	if err := td.pool.QueryRow(ctx, `SELECT count(*) FROM tx_runner_test WHERE val = 'rolled'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("expected no rolled row, count=%d", n)
	}
}

func TestWithTx_connSeesUncommittedInsideFn(t *testing.T) {
	td := openTestDB(t)
	ctx := context.Background()
	seen := false
	_ = td.runner.WithTx(ctx, func(ctx context.Context) error {
		_, err := commondb.Conn(ctx, td.pool).Exec(ctx, `INSERT INTO tx_runner_test (val) VALUES ('inner')`)
		if err != nil {
			return err
		}
		err = commondb.Conn(ctx, td.pool).QueryRow(ctx, `SELECT true FROM tx_runner_test WHERE val = 'inner'`).Scan(&seen)
		return err
	})
	if !seen {
		t.Fatal("expected to see row inside same transaction")
	}
}

func TestWithTx_savepointRollbackPreservesOuter(t *testing.T) {
	td := openTestDB(t)
	ctx := context.Background()
	if err := td.runner.WithTx(ctx, func(outer context.Context) error {
		_, err := commondb.Conn(outer, td.pool).Exec(outer, `INSERT INTO tx_runner_test (val) VALUES ('outer')`)
		if err != nil {
			return err
		}
		sp, err := commondb.Begin(outer, td.pool)
		if err != nil {
			return err
		}
		_, err = sp.Exec(outer, `INSERT INTO tx_runner_test (val) VALUES ('inner')`)
		if err != nil {
			_ = sp.Rollback(outer)
			return err
		}
		return sp.Rollback(outer)
	}); err != nil {
		t.Fatalf("commit outer: %v", err)
	}
	var outerN, innerN int
	_ = td.pool.QueryRow(ctx, `SELECT count(*) FROM tx_runner_test WHERE val = 'outer'`).Scan(&outerN)
	_ = td.pool.QueryRow(ctx, `SELECT count(*) FROM tx_runner_test WHERE val = 'inner'`).Scan(&innerN)
	if outerN != 1 || innerN != 0 {
		t.Fatalf("want outer=1 inner=0, got outer=%d inner=%d", outerN, innerN)
	}
}

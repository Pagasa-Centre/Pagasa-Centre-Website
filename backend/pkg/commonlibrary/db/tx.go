package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ctxKey struct{}

// Querier is the subset of pgx pool/tx used by repositories.
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// TxRunner runs callbacks inside a database transaction carried on context.
type TxRunner struct {
	pool *pgxpool.Pool
}

func NewTxRunner(pool *pgxpool.Pool) *TxRunner {
	return &TxRunner{pool: pool}
}

func txFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(ctxKey{}).(pgx.Tx)
	return tx, ok
}

func withTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, ctxKey{}, tx)
}

// Conn returns the transaction from ctx when present, otherwise the pool.
func Conn(ctx context.Context, pool *pgxpool.Pool) Querier {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return pool
}

// Begin starts a transaction, or a savepoint when ctx already carries a transaction.
func Begin(ctx context.Context, pool *pgxpool.Pool) (pgx.Tx, error) {
	if tx, ok := txFromContext(ctx); ok {
		sp, err := tx.Begin(ctx)
		if err != nil {
			return nil, fmt.Errorf("begin savepoint: %w", err)
		}
		return sp, nil
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	return tx, nil
}

// WithTx runs fn in a transaction stored on the context passed to fn.
// fn's error is returned unchanged. Begin and commit failures are wrapped.
func (t *TxRunner) WithTx(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	txCtx := withTx(ctx, tx)
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()
	if err := fn(txCtx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

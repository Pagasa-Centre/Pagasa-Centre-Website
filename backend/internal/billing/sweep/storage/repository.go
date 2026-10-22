package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/registration/storage/groupsql"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) ListOverdueInvoiced(ctx context.Context, now time.Time) ([]domain.Group, error) {
	const q = `SELECT ` + groupsql.GroupSelectCols + `
		  FROM registration_groups
		 WHERE billing_status = $2
		   AND invoice_due_at IS NOT NULL
		   AND invoice_due_at < $1`
	rows, err := r.pool.Query(ctx, q, now.UTC(), domain.BillingInvoiced)
	if err != nil {
		return nil, fmt.Errorf("list overdue invoiced: %w", err)
	}
	defer rows.Close()

	var out []domain.Group
	for rows.Next() {
		g, err := groupsql.ScanGroup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

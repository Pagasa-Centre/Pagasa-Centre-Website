package accommodation

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// ListOptions returns every accommodation type, ordered by sort_order then code.
// v2: no availability / capacity logic — committee allocates manually.
func (r *Repository) ListOptions(ctx context.Context) ([]Option, error) {
	const q = `
		SELECT code, display_name, COALESCE(notes, '')
		  FROM accommodation_types
		 ORDER BY sort_order, code`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list accommodations: %w", err)
	}
	defer rows.Close()

	var out []Option
	for rows.Next() {
		var o Option
		if err := rows.Scan(&o.Code, &o.DisplayName, &o.Notes); err != nil {
			return nil, fmt.Errorf("scan accommodation: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

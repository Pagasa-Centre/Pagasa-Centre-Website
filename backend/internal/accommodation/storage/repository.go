package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"pagasacentre/backend/internal/accommodation/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) ListOptions(ctx context.Context) ([]domain.Option, error) {
	const q = `
		SELECT code, display_name, COALESCE(notes, ''), available_for_registration
		  FROM accommodation_types
		 ORDER BY sort_order, code`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list accommodations: %w", err)
	}
	defer rows.Close()

	var out []domain.Option
	for rows.Next() {
		var o domain.Option
		if err := rows.Scan(&o.Code, &o.DisplayName, &o.Notes, &o.AvailableForRegistration); err != nil {
			return nil, fmt.Errorf("scan accommodation: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

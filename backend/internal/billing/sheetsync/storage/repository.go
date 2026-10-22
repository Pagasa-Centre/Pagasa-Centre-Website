package storage

import (
	"context"

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

func (r *Repository) StampOnly(ctx context.Context, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error {
	return groupsql.StampOnly(ctx, r.pool, groupID, meta, tr)
}

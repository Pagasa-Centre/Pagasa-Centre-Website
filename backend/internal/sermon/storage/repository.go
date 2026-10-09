package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pagasacentre/backend/internal/sermon/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Latest(ctx context.Context) (*domain.Sermon, error) {
	const q = `SELECT video_id, title, published_at FROM latest_sermon WHERE id = TRUE`
	var s domain.Sermon
	var publishedAt *time.Time
	err := r.pool.QueryRow(ctx, q).Scan(&s.VideoID, &s.Title, &publishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("latest sermon: %w", err)
	}
	if publishedAt != nil {
		s.PublishedAt = *publishedAt
	}
	return &s, nil
}

func (r *Repository) Save(ctx context.Context, s domain.Sermon) error {
	var publishedAt *time.Time
	if !s.PublishedAt.IsZero() {
		t := s.PublishedAt
		publishedAt = &t
	}
	const q = `
INSERT INTO latest_sermon (id, video_id, title, published_at, updated_at)
VALUES (TRUE, $1, $2, $3, now())
ON CONFLICT (id) DO UPDATE SET
    video_id = EXCLUDED.video_id,
    title = EXCLUDED.title,
    published_at = EXCLUDED.published_at,
    updated_at = now()`
	_, err := r.pool.Exec(ctx, q, s.VideoID, s.Title, publishedAt)
	if err != nil {
		return fmt.Errorf("save latest sermon: %w", err)
	}
	return nil
}

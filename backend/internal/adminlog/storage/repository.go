package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pagasacentre/backend/internal/adminlog"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Insert(ctx context.Context, actor, action string, groupID any, summary string, meta []byte) (adminlog.Event, error) {
	var ev adminlog.Event
	err := r.pool.QueryRow(ctx,
		`INSERT INTO admin_events (actor_name, action, group_id, summary, metadata)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at, actor_name, action, group_id, summary, metadata`,
		actor, action, groupID, summary, meta,
	).Scan(&ev.ID, &ev.CreatedAt, &ev.ActorName, &ev.Action, &ev.GroupID, &ev.Summary, &ev.Metadata)
	if err != nil {
		return adminlog.Event{}, fmt.Errorf("insert admin_event: %w", err)
	}
	return ev, nil
}

func (r *Repository) List(ctx context.Context, limit int, beforeID int64) ([]adminlog.Event, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if beforeID > 0 {
		rows, err = r.pool.Query(ctx,
			`SELECT id, created_at, actor_name, action, group_id, summary, metadata
			   FROM admin_events
			  WHERE id < $1
			  ORDER BY id DESC
			  LIMIT $2`, beforeID, limit)
	} else {
		rows, err = r.pool.Query(ctx,
			`SELECT id, created_at, actor_name, action, group_id, summary, metadata
			   FROM admin_events
			  ORDER BY id DESC
			  LIMIT $1`, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list admin_events: %w", err)
	}
	defer rows.Close()
	var out []adminlog.Event
	for rows.Next() {
		var ev adminlog.Event
		if err := rows.Scan(&ev.ID, &ev.CreatedAt, &ev.ActorName, &ev.Action, &ev.GroupID, &ev.Summary, &ev.Metadata); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

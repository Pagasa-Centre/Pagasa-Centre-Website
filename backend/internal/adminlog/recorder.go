package adminlog

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Recorder persists admin_events and broadcasts over the Hub.
type Recorder struct {
	pool *pgxpool.Pool
	hub  *Hub
}

func NewRecorder(pool *pgxpool.Pool, hub *Hub) *Recorder {
	return &Recorder{pool: pool, hub: hub}
}

// Record inserts an audit row and broadcasts it to SSE subscribers.
func (rec *Recorder) Record(ctx context.Context, actor, action string, groupID *string, summary string, meta any) (Event, error) {
	var metaJSON []byte
	if meta != nil {
		var err error
		metaJSON, err = json.Marshal(meta)
		if err != nil {
			return Event{}, fmt.Errorf("marshal metadata: %w", err)
		}
	}
	var gid any
	if groupID != nil && *groupID != "" {
		gid = *groupID
	}
	var ev Event
	err := rec.pool.QueryRow(ctx,
		`INSERT INTO admin_events (actor_name, action, group_id, summary, metadata)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at, actor_name, action, group_id, summary, metadata`,
		actor, action, gid, summary, metaJSON,
	).Scan(&ev.ID, &ev.CreatedAt, &ev.ActorName, &ev.Action, &ev.GroupID, &ev.Summary, &ev.Metadata)
	if err != nil {
		return Event{}, fmt.Errorf("insert admin_event: %w", err)
	}
	rec.hub.Broadcast(ev)
	return ev, nil
}

// List returns recent events, optionally before a cursor id.
func (rec *Recorder) List(ctx context.Context, limit int, beforeID int64) ([]Event, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var (
		rows pgxRows
		err  error
	)
	if beforeID > 0 {
		rows, err = rec.pool.Query(ctx,
			`SELECT id, created_at, actor_name, action, group_id, summary, metadata
			   FROM admin_events
			  WHERE id < $1
			  ORDER BY id DESC
			  LIMIT $2`, beforeID, limit)
	} else {
		rows, err = rec.pool.Query(ctx,
			`SELECT id, created_at, actor_name, action, group_id, summary, metadata
			   FROM admin_events
			  ORDER BY id DESC
			  LIMIT $1`, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list admin_events: %w", err)
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var ev Event
		if err := rows.Scan(&ev.ID, &ev.CreatedAt, &ev.ActorName, &ev.Action, &ev.GroupID, &ev.Summary, &ev.Metadata); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

// pgxRows matches *pgx.Rows without exporting pgx from this helper.
type pgxRows interface {
	Next() bool
	Scan(dest ...any) error
	Close()
	Err() error
}

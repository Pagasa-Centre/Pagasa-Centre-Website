package adminlog

import (
	"context"
	"encoding/json"
	"fmt"
)

type eventStore interface {
	Insert(ctx context.Context, actor, action string, groupID any, summary string, meta []byte) (Event, error)
	List(ctx context.Context, limit int, beforeID int64) ([]Event, error)
}

// Recorder persists admin_events and broadcasts over the Hub.
type Recorder struct {
	store eventStore
	hub   *Hub
}

func NewRecorder(store eventStore, hub *Hub) *Recorder {
	return &Recorder{store: store, hub: hub}
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
	ev, err := rec.store.Insert(ctx, actor, action, gid, summary, metaJSON)
	if err != nil {
		return Event{}, err
	}
	rec.hub.Broadcast(ev)
	return ev, nil
}

// List returns recent events, optionally before a cursor id.
func (rec *Recorder) List(ctx context.Context, limit int, beforeID int64) ([]Event, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return rec.store.List(ctx, limit, beforeID)
}

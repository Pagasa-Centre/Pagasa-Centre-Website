package adminlog

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type fakeEventStore struct {
	inserted []byte
	listLim  int
}

func (f *fakeEventStore) Insert(_ context.Context, actor, action string, groupID any, summary string, meta []byte) (Event, error) {
	f.inserted = append([]byte(nil), meta...)
	return Event{
		ID:        1,
		CreatedAt: time.Now().UTC(),
		ActorName: actor,
		Action:    action,
		Summary:   summary,
		Metadata:  json.RawMessage(meta),
	}, nil
}

func (f *fakeEventStore) List(_ context.Context, limit int, _ int64) ([]Event, error) {
	f.listLim = limit
	return nil, nil
}

func TestRecord_MarshalsMetadataAndBroadcasts(t *testing.T) {
	store := &fakeEventStore{}
	hub := NewHub()
	ch, unsub := hub.Subscribe()
	defer unsub()

	rec := NewRecorder(store, hub)
	meta := map[string]string{"code": "lodge"}
	ev, err := rec.Record(context.Background(), "admin", ActionAccommodationAvailability, nil, "hid lodge", meta)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if ev.Action != ActionAccommodationAvailability {
		t.Fatalf("unexpected event: %+v", ev)
	}
	var got map[string]string
	if err := json.Unmarshal(store.inserted, &got); err != nil || got["code"] != "lodge" {
		t.Fatalf("stored meta: %q err=%v", store.inserted, err)
	}
	select {
	case b := <-ch:
		if b.ID != ev.ID || b.Action != ev.Action {
			t.Fatalf("broadcast mismatch: %+v vs %+v", b, ev)
		}
	default:
		t.Fatal("expected hub broadcast")
	}
}

func TestList_ClampLimit(t *testing.T) {
	store := &fakeEventStore{}
	rec := NewRecorder(store, NewHub())
	if _, err := rec.List(context.Background(), 0, 0); err != nil {
		t.Fatalf("List: %v", err)
	}
	if store.listLim != 50 {
		t.Fatalf("zero limit: got %d want 50", store.listLim)
	}
	store.listLim = 0
	if _, err := rec.List(context.Background(), 500, 0); err != nil {
		t.Fatalf("List: %v", err)
	}
	if store.listLim != 50 {
		t.Fatalf("500 limit: got %d want 50", store.listLim)
	}
}

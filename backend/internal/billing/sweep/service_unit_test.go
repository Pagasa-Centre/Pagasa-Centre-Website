package sweep

import (
	"context"
	"errors"
	"testing"
	"time"

	"pagasacentre/backend/internal/registration/domain"
)

type fakeOverdue struct {
	groups []domain.Group
	err    error
}

func (f *fakeOverdue) ListOverdueInvoiced(context.Context, time.Time) ([]domain.Group, error) {
	return f.groups, f.err
}

type fakeReleaser struct {
	calls []string
	err   error
}

func (f *fakeReleaser) VoidAndRelease(_ context.Context, groupID, _, _ string, _ int, _ bool) error {
	f.calls = append(f.calls, groupID)
	return f.err
}

func TestSweepOverdue_releasesAll(t *testing.T) {
	list := &fakeOverdue{groups: []domain.Group{{ID: "a"}, {ID: "b"}}}
	rel := &fakeReleaser{}
	svc := NewService(list, rel)
	n, err := svc.SweepOverdue(context.Background())
	if err != nil {
		t.Fatalf("SweepOverdue: %v", err)
	}
	if n != 2 || len(rel.calls) != 2 {
		t.Fatalf("n=%d calls=%v, want 2 releases", n, rel.calls)
	}
}

type flakyReleaser struct {
	attempts int
}

func (f *flakyReleaser) VoidAndRelease(_ context.Context, groupID, _, _ string, _ int, _ bool) error {
	f.attempts++
	if groupID == "fail" {
		return errors.New("stripe down")
	}
	return nil
}

func TestSweepOverdue_countsSuccesses(t *testing.T) {
	list := &fakeOverdue{groups: []domain.Group{{ID: "ok1"}, {ID: "fail"}, {ID: "ok2"}}}
	rel := &flakyReleaser{}
	svc := NewService(list, rel)
	n, err := svc.SweepOverdue(context.Background())
	if err != nil {
		t.Fatalf("SweepOverdue: %v", err)
	}
	if n != 2 {
		t.Fatalf("success count = %d, want 2 (continues after error)", n)
	}
}

func TestSweepOverdue_continuesOnError(t *testing.T) {
	list := &fakeOverdue{groups: []domain.Group{{ID: "g1"}}}
	rel := &fakeReleaser{err: errors.New("release failed")}
	svc := NewService(list, rel)
	n, err := svc.SweepOverdue(context.Background())
	if err != nil {
		t.Fatalf("SweepOverdue: %v", err)
	}
	if n != 0 {
		t.Fatalf("n = %d, want 0 when every release fails", n)
	}
	if len(rel.calls) != 1 {
		t.Fatalf("calls = %v, want one attempt", rel.calls)
	}
}

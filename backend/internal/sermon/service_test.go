package sermon_test

import (
	"context"
	"errors"
	"testing"

	"pagasacentre/backend/internal/sermon"
	"pagasacentre/backend/internal/sermon/domain"
)

type fakeFinder struct {
	sermon *domain.Sermon
	err    error
}

func (f *fakeFinder) LatestCompleted(context.Context) (*domain.Sermon, error) {
	return f.sermon, f.err
}

type fakeRepo struct {
	latest *domain.Sermon
	saved  *domain.Sermon
}

func (r *fakeRepo) Latest(context.Context) (*domain.Sermon, error) {
	return r.latest, nil
}

func (r *fakeRepo) Save(_ context.Context, s domain.Sermon) error {
	r.saved = &s
	return nil
}

func TestSync_SavesWhenFinderReturnsVideo(t *testing.T) {
	repo := &fakeRepo{}
	finder := &fakeFinder{sermon: &domain.Sermon{VideoID: "abc123", Title: "Sunday service"}}
	svc := sermon.NewService(repo, finder)

	if err := svc.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if repo.saved == nil || repo.saved.VideoID != "abc123" {
		t.Fatalf("expected save, got %+v", repo.saved)
	}
}

func TestSync_NoSaveWhenFinderReturnsNil(t *testing.T) {
	repo := &fakeRepo{}
	finder := &fakeFinder{sermon: nil}
	svc := sermon.NewService(repo, finder)

	if err := svc.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if repo.saved != nil {
		t.Fatal("expected no save")
	}
}

func TestSync_NoSaveWhenFinderErrors(t *testing.T) {
	repo := &fakeRepo{}
	finder := &fakeFinder{err: errors.New("youtube down")}
	svc := sermon.NewService(repo, finder)

	if err := svc.Sync(context.Background()); err == nil {
		t.Fatal("expected error")
	}
	if repo.saved != nil {
		t.Fatal("expected no save on finder error")
	}
}

func TestLatest_NotFoundWhenEmpty(t *testing.T) {
	repo := &fakeRepo{latest: nil}
	svc := sermon.NewService(repo, nil)

	_, err := svc.Latest(context.Background())
	if !errors.Is(err, sermon.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestLatest_ReturnsStored(t *testing.T) {
	repo := &fakeRepo{latest: &domain.Sermon{VideoID: "xyz", Title: "Live"}}
	svc := sermon.NewService(repo, nil)

	got, err := svc.Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got.VideoID != "xyz" {
		t.Fatalf("got %q", got.VideoID)
	}
}

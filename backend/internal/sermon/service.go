package sermon

import (
	"context"
	"errors"

	"pagasacentre/backend/internal/sermon/domain"
)

var ErrNotFound = errors.New("latest sermon not found")

type LiveFinder interface {
	LatestCompleted(ctx context.Context) (*domain.Sermon, error)
}

type sermonRepo interface {
	Latest(ctx context.Context) (*domain.Sermon, error)
	Save(ctx context.Context, s domain.Sermon) error
}

type Service struct {
	repo   sermonRepo
	finder LiveFinder
}

func NewService(repo sermonRepo, finder LiveFinder) *Service {
	return &Service{repo: repo, finder: finder}
}

func (s *Service) Sync(ctx context.Context) error {
	if s.finder == nil {
		return nil
	}
	latest, err := s.finder.LatestCompleted(ctx)
	if err != nil {
		return err
	}
	if latest == nil {
		return nil
	}
	return s.repo.Save(ctx, *latest)
}

func (s *Service) Latest(ctx context.Context) (*domain.Sermon, error) {
	sermon, err := s.repo.Latest(ctx)
	if err != nil {
		return nil, err
	}
	if sermon == nil {
		return nil, ErrNotFound
	}
	return sermon, nil
}

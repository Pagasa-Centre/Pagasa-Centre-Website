package accommodation

import (
	"context"

	"pagasacentre/backend/internal/accommodation/domain"
)

type accommodationRepo interface {
	ListOptions(ctx context.Context) ([]domain.Option, error)
}

type Service struct {
	repo accommodationRepo
}

func NewService(repo accommodationRepo) *Service { return &Service{repo: repo} }

func (s *Service) ListOptions(ctx context.Context) ([]domain.Option, error) {
	return s.repo.ListOptions(ctx)
}

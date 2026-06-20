package accommodation

import (
	"context"

	"pagasacentre/backend/internal/accommodation/domain"
	"pagasacentre/backend/internal/accommodation/storage"
)

type Service struct {
	repo *storage.Repository
}

func NewService(repo *storage.Repository) *Service { return &Service{repo: repo} }

func (s *Service) ListOptions(ctx context.Context) ([]domain.Option, error) {
	return s.repo.ListOptions(ctx)
}

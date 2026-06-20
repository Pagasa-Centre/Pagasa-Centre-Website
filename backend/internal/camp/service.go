package camp

import (
	"context"

	"pagasacentre/backend/internal/camp/domain"
	"pagasacentre/backend/internal/camp/storage"
)

type Service struct {
	repo *storage.Repository
}

func NewService(repo *storage.Repository) *Service { return &Service{repo: repo} }

func (s *Service) GetConfig(ctx context.Context) (domain.Config, error) {
	return s.repo.GetConfig(ctx)
}

func (s *Service) ListPrices(ctx context.Context) ([]domain.Price, error) {
	return s.repo.ListPrices(ctx)
}

func (s *Service) RegistrationsOpen(ctx context.Context) (bool, error) {
	return s.repo.RegistrationsOpen(ctx)
}

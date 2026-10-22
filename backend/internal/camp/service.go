package camp

import (
	"context"

	"pagasacentre/backend/internal/camp/domain"
)

type campRepo interface {
	GetConfig(ctx context.Context) (domain.Config, error)
	ListPrices(ctx context.Context) ([]domain.Price, error)
	RegistrationsOpen(ctx context.Context) (bool, error)
	SetRegistrationsOpen(ctx context.Context, open bool) error
	RegistrationPaymentMode(ctx context.Context) (string, error)
	SetRegistrationPaymentMode(ctx context.Context, mode string) error
	GetPrice(ctx context.Context, code string) (domain.Price, error)
	UpdatePrice(ctx context.Context, code string, amountPence int) error
}

type Service struct {
	repo campRepo
}

func NewService(repo campRepo) *Service { return &Service{repo: repo} }

func (s *Service) GetConfig(ctx context.Context) (domain.Config, error) {
	return s.repo.GetConfig(ctx)
}

func (s *Service) ListPrices(ctx context.Context) ([]domain.Price, error) {
	return s.repo.ListPrices(ctx)
}

func (s *Service) RegistrationsOpen(ctx context.Context) (bool, error) {
	return s.repo.RegistrationsOpen(ctx)
}

func (s *Service) SetRegistrationsOpen(ctx context.Context, open bool) error {
	return s.repo.SetRegistrationsOpen(ctx, open)
}

func (s *Service) SetRegistrationPaymentMode(ctx context.Context, mode string) error {
	return s.repo.SetRegistrationPaymentMode(ctx, mode)
}

func (s *Service) UpdatePrice(ctx context.Context, code string, amountPence int) error {
	return s.repo.UpdatePrice(ctx, code, amountPence)
}

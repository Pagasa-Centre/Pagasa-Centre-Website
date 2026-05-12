package accommodation

import "context"

// Service is a thin layer over the repository so handlers don't talk to SQL
// directly. Future caching / business rules go here.
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

func (s *Service) ListAvailability(ctx context.Context) ([]Availability, error) {
	return s.repo.ListWithAvailability(ctx)
}

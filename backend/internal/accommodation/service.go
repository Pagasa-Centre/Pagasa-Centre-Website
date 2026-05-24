package accommodation

import "context"

// Service is a thin layer over the repository so handlers don't talk to SQL
// directly.
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// ListOptions returns the catalogue of accommodation options.
func (s *Service) ListOptions(ctx context.Context) ([]Option, error) {
	return s.repo.ListOptions(ctx)
}

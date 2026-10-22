package sweep

import (
	"context"
	"time"

	"pagasacentre/backend/internal/registration/domain"
)

type overdueLister interface {
	ListOverdueInvoiced(ctx context.Context, now time.Time) ([]domain.Group, error)
}

type releaser interface {
	VoidAndRelease(ctx context.Context, groupID, reason, actor string, expectedVersion int, cancelled bool) error
}

type Service struct {
	repo     overdueLister
	releaser releaser
}

func NewService(repo overdueLister, releaser releaser) *Service {
	return &Service{repo: repo, releaser: releaser}
}

package sweep

import (
	"context"
	"log"
	"time"

	"pagasacentre/backend/internal/registration/domain"
)

func (s *Service) SweepOverdue(ctx context.Context) (int, error) {
	overdue, err := s.repo.ListOverdueInvoiced(ctx, time.Now().UTC())
	if err != nil {
		return 0, err
	}
	n := 0
	for _, g := range overdue {
		if err := s.releaser.VoidAndRelease(ctx, g.ID, "unpaid after invoice due date", "system", domain.SkipVersionCheck, false); err != nil {
			log.Printf("billing: sweep release group %s: %v", g.ID, err)
			continue
		}
		n++
	}
	return n, nil
}

package reconcile

import (
	"context"
	"testing"
	"time"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/registration/domain"
)

type recordingCoachRepo struct {
	recordingReconRepo
	coachMarkCalls int
}

func (r *recordingCoachRepo) MarkCoachFeePaidMeta(context.Context, string, domain.ActionMeta) error {
	r.coachMarkCalls++
	return nil
}

func TestHandleCoachInvoicePaid_idempotent(t *testing.T) {
	paid := time.Now()
	g := &domain.Group{
		ID:             "g1",
		CoachFeePaidAt: &paid,
		PaymentStatus:  domain.PaymentPaid,
	}
	repo := &recordingCoachRepo{}
	pricer := billing.NewPricer(fakeAcc{}, billing.Config{}, nil)
	svc := NewService(&fakeReads{byID: g}, repo, noopMail{}, pricer, billing.Config{})
	if err := svc.HandleCoachInvoicePaid(context.Background(), "in_coach", "g1"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if repo.coachMarkCalls != 0 {
		t.Fatalf("MarkCoachFeePaidMeta calls = %d, want 0", repo.coachMarkCalls)
	}
}

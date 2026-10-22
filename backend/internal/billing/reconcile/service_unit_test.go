package reconcile

import (
	"context"
	"testing"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/notify"
	"pagasacentre/backend/internal/registration/domain"
)

type noopMail struct{}

func (noopMail) SendBalancePaidConfirmation(context.Context, notify.BalancePaidConfirmation) error {
	return nil
}
func (noopMail) SendBalancePaid(context.Context, notify.BalancePaid) error { return nil }

type fakeAcc struct{}

func (fakeAcc) GetAccommodationType(context.Context, string) (*domain.AccommodationType, error) {
	return nil, nil
}
func (fakeAcc) GetAccommodationUnit(context.Context, string) (*domain.AccommodationUnit, error) {
	return nil, nil
}
func (fakeAcc) ListAccommodationTypes(context.Context) ([]domain.AccommodationType, error) {
	return nil, nil
}
func (fakeAcc) ListAccommodationUnits(context.Context) ([]domain.AccommodationUnit, error) {
	return nil, nil
}

type fakeReads struct {
	byID      *domain.Group
	byInvoice *domain.Group
}

func (f *fakeReads) FindGroupByID(_ context.Context, id string) (*domain.Group, error) {
	if f.byID == nil || f.byID.ID != id {
		return nil, nil
	}
	cp := *f.byID
	return &cp, nil
}

func (f *fakeReads) CampersForGroup(context.Context, string) ([]domain.Camper, error) {
	return nil, nil
}

type fakeReconRepo struct {
	byInvoice *domain.Group
}

func (f *fakeReconRepo) FindGroupByBalanceInvoiceRef(_ context.Context, _ string) (*domain.Group, error) {
	if f.byInvoice == nil {
		return nil, nil
	}
	cp := *f.byInvoice
	return &cp, nil
}

func (f *fakeReconRepo) FindGroupByCoachInvoiceRef(context.Context, string) (*domain.Group, error) {
	return nil, nil
}

func (f *fakeReconRepo) MarkCoachFeePaidMeta(context.Context, string, domain.ActionMeta) error {
	return nil
}

type recordingReconRepo struct {
	fakeReconRepo
	markCalls int
}

func (r *recordingReconRepo) MarkBalancePaidMeta(context.Context, string, domain.ActionMeta, domain.BillingTransition) error {
	r.markCalls++
	return nil
}

func TestHandleInvoicePaid_settledNoOp(t *testing.T) {
	g := &domain.Group{
		ID:            "g1",
		BillingStatus: domain.BillingBalancePaid,
		PaymentStatus: domain.PaymentPaid,
	}
	repo := &recordingReconRepo{}
	pricer := billing.NewPricer(fakeAcc{}, billing.Config{}, nil)
	svc := NewService(&fakeReads{byID: g}, repo, noopMail{}, pricer, billing.Config{})
	if err := svc.HandleInvoicePaid(context.Background(), "in_1", "g1", 1000, "gbp"); err != nil {
		t.Fatalf("expected nil for settled group, got %v", err)
	}
	if repo.markCalls != 0 {
		t.Fatalf("MarkBalancePaidMeta calls = %d, want 0", repo.markCalls)
	}
}

func TestHandleInvoicePaid_findGroupByBalanceInvoiceRefFallback(t *testing.T) {
	g := &domain.Group{
		ID:            "g-from-invoice",
		BillingStatus: domain.BillingInvoiced,
		PaymentStatus: domain.PaymentPaid,
	}
	repo := &recordingReconRepo{fakeReconRepo: fakeReconRepo{byInvoice: g}}
	pricer := billing.NewPricer(fakeAcc{}, billing.Config{}, nil)
	svc := NewService(&fakeReads{}, repo, noopMail{}, pricer, billing.Config{})
	if err := svc.HandleInvoicePaid(context.Background(), "in_fallback", "", 5000, "gbp"); err != nil {
		t.Fatalf("HandleInvoicePaid: %v", err)
	}
	if repo.markCalls != 1 {
		t.Fatalf("MarkBalancePaidMeta calls = %d, want 1", repo.markCalls)
	}
}

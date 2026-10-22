package sheetsync

import (
	"context"
	"errors"
	"testing"

	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/sheets"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

type fakeReads struct {
	group   *domain.Group
	campers []domain.Camper
}

func (f *fakeReads) FindGroupByID(_ context.Context, _ string) (*domain.Group, error) {
	if f.group == nil {
		return nil, nil
	}
	cp := *f.group
	return &cp, nil
}

func (f *fakeReads) CampersForGroup(context.Context, string) ([]domain.Camper, error) {
	return f.campers, nil
}

func (f *fakeReads) ListWithBilling(context.Context, domain.ListFilterBilling) ([]domain.Group, error) {
	return nil, nil
}

type noopSheetRepo struct{}

func (noopSheetRepo) StampOnly(context.Context, string, domain.ActionMeta, domain.BillingTransition) error {
	return nil
}

func TestResyncSheet_rejectsBadPaymentStatus(t *testing.T) {
	reads := &fakeReads{
		group:   &domain.Group{ID: "g1", PaymentStatus: domain.PaymentStatus("refunded")},
		campers: []domain.Camper{{ID: "c1", FirstName: "A", LastName: "B"}},
	}
	svc := NewService(reads, noopSheetRepo{}, sheets.NoopSync{})
	err := svc.ResyncSheet(context.Background(), "g1", "admin", 1)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind != apperr.KindInvalid {
		t.Fatalf("expected invalid payment status, got %v", err)
	}
}

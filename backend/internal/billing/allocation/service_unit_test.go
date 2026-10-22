package allocation

import (
	"context"
	"errors"
	"strings"
	"testing"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

type fakeReads struct {
	groups  map[string]*domain.Group
	campers map[string][]domain.Camper
}

func (f *fakeReads) FindGroupByID(_ context.Context, id string) (*domain.Group, error) {
	if f.groups == nil {
		return nil, nil
	}
	g := f.groups[id]
	if g == nil {
		return nil, nil
	}
	cp := *g
	return &cp, nil
}

func (f *fakeReads) CampersForGroup(_ context.Context, id string) ([]domain.Camper, error) {
	return f.campers[id], nil
}

type fakeAllocRepo struct {
	err error
}

func (f *fakeAllocRepo) AllocateGroup(context.Context, string, []domain.CamperAllocation, domain.ActionMeta, domain.BillingTransition) error {
	return f.err
}
func (f *fakeAllocRepo) UnallocateGroup(context.Context, string, domain.ActionMeta, domain.BillingTransition) error {
	return nil
}
func (f *fakeAllocRepo) ConfirmFreeMeta(context.Context, string, domain.ActionMeta, domain.BillingTransition) error {
	return nil
}
func (f *fakeAllocRepo) ClearInvoiceAndReleaseMeta(context.Context, string, domain.ActionMeta, domain.BillingTransition) error {
	return nil
}

func TestAllocate_VersionConflictMapsToStaleState(t *testing.T) {
	who := "Alice"
	reads := &fakeReads{
		groups: map[string]*domain.Group{"g1": {
			ID: "g1", PaymentStatus: domain.PaymentPaid, BillingStatus: domain.BillingNone, LastActionBy: &who,
		}},
		campers: map[string][]domain.Camper{"g1": {{
			ID: "c1", FirstName: "Pat", LastName: "Camper", Age: 25, AttendanceType: domain.AttendanceFullWeek,
		}}},
	}
	svc := NewService(reads, &fakeAllocRepo{err: domain.ErrVersionConflict}, nil, nil, nil, billing.Config{})
	err := svc.Allocate(context.Background(), "g1", "admin", 1, AllocateRequest{
		Campers: []AllocateCamper{{CamperID: "c1", AllocatedAccommodationCode: "lodge", BilledStripePriceID: "price_test"}},
	})
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != "stale_state" {
		t.Fatalf("expected stale_state, got %v", err)
	}
	if !strings.Contains(ae.Message, "Alice") {
		t.Fatalf("message = %q", ae.Message)
	}
}

func TestAllocate_RepositoryErrorWrapped(t *testing.T) {
	cause := errors.New("db down")
	reads := &fakeReads{
		groups: map[string]*domain.Group{"g1": {
			ID: "g1", PaymentStatus: domain.PaymentPaid, BillingStatus: domain.BillingNone,
		}},
		campers: map[string][]domain.Camper{"g1": {{
			ID: "c1", FirstName: "Pat", LastName: "Camper", Age: 25, AttendanceType: domain.AttendanceFullWeek,
		}}},
	}
	svc := NewService(reads, &fakeAllocRepo{err: cause}, nil, nil, nil, billing.Config{})
	err := svc.Allocate(context.Background(), "g1", "admin", 1, AllocateRequest{
		Campers: []AllocateCamper{{
			CamperID: "c1", AllocatedAccommodationCode: "lodge", BilledStripePriceID: "price_test",
		}},
	})
	var ae *apperr.Error
	if errors.As(err, &ae) {
		t.Fatalf("expected non-apperr wrap, got apperr %+v", ae)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("expected wrapped cause, got %v", err)
	}
}

func TestConfirmFree_rejectsNonFreeGroup(t *testing.T) {
	reads := &fakeReads{groups: map[string]*domain.Group{"g1": {
		ID: "g1", BillingStatus: domain.BillingAllocated, PaymentStatus: domain.PaymentPaid,
	}}}
	svc := NewService(reads, &fakeAllocRepo{}, nil, nil, nil, billing.Config{})
	err := svc.ConfirmFree(context.Background(), "g1", "admin", 1)
	if err == nil {
		t.Fatal("expected error for non-free group")
	}
}

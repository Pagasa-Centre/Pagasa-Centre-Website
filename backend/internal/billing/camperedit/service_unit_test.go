package camperedit

import (
	"context"
	"errors"
	"testing"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/registration/domain"
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

type noopCamperRepo struct{}

func (noopCamperRepo) UpdateCamperDetailsMeta(context.Context, string, string, domain.CamperDetails, domain.ActionMeta, domain.BillingTransition) error {
	return nil
}
func (noopCamperRepo) AddCamperMeta(context.Context, string, domain.NewCamper, int, domain.ActionMeta, domain.BillingTransition) (string, error) {
	return "", nil
}
func (noopCamperRepo) SetMainContactMeta(context.Context, string, string, domain.ActionMeta) error {
	return nil
}
func (noopCamperRepo) WaiveCamperDepositMeta(context.Context, string, string, domain.ActionMeta) error {
	return nil
}
func (noopCamperRepo) DeleteCamperMeta(context.Context, string, string, domain.ActionMeta, domain.BillingTransition) error {
	return nil
}
func (noopCamperRepo) ConvertCamperToDayPassMeta(context.Context, string, string, domain.DayPassFields, int, domain.ActionMeta, domain.BillingTransition) error {
	return nil
}
func (noopCamperRepo) UpdateDayPassCamperMeta(context.Context, string, string, string, bool, *string, *string, domain.ActionMeta) error {
	return nil
}
func (noopCamperRepo) UpdateCamperCoachMeta(context.Context, string, string, bool, bool, domain.ActionMeta) error {
	return nil
}
func (noopCamperRepo) SetBillingCustomerRef(context.Context, string, string) error { return nil }

func TestRemoveCamper_blocksMainContact(t *testing.T) {
	reads := &fakeReads{
		group: &domain.Group{ID: "g1", BillingStatus: domain.BillingAllocated},
		campers: []domain.Camper{
			{ID: "c1", IsMainContact: true, FirstName: "Main", LastName: "Contact"},
			{ID: "c2", FirstName: "Other", LastName: "Camper"},
		},
	}
	svc := NewService(reads, noopCamperRepo{}, nil, nil, nil, billing.NewPricer(nil, billing.Config{}, nil), billing.Config{})
	_, err := svc.RemoveCamper(context.Background(), "g1", "c1", "admin", 1)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind != apperr.KindInvalid {
		t.Fatalf("expected invalid, got %v", err)
	}
}

type staleCamperRepo struct{ noopCamperRepo }

func (staleCamperRepo) DeleteCamperMeta(context.Context, string, string, domain.ActionMeta, domain.BillingTransition) error {
	return domain.ErrVersionConflict
}

func TestRemoveCamper_staleVersion(t *testing.T) {
	reads := &fakeReads{
		group: &domain.Group{ID: "g1", BillingStatus: domain.BillingAllocated, Version: 2},
		campers: []domain.Camper{
			{ID: "c1", FirstName: "A", LastName: "B"},
			{ID: "c2", FirstName: "C", LastName: "D"},
		},
	}
	svc := NewService(reads, staleCamperRepo{}, nil, nil, nil, billing.NewPricer(nil, billing.Config{}, nil), billing.Config{})
	_, err := svc.RemoveCamper(context.Background(), "g1", "c2", "admin", 1)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != "stale_state" {
		t.Fatalf("expected stale_state, got %v", err)
	}
}

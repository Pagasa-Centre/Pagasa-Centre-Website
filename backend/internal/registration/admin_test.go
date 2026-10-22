package registration

import (
	"context"
	"errors"
	"testing"

	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

type adminGroupRepo struct {
	groupRepo
	group            *domain.Group
	setPaymentErr    error
	setPaymentCalled bool
}

func (a *adminGroupRepo) FindGroupByID(_ context.Context, id string) (*domain.Group, error) {
	if a.group == nil || a.group.ID != id {
		return nil, nil
	}
	g := *a.group
	return &g, nil
}

func (a *adminGroupRepo) SetPaymentStatus(_ context.Context, _ string, _ domain.PaymentTransition) error {
	a.setPaymentCalled = true
	return a.setPaymentErr
}

func TestSetPaymentStatus_unknownGroup(t *testing.T) {
	svc := NewService(&adminGroupRepo{}, nil, nil, nil, nil, nil, nil, nil, "", Config{})
	err := svc.SetPaymentStatus(context.Background(), "missing", string(domain.PaymentPaid))
	if !errors.Is(err, domain.ErrGroupNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestSetPaymentStatus_invalidStatus(t *testing.T) {
	svc := NewService(&adminGroupRepo{
		group: &domain.Group{ID: "g1", PaymentStatus: domain.PaymentPending},
	}, nil, nil, nil, nil, nil, nil, nil, "", Config{})
	err := svc.SetPaymentStatus(context.Background(), "g1", "not-a-status")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind != apperr.KindInvalid {
		t.Fatalf("got %v", err)
	}
}

func TestSetPaymentStatus_staleConflict(t *testing.T) {
	repo := &adminGroupRepo{
		group:         &domain.Group{ID: "g1", PaymentStatus: domain.PaymentPending},
		setPaymentErr: domain.ErrVersionConflict,
	}
	svc := NewService(repo, nil, nil, nil, nil, nil, nil, nil, "", Config{})
	err := svc.SetPaymentStatus(context.Background(), "g1", string(domain.PaymentPaid))
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind != apperr.KindConflict || ae.Code != "stale_state" {
		t.Fatalf("got %v", err)
	}
}

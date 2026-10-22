package billing

import (
	"context"
	"errors"
	"strings"
	"testing"

	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

func TestSendInvoice_UnknownGroup(t *testing.T) {
	svc := NewService(&fakeRepo{}, nil, nil, nil, Config{})
	err := svc.SendInvoice(context.Background(), "missing", "admin", 1)
	if !errors.Is(err, domain.ErrGroupNotFound) {
		t.Fatalf("expected ErrGroupNotFound, got %v", err)
	}
}

func TestSendInvoice_DepositUnpaid(t *testing.T) {
	repo := &fakeRepo{groups: map[string]*domain.Group{
		"g1": {
			ID:            "g1",
			PaymentStatus: domain.PaymentPending,
			BillingStatus: domain.BillingNone,
		},
	}}
	svc := NewService(repo, nil, nil, nil, Config{})
	err := svc.SendInvoice(context.Background(), "g1", "admin", 1)
	if !errors.Is(err, domain.ErrDepositUnpaid) {
		t.Fatalf("expected ErrDepositUnpaid, got %v", err)
	}
}

func TestAllocate_VersionConflictMapsToStaleState(t *testing.T) {
	who := "Alice"
	repo := &fakeRepo{
		groups: map[string]*domain.Group{
			"g1": {
				ID:            "g1",
				PaymentStatus: domain.PaymentPaid,
				BillingStatus: domain.BillingNone,
				LastActionBy:  &who,
			},
		},
		campers: map[string][]domain.Camper{
			"g1": {{
				ID:             "c1",
				FirstName:      "Pat",
				LastName:       "Camper",
				Age:            25,
				AttendanceType: domain.AttendanceFullWeek,
			}},
		},
		allocateErr: domain.ErrVersionConflict,
	}
	svc := NewService(repo, nil, nil, nil, Config{})
	err := svc.Allocate(context.Background(), "g1", "admin", 1, AllocateRequest{
		Campers: []AllocateCamper{{
			CamperID:                   "c1",
			AllocatedAccommodationCode: "lodge",
			BilledStripePriceID:        "price_test",
		}},
	})
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("expected *apperr.Error, got %T %v", err, err)
	}
	if ae.Kind != apperr.KindConflict || ae.Code != "stale_state" {
		t.Fatalf("unexpected apperr: %+v", ae)
	}
	if ae.Message == "" || !strings.Contains(ae.Message, "Alice") {
		t.Fatalf("expected message to name Alice, got %q", ae.Message)
	}
}

func TestAllocate_RepositoryErrorWrapped(t *testing.T) {
	cause := errors.New("db down")
	repo := &fakeRepo{
		groups: map[string]*domain.Group{
			"g1": {
				ID:            "g1",
				PaymentStatus: domain.PaymentPaid,
				BillingStatus: domain.BillingNone,
			},
		},
		campers: map[string][]domain.Camper{
			"g1": {{
				ID:             "c1",
				FirstName:      "Pat",
				LastName:       "Camper",
				Age:            25,
				AttendanceType: domain.AttendanceFullWeek,
			}},
		},
		allocateErr: cause,
	}
	svc := NewService(repo, nil, nil, nil, Config{})
	err := svc.Allocate(context.Background(), "g1", "admin", 1, AllocateRequest{
		Campers: []AllocateCamper{{
			CamperID:                   "c1",
			AllocatedAccommodationCode: "lodge",
			BilledStripePriceID:        "price_test",
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

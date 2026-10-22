package invoicing

import (
	"context"
	"errors"
	"testing"
	"time"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

func TestSendCoachInvoice_guardOrder(t *testing.T) {
	g := &domain.Group{ID: "g1", IsFree: true, PaymentStatus: domain.PaymentPaid}
	reads := &fakeReads{groups: map[string]*domain.Group{"g1": g}}
	svc := NewService(reads, noopInvRepo{}, nil, nil, nil, billing.NewPricer(nil, billing.Config{}, nil), billing.Config{})
	err := svc.SendCoachInvoice(context.Background(), "g1", "admin", 1)
	assertSameInvalid(t, err, domain.ErrSponsoredNoCoach)

	waived := time.Now()
	g = &domain.Group{ID: "g1", CoachFeeWaivedAt: &waived, PaymentStatus: domain.PaymentPaid, BillingStatus: domain.BillingInvoiced}
	reads.groups["g1"] = g
	err = svc.SendCoachInvoice(context.Background(), "g1", "admin", 1)
	assertMsg(t, err, "coach fee has been waived for this group")

	g = &domain.Group{ID: "g1", PaymentStatus: domain.PaymentPaid, BillingStatus: domain.BillingAllocated}
	reads.groups["g1"] = g
	err = svc.SendCoachInvoice(context.Background(), "g1", "admin", 1)
	assertSameInvalid(t, err, domain.ErrCoachInBalance)
}

func TestResendInvoice_noInvoiceOnFile(t *testing.T) {
	g := &domain.Group{ID: "g1", PaymentStatus: domain.PaymentPaid, BillingStatus: domain.BillingInvoiced}
	reads := &fakeReads{groups: map[string]*domain.Group{"g1": g}}
	svc := NewService(reads, noopInvRepo{}, nil, nil, nil, billing.NewPricer(nil, billing.Config{}, nil), billing.Config{})
	err := svc.ResendInvoice(context.Background(), "g1", "admin", 1)
	assertMsg(t, err, "no invoice on file")
}

func TestWaiveCoachFee_guardOrder(t *testing.T) {
	reads := &fakeReads{groups: map[string]*domain.Group{"g1": {ID: "g1", IsFree: true}}}
	svc := NewService(reads, noopInvRepo{}, nil, nil, nil, billing.NewPricer(nil, billing.Config{}, nil), billing.Config{})
	err := svc.WaiveCoachFee(context.Background(), "g1", "admin", 1)
	assertSameInvalid(t, err, domain.ErrSponsoredNoCoach)

	g := &domain.Group{ID: "g1", CoachIncludedInBalance: true}
	reads.groups["g1"] = g
	err = svc.WaiveCoachFee(context.Background(), "g1", "admin", 1)
	assertSameInvalid(t, err, domain.ErrCoachInBalance)
}

func TestUnwaiveCoachFee_notWaived(t *testing.T) {
	reads := &fakeReads{groups: map[string]*domain.Group{"g1": {ID: "g1"}}}
	svc := NewService(reads, noopInvRepo{}, nil, nil, nil, billing.NewPricer(nil, billing.Config{}, nil), billing.Config{})
	err := svc.UnwaiveCoachFee(context.Background(), "g1", "admin", 1)
	assertMsg(t, err, "coach fee is not waived for this group")
}

func TestSendInvoice_nothingToInvoice(t *testing.T) {
	reads := &readsWithCampers{
		fakeReads: &fakeReads{groups: map[string]*domain.Group{"g1": {
			ID: "g1", PaymentStatus: domain.PaymentPaid, BillingStatus: domain.BillingAllocated,
		}}},
		campers: []domain.Camper{{AttendanceType: domain.AttendanceDayPass, DayPassDays: nil}},
	}
	svc := NewService(reads, noopInvRepo{}, nil, nil, nil, billing.NewPricer(nil, billing.Config{}, nil), billing.Config{})
	err := svc.SendInvoice(context.Background(), "g1", "admin", 1)
	assertMsg(t, err, "nothing to invoice")
}

func TestSendInvoice_camperNotAllocated(t *testing.T) {
	reads := &readsWithCampers{
		fakeReads: &fakeReads{groups: map[string]*domain.Group{"g1": {
			ID: "g1", PaymentStatus: domain.PaymentPaid, BillingStatus: domain.BillingAllocated,
		}}},
		campers: []domain.Camper{
			{FirstName: "Alex", LastName: "Camper", AttendanceType: domain.AttendanceFullWeek},
		},
	}
	svc := NewService(reads, noopInvRepo{}, nil, nil, nil, billing.NewPricer(nil, billing.Config{}, nil), billing.Config{})
	err := svc.SendInvoice(context.Background(), "g1", "admin", 1)
	assertMsg(t, err, "camper Alex Camper is not allocated")
}

func TestDeleteRegistration_prepaidMessage(t *testing.T) {
	g := &domain.Group{
		ID:                       "g1",
		PaidInFullAtRegistration: true,
		PaymentStatus:            domain.PaymentPaid,
	}
	reads := &fakeReads{groups: map[string]*domain.Group{"g1": g}}
	svc := NewService(reads, noopInvRepo{}, nil, nil, nil, billing.NewPricer(nil, billing.Config{}, nil), billing.Config{})
	_, err := svc.DeleteRegistration(context.Background(), "g1", "admin", 1)
	assertMsg(t, err, "this family paid for everything in one payment, so the deposit cannot be separated from the rest automatically. Refund what they are owed in Stripe first, then delete the registration.")
}

func assertSameInvalid(t *testing.T, err error, want *apperr.Error) {
	t.Helper()
	var de *apperr.Error
	if !errors.As(err, &de) || de.Message != want.Message {
		t.Fatalf("want %q, got %v", want.Message, err)
	}
}

func assertMsg(t *testing.T, err error, msg string) {
	t.Helper()
	var de *apperr.Error
	if !errors.As(err, &de) || de.Message != msg {
		t.Fatalf("message = %q, want %q", de.Message, msg)
	}
}

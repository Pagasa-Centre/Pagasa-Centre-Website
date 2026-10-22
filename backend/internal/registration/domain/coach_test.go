package domain

import (
	"errors"
	"testing"
	"time"

	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

func TestCoachEligibleCount(t *testing.T) {
	yes := true
	campers := []Camper{
		{AttendanceType: AttendanceFullWeek, NeedsCoach: &yes, Age: 10},
		{AttendanceType: AttendanceFullWeek, NeedsCoach: &yes, Age: 3},
		{AttendanceType: AttendanceDayPass, NeedsCoach: &yes, Age: 10},
	}
	if n := CoachEligibleCount(campers); n != 1 {
		t.Fatalf("count = %d, want 1", n)
	}
}

func TestCanSendCoachInvoice_order(t *testing.T) {
	waived := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	g := &Group{IsFree: true}
	assertSameInvalid(t, g.CanSendCoachInvoice(), ErrSponsoredNoCoach)

	g = &Group{CoachFeeWaivedAt: &waived}
	assertMsg(t, g.CanSendCoachInvoice(), "coach fee has been waived for this group")

	g = &Group{BillingStatus: BillingAllocated, PaymentStatus: PaymentPaid}
	assertSameInvalid(t, g.CanSendCoachInvoice(), ErrCoachInBalance)

	ref := "in_coach"
	g = &Group{BillingStatus: BillingInvoiced, PaymentStatus: PaymentPaid, CoachInvoiceRef: &ref}
	assertMsg(t, g.CanSendCoachInvoice(), "coach fee already charged")

	g = &Group{BillingStatus: BillingInvoiced, PaymentStatus: PaymentPaid}
	if err := g.CanSendCoachInvoice(); err != nil {
		t.Fatalf("want ok, got %v", err)
	}
}

func TestCanWaiveCoachFee_andUnwaive(t *testing.T) {
	g := &Group{IsFree: true}
	assertSameInvalid(t, g.CanWaiveCoachFee(), ErrSponsoredNoCoach)

	waived := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	g = &Group{CoachFeeWaivedAt: &waived}
	assertMsg(t, g.CanWaiveCoachFee(), "coach fee already waived")

	g = &Group{CoachIncludedInBalance: true, BillingStatus: BillingInvoiced}
	assertSameInvalid(t, g.CanWaiveCoachFee(), ErrCoachInBalance)

	g = &Group{}
	assertMsg(t, g.CanUnwaiveCoachFee(), "coach fee is not waived for this group")
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
		t.Fatalf("message = %q, want %q (%v)", de.Message, msg, err)
	}
}

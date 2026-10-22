package domain

import (
	"errors"
	"testing"
	"time"

	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

func assertInvalidMsg(t *testing.T, err error, want string) {
	t.Helper()
	var de *apperr.Error
	if !errors.As(err, &de) || de.Kind != apperr.KindInvalid {
		t.Fatalf("want invalid apperr, got %v", err)
	}
	if de.Message != want {
		t.Fatalf("message = %q, want %q", de.Message, want)
	}
}

func TestCanRemoveCamper_mainContact(t *testing.T) {
	g := &Group{}
	c := Camper{IsMainContact: true}
	assertInvalidMsg(t, g.CanRemoveCamper(c, []Camper{c}),
		"can't remove the main contact; edit the booking another way")
}

func TestCanRemoveCamper_lastCamper(t *testing.T) {
	g := &Group{}
	c := Camper{}
	assertInvalidMsg(t, g.CanRemoveCamper(c, []Camper{c}),
		"this is the last camper; delete the whole registration instead")
}

func TestCanRemoveCamper_balanceSettled(t *testing.T) {
	g := &Group{BillingStatus: BillingBalancePaid}
	c := Camper{}
	assertInvalidMsg(t, g.CanRemoveCamper(c, []Camper{c, {ID: "other"}}),
		"balance already paid; refund/handle in Stripe first")
}

func TestCoachChange_noOp(t *testing.T) {
	yes := true
	g := &Group{}
	c := Camper{AttendanceType: AttendanceFullWeek, NeedsCoach: &yes}
	noOp, err := g.CoachChange(c, true)
	if err != nil || !noOp {
		t.Fatalf("noOp=%v err=%v", noOp, err)
	}
}

func TestValidateAllocationChoice(t *testing.T) {
	assertInvalidMsg(t, ValidateAllocationChoice("", "unit1"),
		"cannot set a unit without an accommodation")
}

func TestFindCamper(t *testing.T) {
	c := Camper{ID: "c1", FirstName: "A", LastName: "B"}
	found, err := FindCamper([]Camper{c}, "c1")
	if err != nil || found == nil || found.ID != "c1" {
		t.Fatalf("FindCamper: %v err=%v", found, err)
	}
	if _, err := FindCamper([]Camper{c}, "missing"); err != ErrCamperNotFound {
		t.Fatalf("want ErrCamperNotFound, got %v", err)
	}
}

func TestCamperFullName(t *testing.T) {
	if got := (Camper{FirstName: " Pat ", LastName: " Lee "}).FullName(); got != "Pat   Lee" {
		t.Fatalf("FullName = %q", got)
	}
}

func TestHasUnpaidCoachInvoice(t *testing.T) {
	ref := "inv_1"
	g := Group{CoachInvoiceRef: &ref}
	if !g.HasUnpaidCoachInvoice() {
		t.Fatal("expected unpaid coach invoice")
	}
	paid := time.Now()
	g.CoachFeePaidAt = &paid
	if g.HasUnpaidCoachInvoice() {
		t.Fatal("expected paid coach invoice")
	}
}

func TestCanConvertToDayVisitor_notFullWeek(t *testing.T) {
	g := &Group{}
	c := Camper{AttendanceType: AttendanceDayPass}
	assertInvalidMsg(t, g.CanConvertToDayVisitor(c), "only full-week campers can be converted")
}

func TestCanConvertToDayVisitor_sponsored(t *testing.T) {
	g := &Group{IsFree: true}
	c := Camper{AttendanceType: AttendanceFullWeek}
	assertInvalidMsg(t, g.CanConvertToDayVisitor(c),
		"church-sponsored registrations can't be converted here; handle manually")
}

func TestCanEditDayPass_notDayPass(t *testing.T) {
	assertInvalidMsg(t, (Camper{AttendanceType: AttendanceFullWeek}).CanEditDayPass(),
		"only day-pass campers can be edited here")
}

func TestCoachChange_notFullWeek(t *testing.T) {
	g := &Group{}
	_, err := g.CoachChange(Camper{AttendanceType: AttendanceDayPass}, true)
	assertInvalidMsg(t, err, "only full-week campers can use the coach")
}

func TestCoachChange_feeAlreadyPaid(t *testing.T) {
	paid := time.Now()
	g := &Group{CoachFeePaidAt: &paid}
	yes := true
	_, err := g.CoachChange(Camper{AttendanceType: AttendanceFullWeek, NeedsCoach: &yes}, false)
	assertInvalidMsg(t, err, "coach fee already paid; handle any refund in Stripe first")
}

func TestCoachChange_includedInBalance(t *testing.T) {
	g := &Group{CoachIncludedInBalance: true}
	yes := true
	_, err := g.CoachChange(Camper{AttendanceType: AttendanceFullWeek, NeedsCoach: &yes}, false)
	assertInvalidMsg(t, err, "coach fee is included in the balance invoice for this group")
}

func TestCheckReprice_settled(t *testing.T) {
	g := &Group{BillingStatus: BillingBalancePaid}
	assertInvalidMsg(t, g.CheckReprice(true),
		"this balance is already paid, so changes that alter what they owe "+
			"have to be settled in Stripe; details that don't change the price "+
			"can still be edited")
}

func TestCanAddCamper_depositUnpaid(t *testing.T) {
	g := &Group{PaymentStatus: PaymentPending}
	assertInvalidMsg(t, g.CanAddCamper(),
		"this group has not paid their deposit yet; wait for it to clear before adding anyone")
}

func TestCanAddCamper_sponsored(t *testing.T) {
	g := &Group{PaymentStatus: PaymentPaid, IsFree: true}
	assertInvalidMsg(t, g.CanAddCamper(),
		"church-sponsored registrations are agreed as a whole; add this person as their own registration")
}

func TestCanAddCamper_paidInFullAtRegistration(t *testing.T) {
	g := &Group{PaymentStatus: PaymentPaid, PaidInFullAtRegistration: true}
	assertInvalidMsg(t, g.CanAddCamper(),
		"this family paid for camp in full at registration; add this person as their own registration")
}

func TestCanAddCamper_balanceSettled(t *testing.T) {
	g := &Group{PaymentStatus: PaymentPaid, BillingStatus: BillingBalancePaid}
	assertInvalidMsg(t, g.CanAddCamper(),
		"this balance is already paid, so there is no invoice left to charge them on; "+
			"add this person as their own registration")
}

func TestCanWaiveCamperDeposit_settled(t *testing.T) {
	g := &Group{BillingStatus: BillingBalancePaid}
	assertInvalidMsg(t, g.CanWaiveCamperDeposit(),
		"this balance is already paid; there is no invoice left to take it off")
}

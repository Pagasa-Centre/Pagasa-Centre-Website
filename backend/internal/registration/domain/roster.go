package domain

import (
	"strings"

	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

func FindCamper(campers []Camper, id string) (*Camper, error) {
	for i := range campers {
		if campers[i].ID == id {
			return &campers[i], nil
		}
	}
	return nil, ErrCamperNotFound
}

func (c Camper) FullName() string {
	return strings.TrimSpace(c.FirstName + " " + c.LastName)
}

func (g *Group) HasUnpaidCoachInvoice() bool {
	return g.CoachInvoiceRef != nil && strings.TrimSpace(*g.CoachInvoiceRef) != "" && g.CoachFeePaidAt == nil
}

func (g *Group) CanRemoveCamper(target Camper, campers []Camper) error {
	if target.IsMainContact {
		return apperr.Invalid("can't remove the main contact; edit the booking another way")
	}
	if len(campers) <= 1 {
		return apperr.Invalid("this is the last camper; delete the whole registration instead")
	}
	if g.BalanceSettled() {
		return apperr.Invalid("balance already paid; refund/handle in Stripe first")
	}
	return nil
}

func (g *Group) CanConvertToDayVisitor(target Camper) error {
	if target.AttendanceType != AttendanceFullWeek {
		return apperr.Invalid("only full-week campers can be converted")
	}
	if g.IsFree {
		return apperr.Invalid(
			"church-sponsored registrations can't be converted here; handle manually")
	}
	if g.BalanceSettled() {
		return apperr.Invalid("balance already paid; handle in Stripe first")
	}
	return nil
}

func (c Camper) CanEditDayPass() error {
	if c.AttendanceType != AttendanceDayPass {
		return apperr.Invalid("only day-pass campers can be edited here")
	}
	return nil
}

func (g *Group) CoachChange(target Camper, needsCoach bool) (noOp bool, err error) {
	if target.AttendanceType != AttendanceFullWeek {
		return false, apperr.Invalid("only full-week campers can use the coach")
	}
	current := target.NeedsCoach != nil && *target.NeedsCoach
	if current == needsCoach {
		return true, nil
	}
	if g.CoachFeePaidAt != nil ||
		(g.CoachIncludedInBalance && g.BillingStatus == BillingBalancePaid) {
		return false, apperr.Invalid(
			"coach fee already paid; handle any refund in Stripe first")
	}
	if g.CoachIncludedInBalance {
		return false, apperr.Invalid(
			"coach fee is included in the balance invoice for this group")
	}
	return false, nil
}

func ValidateAllocationChoice(accommodationCode, unitCode string) error {
	allocCode := strings.TrimSpace(accommodationCode)
	unit := strings.TrimSpace(unitCode)
	if allocCode == "" && unit != "" {
		return apperr.Invalid("cannot set a unit without an accommodation")
	}
	return nil
}

func (g *Group) CheckReprice(repriced bool) error {
	if repriced && g.BalanceSettled() {
		return apperr.Invalid(
			"this balance is already paid, so changes that alter what they owe " +
				"have to be settled in Stripe; details that don't change the price " +
				"can still be edited")
	}
	return nil
}

func (g *Group) CanAddCamper() error {
	if g.PaymentStatus != PaymentPaid {
		return apperr.Invalid(
			"this group has not paid their deposit yet; wait for it to clear before adding anyone")
	}
	if g.IsFree {
		return apperr.Invalid(
			"church-sponsored registrations are agreed as a whole; add this person as their own registration")
	}
	if g.PaidInFullAtRegistration {
		return apperr.Invalid(
			"this family paid for camp in full at registration; add this person as their own registration")
	}
	if g.BalanceSettled() {
		return apperr.Invalid(
			"this balance is already paid, so there is no invoice left to charge them on; " +
				"add this person as their own registration")
	}
	return nil
}

func (g *Group) CanWaiveCamperDeposit() error {
	if g.BalanceSettled() {
		return apperr.Invalid(
			"this balance is already paid; there is no invoice left to take it off")
	}
	return nil
}

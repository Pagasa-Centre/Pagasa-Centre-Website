package domain

import (
	"strings"

	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

func CoachEligibleCount(campers []Camper) int64 {
	var n int64
	for _, c := range campers {
		if c.AttendanceType != AttendanceFullWeek {
			continue
		}
		if c.CoachEligible() {
			n++
		}
	}
	return n
}

func (g *Group) CoachCountOnBalance(campers []Camper) int64 {
	if !g.CoachIncludedInBalance {
		return 0
	}
	return CoachEligibleCount(campers)
}

func (g *Group) CoachAlreadyCharged() bool {
	return g.CoachIncludedInBalance ||
		(g.CoachInvoiceRef != nil && strings.TrimSpace(*g.CoachInvoiceRef) != "")
}

func (g *Group) CanSendCoachInvoice() error {
	if g.IsFree {
		return ErrSponsoredNoCoach
	}
	if g.CoachFeeWaivedAt != nil {
		return apperr.Invalid("coach fee has been waived for this group")
	}
	if g.BillingStatus != BillingInvoiced && g.BillingStatus != BillingBalancePaid {
		return ErrCoachInBalance
	}
	if g.CoachAlreadyCharged() {
		return apperr.Invalid("coach fee already charged")
	}
	return nil
}

func (g *Group) CheckCoachInvoicePassengers(n int64) error {
	if n == 0 {
		return apperr.Invalid("no coach passengers to charge")
	}
	return nil
}

func (g *Group) CanWaiveCoachFee() error {
	if g.IsFree {
		return ErrSponsoredNoCoach
	}
	if g.CoachFeeWaivedAt != nil {
		return apperr.Invalid("coach fee already waived")
	}
	if g.CoachIncludedInBalance {
		return ErrCoachInBalance
	}
	if g.CoachFeePaidAt != nil {
		return apperr.Invalid("coach fee already paid")
	}
	return nil
}

func (g *Group) CheckCoachWaivePassengers(n int64) error {
	if n == 0 {
		return apperr.Invalid("no coach passengers to waive")
	}
	return nil
}

func (g *Group) CanUnwaiveCoachFee() error {
	if g.CoachFeeWaivedAt == nil {
		return apperr.Invalid("coach fee is not waived for this group")
	}
	return nil
}

func (g *Group) CoachFeeSettled() bool {
	return g.CoachFeePaidAt != nil
}

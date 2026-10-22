package domain

import (
	"strings"

	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

func (g *Group) ContactName() string {
	return strings.TrimSpace(g.ContactFirstName + " " + g.ContactLastName)
}

func (g *Group) SkipsBulkInvoice() bool {
	return g.IsFree || g.PaidInFullAtRegistration
}

func (g *Group) CanResendInvoice() (BillingTransition, error) {
	if g.BalanceInvoiceRef == nil || *g.BalanceInvoiceRef == "" {
		return BillingTransition{}, apperr.Invalid("no invoice on file")
	}
	return g.RequireOpenInvoice()
}

func (g *Group) CanDelete() error {
	if g.PaidInFullAtRegistration && g.PaymentStatus == PaymentPaid {
		return apperr.Invalid(
			"this family paid for everything in one payment, so the deposit cannot be " +
				"separated from the rest automatically. Refund what they are owed in " +
				"Stripe first, then delete the registration.")
	}
	return nil
}

func (g *Group) HasSettledBalanceInvoice() bool {
	return g.BalanceSettled() &&
		g.BalanceInvoiceRef != nil && strings.TrimSpace(*g.BalanceInvoiceRef) != ""
}

func DeleteRetention(g *Group, campers []Camper) (retainedPence, depositOnInvoicePence int64) {
	var depositOnInvoice int64
	for _, c := range campers {
		depositOnInvoice += int64(c.DepositPaidPence)
	}
	if g.PaymentStatus == PaymentPaid {
		retainedPence = int64(g.TotalAmountPence)
	}
	retainedPence += depositOnInvoice
	return retainedPence, depositOnInvoice
}

func (c Camper) AllocatedForInvoice() (code string, err error) {
	if c.AllocatedAccommodationCode == nil || strings.TrimSpace(*c.AllocatedAccommodationCode) == "" {
		return "", apperr.Invalidf("camper %s %s is not allocated", c.FirstName, c.LastName)
	}
	return strings.TrimSpace(*c.AllocatedAccommodationCode), nil
}

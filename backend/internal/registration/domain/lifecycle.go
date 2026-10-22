package domain

import (
	"strings"

	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

// BillingTransition is a status change the domain approved. Storage writes To
// only while the row is still in From. A zero value means no billing change;
// From == To means require From, change nothing on billing_status.
type BillingTransition struct {
	From, To BillingStatus
}

// ChangesBilling reports whether billing_status should be updated.
func (t BillingTransition) ChangesBilling() bool {
	return t.From != "" && t.To != t.From
}

// PaymentTransition is a payment_status compare-and-set write.
type PaymentTransition struct {
	From, To PaymentStatus
}

// Settlement is what a completed checkout or zero-total submit changes.
type Settlement struct {
	Payment PaymentTransition
	Billing BillingTransition
}

func (g *Group) transitionBillingStatusTo(to BillingStatus) BillingTransition {
	return BillingTransition{From: g.BillingStatus, To: to}
}

// Allocate validates and returns the billing transition for a new allocation.
func (g *Group) Allocate() (BillingTransition, error) {
	if g.PaymentStatus != PaymentPaid {
		return BillingTransition{}, apperr.Invalid("deposit must be paid before allocation")
	}
	if g.BillingStatus == BillingInvoiced {
		return BillingTransition{}, apperr.Invalid("cannot change allocation while invoice is open; release first")
	}
	if g.BillingStatus == BillingBalancePaid && !g.PaidInFullAtRegistration {
		return BillingTransition{}, ErrBalancePaid
	}
	if g.PaidInFullAtRegistration {
		return g.transitionBillingStatusTo(g.BillingStatus), nil
	}
	return g.transitionBillingStatusTo(BillingAllocated), nil
}

// Unallocate validates and returns the billing transition for clearing placements.
func (g *Group) Unallocate() (BillingTransition, error) {
	switch g.BillingStatus {
	case BillingInvoiced:
		return BillingTransition{}, ErrInvoiceOpen
	case BillingBalancePaid:
		if !g.PaidInFullAtRegistration {
			return BillingTransition{}, ErrBalancePaid
		}
	}
	if g.PaidInFullAtRegistration {
		return g.transitionBillingStatusTo(g.BillingStatus), nil
	}
	return g.transitionBillingStatusTo(BillingNone), nil
}

// Invoice validates and returns the transition to invoiced.
func (g *Group) Invoice(hasFullWeek bool) (BillingTransition, error) {
	if g.IsFree {
		return BillingTransition{}, apperr.Invalid("this is a church-sponsored registration; confirm the sponsorship instead of invoicing")
	}
	if g.PaidInFullAtRegistration {
		return BillingTransition{}, apperr.Invalid("this family paid in full at registration; there is nothing to invoice")
	}
	if g.PaymentStatus != PaymentPaid {
		return BillingTransition{}, ErrDepositUnpaid
	}
	switch g.BillingStatus {
	case BillingBalancePaid:
		return BillingTransition{}, ErrBalancePaid
	case BillingInvoiced:
		return BillingTransition{}, ErrInvoiceAlreadySent
	case BillingNone, BillingReleased:
		if hasFullWeek {
			return BillingTransition{}, apperr.Invalid("group must be allocated before invoicing")
		}
	}
	return g.transitionBillingStatusTo(BillingInvoiced), nil
}

// ConfirmSponsorship validates and returns the transition to free_confirmed.
func (g *Group) ConfirmSponsorship() (BillingTransition, error) {
	if !g.IsFree {
		return BillingTransition{}, apperr.Invalid("this group is not church-sponsored")
	}
	if g.PaymentStatus != PaymentPaid {
		return BillingTransition{}, ErrDepositUnpaid
	}
	if g.BillingStatus != BillingAllocated {
		return BillingTransition{}, apperr.Invalid("group must be allocated before confirming sponsorship")
	}
	return g.transitionBillingStatusTo(BillingFreeConfirmed), nil
}

// MarkBalancePaidManually validates and returns the transition to balance_paid.
func (g *Group) MarkBalancePaidManually() (BillingTransition, error) {
	if g.PaymentStatus != PaymentPaid {
		return BillingTransition{}, ErrDepositUnpaid
	}
	if g.IsFree {
		return BillingTransition{}, apperr.Invalid(
			"church-sponsored groups are settled with Confirm sponsorship")
	}
	switch g.BillingStatus {
	case BillingAllocated, BillingInvoiced:
	default:
		return BillingTransition{}, apperr.Invalid(
			"only allocated or invoiced groups can be marked paid; this group is " +
				g.BillingStatus.Label())
	}
	return g.transitionBillingStatusTo(BillingBalancePaid), nil
}

// Release returns the transition to released (permissive — any starting status).
func (g *Group) Release() BillingTransition {
	return g.transitionBillingStatusTo(BillingReleased)
}

// SettleBalance returns the transition for an invoice.paid webhook. already is
// true when the group was already balance_paid (idempotent no-op for storage).
func (g *Group) SettleBalance() (tr BillingTransition, already bool) {
	if g.BillingStatus == BillingBalancePaid {
		return BillingTransition{}, true
	}
	return g.transitionBillingStatusTo(BillingBalancePaid), false
}

// RequireOpenInvoice returns a CAS transition that requires invoiced status.
func (g *Group) RequireOpenInvoice() (BillingTransition, error) {
	if g.BillingStatus != BillingInvoiced {
		return BillingTransition{}, apperr.Invalid("invoice is not open")
	}
	return g.transitionBillingStatusTo(BillingInvoiced), nil
}

// HasOpenInvoice is true when the group has an open balance invoice in Stripe.
func (g *Group) HasOpenInvoice() bool {
	return g.BillingStatus == BillingInvoiced &&
		g.StripeInvoiceID != nil && strings.TrimSpace(*g.StripeInvoiceID) != ""
}

// BalanceSettled is true when the balance lifecycle is complete.
func (g *Group) BalanceSettled() bool {
	return g.BillingStatus == BillingBalancePaid
}

func allocatedOrNone(campers []Camper) BillingStatus {
	anyFullWeek := false
	for _, c := range campers {
		if c.AttendanceType != AttendanceFullWeek {
			continue
		}
		anyFullWeek = true
		if strVal(c.AllocatedAccommodationCode) == "" {
			return BillingNone
		}
	}
	if anyFullWeek {
		return BillingAllocated
	}
	return BillingNone
}

func hasUnplacedFullWeek(campers []Camper) bool {
	for _, c := range campers {
		if c.AttendanceType == AttendanceFullWeek && strVal(c.AllocatedAccommodationCode) == "" {
			return true
		}
	}
	return false
}

func strVal(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

// RosterChanged computes billing fallout after a roster edit. invoiceStale is
// true when the change voids a stale open invoice (repriced edit, remove, etc.).
func (g *Group) RosterChanged(after []Camper, invoiceStale bool) (tr BillingTransition, voidInvoice bool) {
	if g.HasOpenInvoice() && invoiceStale {
		voidInvoice = true
		to := allocatedOrNone(after)
		return g.transitionBillingStatusTo(to), voidInvoice
	}
	if g.BillingStatus == BillingAllocated && hasUnplacedFullWeek(after) {
		return g.transitionBillingStatusTo(BillingNone), false
	}
	return BillingTransition{}, false
}

// AllocationIsEdit reports whether an allocation PUT is editing vs first-time.
func (g *Group) AllocationIsEdit(campers []Camper) bool {
	if g.BillingStatus == BillingAllocated {
		return true
	}
	if !g.PaidInFullAtRegistration {
		return false
	}
	for _, c := range campers {
		if c.AttendanceType == AttendanceFullWeek &&
			strVal(c.AllocatedAccommodationCode) != "" {
			return true
		}
	}
	return false
}

// SettleCheckout returns the settlement for a pending checkout completion.
// ok is false when the group is not pending (idempotent webhook replay).
func (g *Group) SettleCheckout() (s Settlement, ok bool) {
	if g.PaymentStatus != PaymentPending {
		return Settlement{}, false
	}
	s.Payment = PaymentTransition{From: PaymentPending, To: PaymentPaid}
	if g.PaidInFullAtRegistration {
		s.Billing = g.transitionBillingStatusTo(BillingBalancePaid)
	}
	return s, true
}

// ExpireCheckout returns pending → cancelled when the session expired.
func (g *Group) ExpireCheckout() (tr PaymentTransition, ok bool) {
	if g.PaymentStatus != PaymentPending {
		return PaymentTransition{}, false
	}
	return PaymentTransition{From: PaymentPending, To: PaymentCancelled}, true
}

// OverridePaymentStatus is the admin PATCH with no transition rules.
func (g *Group) OverridePaymentStatus(to PaymentStatus) PaymentTransition {
	return PaymentTransition{From: g.PaymentStatus, To: to}
}

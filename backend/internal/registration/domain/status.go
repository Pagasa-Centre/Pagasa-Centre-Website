package domain

type PaymentStatus string
type BillingStatus string

const (
	PaymentPending        PaymentStatus = "pending"
	PaymentPaid           PaymentStatus = "paid"
	PaymentFailed         PaymentStatus = "failed"
	PaymentFailedCapacity PaymentStatus = "failed_capacity"
	PaymentRefunded       PaymentStatus = "refunded"
	PaymentCancelled      PaymentStatus = "cancelled"

	BillingNone          BillingStatus = "none"
	BillingAllocated     BillingStatus = "allocated"
	BillingInvoiced      BillingStatus = "invoiced"
	BillingBalancePaid   BillingStatus = "balance_paid"
	BillingReleased      BillingStatus = "released"
	BillingFreeConfirmed BillingStatus = "free_confirmed"
)

// ParsePaymentStatus validates a wire payment_status value.
func ParsePaymentStatus(s string) (PaymentStatus, bool) {
	switch PaymentStatus(s) {
	case PaymentPending, PaymentPaid, PaymentFailed, PaymentFailedCapacity, PaymentRefunded, PaymentCancelled:
		return PaymentStatus(s), true
	default:
		return "", false
	}
}

// Label returns a human-readable billing status for admin messages.
func (b BillingStatus) Label() string {
	switch b {
	case BillingNone:
		return "Needs accommodation"
	case BillingAllocated:
		return "Ready to invoice"
	case BillingInvoiced:
		return "Awaiting payment"
	case BillingBalancePaid:
		return "Paid in full"
	case BillingFreeConfirmed:
		return "Sponsorship confirmed"
	case BillingReleased:
		return "Released"
	default:
		return string(b)
	}
}

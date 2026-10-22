package billing

import "time"

// InvoiceLine is one line on an invoice, billed either from a catalogued Stripe
// Price or as an ad-hoc amount. Ad-hoc lines exist for charges that have no
// standing Price of their own — a deposit owed by someone who joined after the
// group's deposit checkout was already paid. Quantity < 1 is treated as 1.
type InvoiceLine struct {
	PriceID  string
	Quantity int64

	// AmountPence and Description drive an ad-hoc line, used only when PriceID
	// is empty. Currency defaults to GBP when blank.
	AmountPence int64
	Description string
	Currency    string
}

func (l InvoiceLine) IsAdHoc() bool { return l.PriceID == "" && l.AmountPence > 0 }

// InvoiceResult is the outcome of creating/looking up a balance invoice.
type InvoiceResult struct {
	ID              string
	HostedURL       string // Stripe hosted invoice page (where the family pays)
	DueAt           time.Time
	AmountDuePence  int64
	Currency        string
	ProviderEmailed bool // true if Stripe successfully emailed the invoice
}

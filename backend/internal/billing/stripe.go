package billing

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/customer"
	"github.com/stripe/stripe-go/v79/invoice"
	"github.com/stripe/stripe-go/v79/invoiceitem"
)

// StripeBilling wraps Stripe Invoice operations.
type StripeBilling struct{}

func NewStripeBilling() *StripeBilling { return &StripeBilling{} }

// EnsureCustomer returns an existing or newly created Stripe Customer id.
func (s *StripeBilling) EnsureCustomer(ctx context.Context, existingID, email, name, groupID string) (string, error) {
	if existingID != "" {
		return existingID, nil
	}
	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Name:  stripe.String(name),
		Metadata: map[string]string{
			"group_id": groupID,
		},
	}
	params.Context = ctx
	c, err := customer.New(params)
	if err != nil {
		return "", fmt.Errorf("create stripe customer: %w", err)
	}
	return c.ID, nil
}

// InvoiceResult is the outcome of creating/looking up a balance invoice.
type InvoiceResult struct {
	ID             string
	HostedURL      string // Stripe hosted invoice page (where the family pays)
	DueAt          time.Time
	AmountDuePence int64
	Currency       string
	StripeEmailed  bool // true if Stripe successfully emailed the invoice
}

// CreateInvoice builds line items from Stripe Price ids, finalizes a
// send_invoice-collection invoice, and asks Stripe to email it. If Stripe's
// email send fails (e.g. the account's invoice-email capability is restricted),
// the invoice is still finalized and payable — StripeEmailed is returned false
// so the caller can fall back to emailing the hosted link itself.
func (s *StripeBilling) CreateInvoice(
	ctx context.Context,
	customerID string,
	groupID string,
	priceIDs []string,
	daysUntilDue int64,
) (InvoiceResult, error) {
	for _, priceID := range priceIDs {
		if priceID == "" {
			continue
		}
		itemParams := &stripe.InvoiceItemParams{
			Customer: stripe.String(customerID),
			Price:    stripe.String(priceID),
		}
		itemParams.Context = ctx
		if _, err := invoiceitem.New(itemParams); err != nil {
			return InvoiceResult{}, fmt.Errorf("create invoice item for price %s: %w", priceID, err)
		}
	}

	invParams := &stripe.InvoiceParams{
		Customer:         stripe.String(customerID),
		CollectionMethod: stripe.String(string(stripe.InvoiceCollectionMethodSendInvoice)),
		DaysUntilDue:     stripe.Int64(daysUntilDue),
		AutoAdvance:      stripe.Bool(true),
		// Stripe defaults pending_invoice_items_behavior to "exclude", which
		// would create an EMPTY (£0) invoice even though we just created the
		// line items above. We must explicitly pull them in, or the invoice is
		// £0, auto-finalizes, and is instantly marked paid.
		PendingInvoiceItemsBehavior: stripe.String("include"),
		Metadata: map[string]string{
			"group_id": groupID,
		},
	}
	invParams.Context = ctx
	inv, err := invoice.New(invParams)
	if err != nil {
		return InvoiceResult{}, fmt.Errorf("create invoice: %w", err)
	}

	finalParams := &stripe.InvoiceFinalizeInvoiceParams{}
	finalParams.Context = ctx
	inv, err = invoice.FinalizeInvoice(inv.ID, finalParams)
	if err != nil {
		return InvoiceResult{}, fmt.Errorf("finalize invoice: %w", err)
	}

	// Primary delivery: ask Stripe to email the invoice. Non-fatal on failure —
	// the caller falls back to our own email.
	stripeEmailed := true
	sendParams := &stripe.InvoiceSendInvoiceParams{}
	sendParams.Context = ctx
	if _, sendErr := invoice.SendInvoice(inv.ID, sendParams); sendErr != nil {
		stripeEmailed = false
		log.Printf("billing: Stripe could not email invoice %s; falling back to Resend: %v", inv.ID, sendErr)
	}

	due := time.Now().UTC().Add(time.Duration(daysUntilDue) * 24 * time.Hour)
	if inv.DueDate > 0 {
		due = time.Unix(inv.DueDate, 0).UTC()
	}
	return InvoiceResult{
		ID:             inv.ID,
		HostedURL:      inv.HostedInvoiceURL,
		DueAt:          due,
		AmountDuePence: inv.AmountDue,
		Currency:       string(inv.Currency),
		StripeEmailed:  stripeEmailed,
	}, nil
}

// SendInvoiceEmail asks Stripe to (re-)email an existing invoice. Returns an
// error if Stripe declines (caller may then fall back to its own email).
func (s *StripeBilling) SendInvoiceEmail(ctx context.Context, invoiceID string) error {
	params := &stripe.InvoiceSendInvoiceParams{}
	params.Context = ctx
	if _, err := invoice.SendInvoice(invoiceID, params); err != nil {
		return fmt.Errorf("stripe send invoice: %w", err)
	}
	return nil
}

// GetInvoice fetches an existing invoice (used to re-email its payment link).
func (s *StripeBilling) GetInvoice(ctx context.Context, invoiceID string) (InvoiceResult, error) {
	params := &stripe.InvoiceParams{}
	params.Context = ctx
	inv, err := invoice.Get(invoiceID, params)
	if err != nil {
		return InvoiceResult{}, fmt.Errorf("get invoice: %w", err)
	}
	due := time.Time{}
	if inv.DueDate > 0 {
		due = time.Unix(inv.DueDate, 0).UTC()
	}
	return InvoiceResult{
		ID:             inv.ID,
		HostedURL:      inv.HostedInvoiceURL,
		DueAt:          due,
		AmountDuePence: inv.AmountDue,
		Currency:       string(inv.Currency),
	}, nil
}

// VoidInvoice voids an open invoice in Stripe.
func (s *StripeBilling) VoidInvoice(ctx context.Context, invoiceID string) error {
	params := &stripe.InvoiceVoidInvoiceParams{}
	params.Context = ctx
	_, err := invoice.VoidInvoice(invoiceID, params)
	if err != nil {
		return fmt.Errorf("void invoice: %w", err)
	}
	return nil
}

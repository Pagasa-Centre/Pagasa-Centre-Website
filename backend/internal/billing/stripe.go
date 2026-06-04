package billing

import (
	"context"
	"fmt"
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

// CreateAndSendInvoice builds line items from Stripe Price ids, creates an
// invoice with send_invoice collection, and returns the finalized invoice id
// and due time.
func (s *StripeBilling) CreateAndSendInvoice(
	ctx context.Context,
	customerID string,
	groupID string,
	priceIDs []string,
	daysUntilDue int64,
) (invoiceID string, dueAt time.Time, err error) {
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
			return "", time.Time{}, fmt.Errorf("create invoice item for price %s: %w", priceID, err)
		}
	}

	invParams := &stripe.InvoiceParams{
		Customer:         stripe.String(customerID),
		CollectionMethod: stripe.String(string(stripe.InvoiceCollectionMethodSendInvoice)),
		DaysUntilDue:     stripe.Int64(daysUntilDue),
		AutoAdvance:      stripe.Bool(true),
		Metadata: map[string]string{
			"group_id": groupID,
		},
	}
	invParams.Context = ctx
	inv, err := invoice.New(invParams)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create invoice: %w", err)
	}

	// Finalize and send (auto_advance may finalize async; explicit finalize+send is reliable).
	finalParams := &stripe.InvoiceFinalizeInvoiceParams{}
	finalParams.Context = ctx
	inv, err = invoice.FinalizeInvoice(inv.ID, finalParams)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("finalize invoice: %w", err)
	}

	sendParams := &stripe.InvoiceSendInvoiceParams{}
	sendParams.Context = ctx
	inv, err = invoice.SendInvoice(inv.ID, sendParams)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("send invoice: %w", err)
	}

	due := time.Now().UTC().Add(time.Duration(daysUntilDue) * 24 * time.Hour)
	if inv.DueDate > 0 {
		due = time.Unix(inv.DueDate, 0).UTC()
	}
	return inv.ID, due, nil
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

// ResendInvoice re-sends an existing invoice email.
func (s *StripeBilling) ResendInvoice(ctx context.Context, invoiceID string) error {
	params := &stripe.InvoiceSendInvoiceParams{}
	params.Context = ctx
	_, err := invoice.SendInvoice(invoiceID, params)
	if err != nil {
		return fmt.Errorf("resend invoice: %w", err)
	}
	return nil
}

package billing

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v85"
	"github.com/stripe/stripe-go/v85/customer"
	"github.com/stripe/stripe-go/v85/customerbalancetransaction"
	"github.com/stripe/stripe-go/v85/invoice"
	"github.com/stripe/stripe-go/v85/invoiceitem"
	"github.com/stripe/stripe-go/v85/invoicepayment"
	"github.com/stripe/stripe-go/v85/refund"
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

// InvoiceLine is one billed item: a Stripe Price and how many units of it.
// Quantity < 1 is treated as 1.
type InvoiceLine struct {
	PriceID  string
	Quantity int64
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
	lines []InvoiceLine,
	daysUntilDue int64,
	invoiceType string,
) (InvoiceResult, error) {
	// Create the draft invoice FIRST, then attach each line item directly to it
	// by invoice id. This is deliberately NOT the "create pending items, then
	// create invoice" pattern: Stripe defaults pending_invoice_items_behavior
	// to "exclude", which silently produced empty £0 invoices. Setting it to
	// "include" would instead sweep in any stale pending items left on the
	// customer from earlier failed attempts. Attaching by invoice id bills
	// exactly the items we intend — nothing more, nothing less.
	invParams := &stripe.InvoiceParams{
		Customer:         stripe.String(customerID),
		CollectionMethod: stripe.String(string(stripe.InvoiceCollectionMethodSendInvoice)),
		DaysUntilDue:     stripe.Int64(daysUntilDue),
		AutoAdvance:      stripe.Bool(true),
		Metadata: map[string]string{
			"group_id":     groupID,
			"invoice_type": invoiceType,
		},
	}
	invParams.Context = ctx
	inv, err := invoice.New(invParams)
	if err != nil {
		return InvoiceResult{}, fmt.Errorf("create invoice: %w", err)
	}

	for _, line := range lines {
		if line.PriceID == "" {
			continue
		}
		qty := line.Quantity
		if qty < 1 {
			qty = 1
		}
		itemParams := &stripe.InvoiceItemParams{
			Customer: stripe.String(customerID),
			Pricing: &stripe.InvoiceItemPricingParams{
				Price: stripe.String(line.PriceID),
			},
			Quantity: stripe.Int64(qty),
			Invoice:  stripe.String(inv.ID),
		}
		itemParams.Context = ctx
		if _, err := invoiceitem.New(itemParams); err != nil {
			return InvoiceResult{}, fmt.Errorf("create invoice item for price %s: %w", line.PriceID, err)
		}
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

// VoidInvoiceIdempotent voids an open invoice, treating an already-void (or
// otherwise non-open) invoice as success. Stripe has no dedicated
// "already void" error code, so we inspect the invoice status first. This lets
// a delete retry after a partial failure complete instead of aborting forever.
func (s *StripeBilling) VoidInvoiceIdempotent(ctx context.Context, invoiceID string) error {
	getParams := &stripe.InvoiceParams{}
	getParams.Context = ctx
	inv, err := invoice.Get(invoiceID, getParams)
	if err != nil {
		return fmt.Errorf("get invoice %s: %w", invoiceID, err)
	}
	if inv.Status != stripe.InvoiceStatusOpen {
		// Already void/paid/uncollectible/draft — nothing to void.
		return nil
	}
	return s.VoidInvoice(ctx, invoiceID)
}

// RefundPaymentIntent refunds a captured PaymentIntent. Returns the refunded
// amount in pence. Treats charge_already_refunded as success so a delete retry
// after a partial failure can still complete.
func (s *StripeBilling) RefundPaymentIntent(ctx context.Context, paymentIntentID string) (int64, error) {
	params := &stripe.RefundParams{PaymentIntent: stripe.String(paymentIntentID)}
	params.Context = ctx
	rf, err := refund.New(params)
	if err != nil {
		var se *stripe.Error
		if errors.As(err, &se) && se.Code == stripe.ErrorCodeChargeAlreadyRefunded {
			return 0, nil
		}
		return 0, fmt.Errorf("refund payment intent %s: %w", paymentIntentID, err)
	}
	return rf.Amount, nil
}

// RefundInvoice refunds the payment associated with a paid balance invoice.
// v85 invoices expose payments via InvoicePayment objects, not a top-level
// PaymentIntent field.
func (s *StripeBilling) RefundInvoice(ctx context.Context, invoiceID string) (int64, error) {
	listParams := &stripe.InvoicePaymentListParams{Invoice: stripe.String(invoiceID)}
	listParams.Context = ctx
	it := invoicepayment.List(listParams)
	for it.Next() {
		ip := it.InvoicePayment()
		if ip.Payment != nil && ip.Payment.PaymentIntent != nil {
			return s.RefundPaymentIntent(ctx, ip.Payment.PaymentIntent.ID)
		}
	}
	if err := it.Err(); err != nil {
		return 0, fmt.Errorf("list invoice payments %s: %w", invoiceID, err)
	}
	return 0, nil
}

// CreditCustomerBalance adds a credit to the Stripe customer's balance. amountPence
// is a positive credit amount; Stripe expects a negative Amount on the transaction.
func (s *StripeBilling) CreditCustomerBalance(
	ctx context.Context,
	customerID string,
	amountPence int64,
	currency, description, idempotencyKey string,
) error {
	if amountPence <= 0 {
		return fmt.Errorf("credit amount must be positive")
	}
	params := &stripe.CustomerBalanceTransactionParams{
		Customer:    stripe.String(customerID),
		Amount:      stripe.Int64(-amountPence),
		Currency:    stripe.String(strings.ToLower(currency)),
		Description: stripe.String(description),
	}
	params.Context = ctx
	if idempotencyKey != "" {
		params.SetIdempotencyKey(idempotencyKey)
	}
	if _, err := customerbalancetransaction.New(params); err != nil {
		return fmt.Errorf("credit customer balance: %w", err)
	}
	return nil
}

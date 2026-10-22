package stripe

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	stripesdk "github.com/stripe/stripe-go/v85"
	"github.com/stripe/stripe-go/v85/customer"
	"github.com/stripe/stripe-go/v85/customerbalancetransaction"
	"github.com/stripe/stripe-go/v85/invoice"
	"github.com/stripe/stripe-go/v85/invoiceitem"
	"github.com/stripe/stripe-go/v85/invoicepayment"
	"github.com/stripe/stripe-go/v85/refund"

	"pagasacentre/backend/internal/billing"
)

// BillingClient wraps Stripe Invoice operations.
type BillingClient struct{}

func NewBillingClient() *BillingClient { return &BillingClient{} }

// EnsureCustomer returns an existing or newly created Stripe Customer id.
func (s *BillingClient) EnsureCustomer(ctx context.Context, existingID, email, name, groupID string) (string, error) {
	if existingID != "" {
		return existingID, nil
	}
	params := &stripesdk.CustomerParams{
		Email: stripesdk.String(email),
		Name:  stripesdk.String(name),
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

func (s *BillingClient) CreateInvoice(
	ctx context.Context,
	customerID string,
	groupID string,
	lines []billing.InvoiceLine,
	daysUntilDue int64,
	invoiceType string,
) (billing.InvoiceResult, error) {
	invParams := &stripesdk.InvoiceParams{
		Customer:         stripesdk.String(customerID),
		CollectionMethod: stripesdk.String(string(stripesdk.InvoiceCollectionMethodSendInvoice)),
		DaysUntilDue:     stripesdk.Int64(daysUntilDue),
		AutoAdvance:      stripesdk.Bool(true),
		Metadata: map[string]string{
			"group_id":     groupID,
			"invoice_type": invoiceType,
		},
	}
	invParams.Context = ctx
	inv, err := invoice.New(invParams)
	if err != nil {
		return billing.InvoiceResult{}, fmt.Errorf("create invoice: %w", err)
	}

	for _, line := range lines {
		if line.PriceID == "" && !line.IsAdHoc() {
			continue
		}
		qty := line.Quantity
		if qty < 1 {
			qty = 1
		}
		itemParams := &stripesdk.InvoiceItemParams{
			Customer: stripesdk.String(customerID),
			Invoice:  stripesdk.String(inv.ID),
		}
		if line.IsAdHoc() {
			currency := line.Currency
			if currency == "" {
				currency = string(stripesdk.CurrencyGBP)
			}
			itemParams.Amount = stripesdk.Int64(line.AmountPence * qty)
			itemParams.Currency = stripesdk.String(strings.ToLower(currency))
			itemParams.Description = stripesdk.String(line.Description)
		} else {
			itemParams.Quantity = stripesdk.Int64(qty)
			itemParams.Pricing = &stripesdk.InvoiceItemPricingParams{
				Price: stripesdk.String(line.PriceID),
			}
		}
		itemParams.Context = ctx
		if _, err := invoiceitem.New(itemParams); err != nil {
			return billing.InvoiceResult{}, fmt.Errorf(
				"create invoice item (price %q, description %q): %w",
				line.PriceID, line.Description, err)
		}
	}

	finalParams := &stripesdk.InvoiceFinalizeInvoiceParams{}
	finalParams.Context = ctx
	inv, err = invoice.FinalizeInvoice(inv.ID, finalParams)
	if err != nil {
		return billing.InvoiceResult{}, fmt.Errorf("finalize invoice: %w", err)
	}

	stripeEmailed := true
	sendParams := &stripesdk.InvoiceSendInvoiceParams{}
	sendParams.Context = ctx
	if _, sendErr := invoice.SendInvoice(inv.ID, sendParams); sendErr != nil {
		stripeEmailed = false
		log.Printf("billing: Stripe could not email invoice %s; falling back to Resend: %v", inv.ID, sendErr)
	}

	due := time.Now().UTC().Add(time.Duration(daysUntilDue) * 24 * time.Hour)
	if inv.DueDate > 0 {
		due = time.Unix(inv.DueDate, 0).UTC()
	}
	return billing.InvoiceResult{
		ID:              inv.ID,
		HostedURL:       inv.HostedInvoiceURL,
		DueAt:           due,
		AmountDuePence:  inv.AmountDue,
		Currency:        string(inv.Currency),
		ProviderEmailed: stripeEmailed,
	}, nil
}

func (s *BillingClient) SendInvoiceEmail(ctx context.Context, invoiceID string) error {
	params := &stripesdk.InvoiceSendInvoiceParams{}
	params.Context = ctx
	if _, err := invoice.SendInvoice(invoiceID, params); err != nil {
		return fmt.Errorf("stripe send invoice: %w", err)
	}
	return nil
}

func (s *BillingClient) GetInvoice(ctx context.Context, invoiceID string) (billing.InvoiceResult, error) {
	params := &stripesdk.InvoiceParams{}
	params.Context = ctx
	inv, err := invoice.Get(invoiceID, params)
	if err != nil {
		return billing.InvoiceResult{}, fmt.Errorf("get invoice: %w", err)
	}
	due := time.Time{}
	if inv.DueDate > 0 {
		due = time.Unix(inv.DueDate, 0).UTC()
	}
	return billing.InvoiceResult{
		ID:             inv.ID,
		HostedURL:      inv.HostedInvoiceURL,
		DueAt:          due,
		AmountDuePence: inv.AmountDue,
		Currency:       string(inv.Currency),
	}, nil
}

func (s *BillingClient) VoidInvoice(ctx context.Context, invoiceID string) error {
	params := &stripesdk.InvoiceVoidInvoiceParams{}
	params.Context = ctx
	_, err := invoice.VoidInvoice(invoiceID, params)
	if err != nil {
		return fmt.Errorf("void invoice: %w", err)
	}
	return nil
}

func (s *BillingClient) VoidInvoiceIdempotent(ctx context.Context, invoiceID string) error {
	getParams := &stripesdk.InvoiceParams{}
	getParams.Context = ctx
	inv, err := invoice.Get(invoiceID, getParams)
	if err != nil {
		return fmt.Errorf("get invoice %s: %w", invoiceID, err)
	}
	if inv.Status != stripesdk.InvoiceStatusOpen {
		return nil
	}
	return s.VoidInvoice(ctx, invoiceID)
}

func (s *BillingClient) refundPaymentIntent(
	ctx context.Context, paymentIntentID string, amountPence int64,
) (int64, error) {
	params := &stripesdk.RefundParams{PaymentIntent: stripesdk.String(paymentIntentID)}
	if amountPence > 0 {
		params.Amount = stripesdk.Int64(amountPence)
	}
	params.Context = ctx
	rf, err := refund.New(params)
	if err != nil {
		var se *stripesdk.Error
		if errors.As(err, &se) && se.Code == stripesdk.ErrorCodeChargeAlreadyRefunded {
			return 0, nil
		}
		return 0, fmt.Errorf("refund payment intent %s: %w", paymentIntentID, err)
	}
	return rf.Amount, nil
}

func (s *BillingClient) RefundInvoiceKeeping(
	ctx context.Context, invoiceID string, keepPence int64,
) (int64, error) {
	listParams := &stripesdk.InvoicePaymentListParams{Invoice: stripesdk.String(invoiceID)}
	listParams.Context = ctx
	it := invoicepayment.List(listParams)
	for it.Next() {
		ip := it.InvoicePayment()
		if ip.Payment == nil || ip.Payment.PaymentIntent == nil {
			continue
		}
		if keepPence <= 0 {
			return s.refundPaymentIntent(ctx, ip.Payment.PaymentIntent.ID, 0)
		}
		refundable := ip.AmountPaid - keepPence
		if refundable <= 0 {
			return 0, nil
		}
		return s.refundPaymentIntent(ctx, ip.Payment.PaymentIntent.ID, refundable)
	}
	if err := it.Err(); err != nil {
		return 0, fmt.Errorf("list invoice payments %s: %w", invoiceID, err)
	}
	return 0, nil
}

func (s *BillingClient) CreditCustomerBalance(
	ctx context.Context,
	customerID string,
	amountPence int64,
	currency, description, idempotencyKey string,
) error {
	if amountPence <= 0 {
		return fmt.Errorf("credit amount must be positive")
	}
	params := &stripesdk.CustomerBalanceTransactionParams{
		Customer:    stripesdk.String(customerID),
		Amount:      stripesdk.Int64(-amountPence),
		Currency:    stripesdk.String(strings.ToLower(currency)),
		Description: stripesdk.String(description),
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

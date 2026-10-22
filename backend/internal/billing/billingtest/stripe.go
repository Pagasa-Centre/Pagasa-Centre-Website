package billingtest

import (
	"context"
	"errors"

	"pagasacentre/backend/internal/billing"
)

type StubStripe struct {
	RefundInv     func(ctx context.Context, id string, keepPence int64) (int64, error)
	RefundInvCall []RefundInvCall
	VoidInv       func(ctx context.Context, id string) error
	VoidInvIdem   func(ctx context.Context, id string) error
	GetInvoiceFn  func(ctx context.Context, id string) (billing.InvoiceResult, error)
	EnsureCust    func(ctx context.Context, existingID, email, name, groupID string) (string, error)
	CreateInv     func(ctx context.Context, customerID, groupID string, lines []billing.InvoiceLine, daysUntilDue int64, invoiceType string) (billing.InvoiceResult, error)
	CreditBalance func(ctx context.Context, customerID string, amountPence int64, currency, description, idempotencyKey string) error
	CreditCalls   []CreditCall
}

type CreditCall struct {
	CustomerID     string
	AmountPence    int64
	IdempotencyKey string
}

type RefundInvCall struct {
	InvoiceID string
	KeepPence int64
}

func (s *StubStripe) EnsureCustomer(ctx context.Context, existingID, email, name, groupID string) (string, error) {
	if s.EnsureCust != nil {
		return s.EnsureCust(ctx, existingID, email, name, groupID)
	}
	return "", errors.New("unexpected EnsureCustomer")
}

func (s *StubStripe) CreateInvoice(ctx context.Context, customerID, groupID string, lines []billing.InvoiceLine, daysUntilDue int64, invoiceType string) (billing.InvoiceResult, error) {
	if s.CreateInv != nil {
		return s.CreateInv(ctx, customerID, groupID, lines, daysUntilDue, invoiceType)
	}
	return billing.InvoiceResult{}, errors.New("unexpected CreateInvoice")
}

func (s *StubStripe) VoidInvoice(ctx context.Context, invoiceID string) error {
	if s.VoidInv != nil {
		return s.VoidInv(ctx, invoiceID)
	}
	return nil
}

func (s *StubStripe) VoidInvoiceIdempotent(ctx context.Context, invoiceID string) error {
	if s.VoidInvIdem != nil {
		return s.VoidInvIdem(ctx, invoiceID)
	}
	return nil
}

func (s *StubStripe) SendInvoiceEmail(context.Context, string) error {
	return errors.New("unexpected SendInvoiceEmail")
}

func (s *StubStripe) GetInvoice(ctx context.Context, invoiceID string) (billing.InvoiceResult, error) {
	if s.GetInvoiceFn != nil {
		return s.GetInvoiceFn(ctx, invoiceID)
	}
	return billing.InvoiceResult{}, errors.New("unexpected GetInvoice")
}

func (s *StubStripe) RefundInvoiceKeeping(ctx context.Context, id string, keepPence int64) (int64, error) {
	s.RefundInvCall = append(s.RefundInvCall, RefundInvCall{InvoiceID: id, KeepPence: keepPence})
	if s.RefundInv != nil {
		return s.RefundInv(ctx, id, keepPence)
	}
	return 0, nil
}

func (s *StubStripe) CreditCustomerBalance(
	ctx context.Context,
	customerID string,
	amountPence int64,
	currency, description, idempotencyKey string,
) error {
	s.CreditCalls = append(s.CreditCalls, CreditCall{
		CustomerID:     customerID,
		AmountPence:    amountPence,
		IdempotencyKey: idempotencyKey,
	})
	if s.CreditBalance != nil {
		return s.CreditBalance(ctx, customerID, amountPence, currency, description, idempotencyKey)
	}
	return nil
}

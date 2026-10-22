package invoicing

import (
	"context"
	"time"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/email"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/sheets"
)

type groupReader interface {
	FindGroupByID(ctx context.Context, id string) (*domain.Group, error)
	CampersForGroup(ctx context.Context, id string) ([]domain.Camper, error)
}

type invoicingRepo interface {
	SetInvoiceDetailsMeta(ctx context.Context, groupID, invoiceID string, dueAt time.Time, coachIncluded bool, meta domain.ActionMeta, tr domain.BillingTransition) error
	SetCoachInvoiceMeta(ctx context.Context, groupID, coachInvoiceID string, dueAt time.Time, meta domain.ActionMeta) error
	WaiveCoachFeeMeta(ctx context.Context, groupID string, meta domain.ActionMeta) error
	UnwaiveCoachFeeMeta(ctx context.Context, groupID string, meta domain.ActionMeta) error
	ExtendInvoiceDueAtMeta(ctx context.Context, groupID string, dueAt time.Time, meta domain.ActionMeta, tr domain.BillingTransition) error
	DeleteGroupMeta(ctx context.Context, groupID string, meta domain.ActionMeta) error
	MarkBalancePaidMeta(ctx context.Context, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error
	StampOnly(ctx context.Context, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error
	SetBillingCustomerRef(ctx context.Context, groupID, customerID string) error
}

type invoiceProvider interface {
	EnsureCustomer(ctx context.Context, existingID, email, name, groupID string) (string, error)
	CreateInvoice(ctx context.Context, customerID, groupID string, lines []billing.InvoiceLine, daysUntilDue int64, invoiceType string) (billing.InvoiceResult, error)
	VoidInvoiceIdempotent(ctx context.Context, invoiceID string) error
	SendInvoiceEmail(ctx context.Context, invoiceID string) error
	GetInvoice(ctx context.Context, invoiceID string) (billing.InvoiceResult, error)
	RefundInvoiceKeeping(ctx context.Context, invoiceID string, keepPence int64) (int64, error)
}

type mailer interface {
	SendBalanceInvoice(ctx context.Context, p email.BalanceInvoice) error
	SendCoachInvoice(ctx context.Context, p email.CoachInvoice) error
	SendBalancePaidConfirmation(ctx context.Context, p email.BalancePaidConfirmation) error
	SendBalancePaid(ctx context.Context, p email.BalancePaid) error
	SendAccommodationChanged(ctx context.Context, p email.AccommodationChangedNotice) error
}

type Service struct {
	reads    groupReader
	repo     invoicingRepo
	invoices invoiceProvider
	mailer   mailer
	sheets   sheets.Sync
	pricer   *billing.Pricer
	cfg      billing.Config
}

func NewService(reads groupReader, repo invoicingRepo, invoices invoiceProvider, mail mailer, sheetSync sheets.Sync, pricer *billing.Pricer, cfg billing.Config) *Service {
	if cfg.InvoiceDueDays <= 0 {
		cfg.InvoiceDueDays = 15
	}
	if sheetSync == nil {
		sheetSync = sheets.NewNoopSync()
	}
	return &Service{reads: reads, repo: repo, invoices: invoices, mailer: mail, sheets: sheetSync, pricer: pricer, cfg: cfg}
}

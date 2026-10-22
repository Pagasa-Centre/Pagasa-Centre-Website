package billingtest

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/billing/allocation"
	allocstorage "pagasacentre/backend/internal/billing/allocation/storage"
	"pagasacentre/backend/internal/billing/camperedit"
	camperstorage "pagasacentre/backend/internal/billing/camperedit/storage"
	"pagasacentre/backend/internal/billing/invoicing"
	invstorage "pagasacentre/backend/internal/billing/invoicing/storage"
	"pagasacentre/backend/internal/billing/reconcile"
	reconcilestorage "pagasacentre/backend/internal/billing/reconcile/storage"
	"pagasacentre/backend/internal/billing/sheetsync"
	sheetstorage "pagasacentre/backend/internal/billing/sheetsync/storage"
	"pagasacentre/backend/internal/billing/sweep"
	sweepstorage "pagasacentre/backend/internal/billing/sweep/storage"
	"pagasacentre/backend/internal/email"
	"pagasacentre/backend/internal/registration"
	regstorage "pagasacentre/backend/internal/registration/storage"
	"pagasacentre/backend/internal/sheets"
)

// Env wires all billing use-case services against one registration read repo.
type Env struct {
	Repo       *regstorage.Repository
	Pricer     *billing.Pricer
	Allocation *allocation.Service
	Invoicing  *invoicing.Service
	Campers    *camperedit.Service
	Reconcile  *reconcile.Service
	Sweep      *sweep.Service
	Sheets     *sheetsync.Service
}

func NewEnv(t *testing.T, pool *pgxpool.Pool, stripe invoicingStripe, mail email.Mailer, sheetSync sheets.Sync, cfg billing.Config) *Env {
	return newEnv(t, pool, stripe, mail, sheetSync, cfg, nil)
}

// NewEnvWithPrices is like NewEnv but wires a live deposit price lookup (late-arrival tests).
func NewEnvWithPrices(t *testing.T, pool *pgxpool.Pool, stripe invoicingStripe, mail email.Mailer, sheetSync sheets.Sync, cfg billing.Config, prices registration.PriceLookup) *Env {
	return newEnv(t, pool, stripe, mail, sheetSync, cfg, prices)
}

func newEnv(t *testing.T, pool *pgxpool.Pool, stripe invoicingStripe, mail email.Mailer, sheetSync sheets.Sync, cfg billing.Config, prices registration.PriceLookup) *Env {
	t.Helper()
	repo := regstorage.NewRepository(pool)
	pricer := billing.NewPricer(repo, cfg, prices)
	allocRepo := allocstorage.NewRepository(pool)
	invRepo := invstorage.NewRepository(pool)
	camperRepo := camperstorage.NewRepository(pool)
	reconRepo := reconcilestorage.NewRepository(pool)
	sweepRepo := sweepstorage.NewRepository(pool)
	sheetRepo := sheetstorage.NewRepository(pool)

	allocSvc := allocation.NewService(repo, allocRepo, stripe, mail, pricer, cfg)
	invSvc := invoicing.NewService(repo, invRepo, stripe, mail, sheetSync, pricer, cfg)
	camperSvc := camperedit.NewService(repo, camperRepo, stripe, mail, sheetSync, pricer, cfg)
	reconSvc := reconcile.NewService(repo, reconRepo, mail, pricer, cfg)
	sweepSvc := sweep.NewService(sweepRepo, allocSvc)
	sheetSvc := sheetsync.NewService(repo, sheetRepo, sheetSync)

	return &Env{
		Repo:       repo,
		Pricer:     pricer,
		Allocation: allocSvc,
		Invoicing:  invSvc,
		Campers:    camperSvc,
		Reconcile:  reconSvc,
		Sweep:      sweepSvc,
		Sheets:     sheetSvc,
	}
}

// invoicingStripe is satisfied by billingtest.StubStripe and *billing.StripeBilling.
type invoicingStripe interface {
	EnsureCustomer(ctx context.Context, existingID, email, name, groupID string) (string, error)
	CreateInvoice(ctx context.Context, customerID, groupID string, lines []billing.InvoiceLine, daysUntilDue int64, invoiceType string) (billing.InvoiceResult, error)
	VoidInvoice(ctx context.Context, invoiceID string) error
	VoidInvoiceIdempotent(ctx context.Context, invoiceID string) error
	SendInvoiceEmail(ctx context.Context, invoiceID string) error
	GetInvoice(ctx context.Context, invoiceID string) (billing.InvoiceResult, error)
	RefundInvoiceKeeping(ctx context.Context, invoiceID string, keepPence int64) (int64, error)
	CreditCustomerBalance(ctx context.Context, customerID string, amountPence int64, currency, description, idempotencyKey string) error
}

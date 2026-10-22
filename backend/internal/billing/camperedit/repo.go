package camperedit

import (
	"context"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/email"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/sheets"
)

type groupReader interface {
	FindGroupByID(ctx context.Context, id string) (*domain.Group, error)
	CampersForGroup(ctx context.Context, id string) ([]domain.Camper, error)
}

type camperEditRepo interface {
	UpdateCamperDetailsMeta(ctx context.Context, groupID, camperID string, d domain.CamperDetails, meta domain.ActionMeta, tr domain.BillingTransition) error
	AddCamperMeta(ctx context.Context, groupID string, c domain.NewCamper, depositOwedPence int, meta domain.ActionMeta, tr domain.BillingTransition) (string, error)
	SetMainContactMeta(ctx context.Context, groupID, camperID string, meta domain.ActionMeta) error
	WaiveCamperDepositMeta(ctx context.Context, groupID, camperID string, meta domain.ActionMeta) error
	DeleteCamperMeta(ctx context.Context, groupID, camperID string, meta domain.ActionMeta, tr domain.BillingTransition) error
	ConvertCamperToDayPassMeta(ctx context.Context, groupID, camperID string, dp domain.DayPassFields, depositCreditPence int, meta domain.ActionMeta, tr domain.BillingTransition) error
	UpdateDayPassCamperMeta(ctx context.Context, groupID, camperID string, tshirtOption string, needsCatering bool, shirtSize, dietary *string, meta domain.ActionMeta) error
	UpdateCamperCoachMeta(ctx context.Context, groupID, camperID string, needsCoach bool, clearCoachInvoice bool, meta domain.ActionMeta) error
	SetBillingCustomerRef(ctx context.Context, groupID, customerID string) error
}

type invoiceProvider interface {
	EnsureCustomer(ctx context.Context, existingID, email, name, groupID string) (string, error)
	VoidInvoiceIdempotent(ctx context.Context, invoiceID string) error
	CreditCustomerBalance(ctx context.Context, customerID string, amountPence int64, currency, description, idempotencyKey string) error
}

type mailer interface {
	SendBookingUpdated(ctx context.Context, p email.BookingUpdated) error
}

type Service struct {
	reads    groupReader
	repo     camperEditRepo
	invoices invoiceProvider
	mailer   mailer
	sheets   sheets.Sync
	pricer   *billing.Pricer
	cfg      billing.Config
}

func NewService(reads groupReader, repo camperEditRepo, invoices invoiceProvider, mail mailer, sheetSync sheets.Sync, pricer *billing.Pricer, cfg billing.Config) *Service {
	if sheetSync == nil {
		sheetSync = sheets.NewNoopSync()
	}
	return &Service{reads: reads, repo: repo, invoices: invoices, mailer: mail, sheets: sheetSync, pricer: pricer, cfg: cfg}
}

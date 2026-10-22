package allocation

import (
	"context"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/email"
	"pagasacentre/backend/internal/registration/domain"
)

type groupReader interface {
	FindGroupByID(ctx context.Context, id string) (*domain.Group, error)
	CampersForGroup(ctx context.Context, id string) ([]domain.Camper, error)
}

type allocationRepo interface {
	AllocateGroup(ctx context.Context, groupID string, allocs []domain.CamperAllocation, meta domain.ActionMeta, tr domain.BillingTransition) error
	UnallocateGroup(ctx context.Context, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error
	ConfirmFreeMeta(ctx context.Context, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error
	ClearInvoiceAndReleaseMeta(ctx context.Context, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error
}

type invoiceVoider interface {
	VoidInvoice(ctx context.Context, invoiceID string) error
}

type mailer interface {
	SendAllocationReleased(ctx context.Context, p email.AllocationReleased) error
	SendWhiteTeamNotification(ctx context.Context, p email.WhiteTeamNotification) error
	SendSponsorshipConfirmed(ctx context.Context, p email.SponsorshipConfirmed) error
}

type Service struct {
	reads    groupReader
	repo     allocationRepo
	invoices invoiceVoider
	mailer   mailer
	pricer   *billing.Pricer
	cfg      billing.Config
}

func NewService(reads groupReader, repo allocationRepo, invoices invoiceVoider, mail mailer, pricer *billing.Pricer, cfg billing.Config) *Service {
	return &Service{reads: reads, repo: repo, invoices: invoices, mailer: mail, pricer: pricer, cfg: cfg}
}

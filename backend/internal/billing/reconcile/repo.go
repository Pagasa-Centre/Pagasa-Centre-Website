package reconcile

import (
	"context"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/notify"
	"pagasacentre/backend/internal/registration/domain"
)

type groupReader interface {
	FindGroupByID(ctx context.Context, id string) (*domain.Group, error)
	CampersForGroup(ctx context.Context, id string) ([]domain.Camper, error)
}

type reconcileRepo interface {
	FindGroupByBalanceInvoiceRef(ctx context.Context, invoiceID string) (*domain.Group, error)
	FindGroupByCoachInvoiceRef(ctx context.Context, invoiceID string) (*domain.Group, error)
	MarkCoachFeePaidMeta(ctx context.Context, groupID string, meta domain.ActionMeta) error
	MarkBalancePaidMeta(ctx context.Context, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error
}

type mailer interface {
	SendBalancePaidConfirmation(ctx context.Context, p notify.BalancePaidConfirmation) error
	SendBalancePaid(ctx context.Context, p notify.BalancePaid) error
}

type Service struct {
	reads  groupReader
	repo   reconcileRepo
	mailer mailer
	pricer *billing.Pricer
	cfg    billing.Config
}

func NewService(reads groupReader, repo reconcileRepo, mail mailer, pricer *billing.Pricer, cfg billing.Config) *Service {
	return &Service{reads: reads, repo: repo, mailer: mail, pricer: pricer, cfg: cfg}
}

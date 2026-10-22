package payment

import (
	"context"

	"pagasacentre/backend/internal/registration/domain"
)

type txRunner interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type regRepo interface {
	GetGroupBySession(ctx context.Context, sessionID string) (*domain.Group, error)
	MarkPaid(ctx context.Context, groupID, paymentIntentID string) error
	MarkPaidInFull(ctx context.Context, groupID, paymentIntentID string, coachIncluded bool) error
	MarkStatus(ctx context.Context, groupID, status string) error
	CampersForGroup(ctx context.Context, groupID string) ([]domain.Camper, error)
	FindGroupByID(ctx context.Context, groupID string) (*domain.Group, error)
	UpdateContact(ctx context.Context, groupID, firstName, lastName, email, phone string) error
}

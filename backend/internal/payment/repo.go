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
	FindGroupByID(ctx context.Context, groupID string) (*domain.Group, error)
	CampersForGroup(ctx context.Context, groupID string) ([]domain.Camper, error)
	UpdateContact(ctx context.Context, groupID, firstName, lastName, email, phone string) error
	ApplySettlement(ctx context.Context, groupID, paymentIntentID string, coachIncluded bool, s domain.Settlement) error
	SetPaymentStatus(ctx context.Context, groupID string, tr domain.PaymentTransition) error
}

package registration

import (
	"context"

	"pagasacentre/backend/internal/registration/domain"
)

type txRunner interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type groupRepo interface {
	AccommodationAvailability(ctx context.Context) (map[string]bool, error)
	ReserveFreeCode(ctx context.Context, code string) (string, error)
	InsertGroup(ctx context.Context, req domain.Submission, totalPence int, currency string, isFree, paidInFull bool) (string, error)
	InsertCamper(ctx context.Context, groupID string, c domain.NewCamper) error
	MarkFreeCodeUsed(ctx context.Context, codeID, groupID string) error
	ApplySettlement(ctx context.Context, groupID, paymentIntentID string, coachIncluded bool, s domain.Settlement) error
	SetPaymentStatus(ctx context.Context, groupID string, tr domain.PaymentTransition) error
	SetStripeSession(ctx context.Context, groupID, sessionID string) error
	ListAccommodationTypes(ctx context.Context) ([]domain.AccommodationType, error)
	ListAccommodationUnits(ctx context.Context) ([]domain.AccommodationUnit, error)
	SetAccommodationAvailableForRegistration(ctx context.Context, code string, available bool) error
	FindGroupByID(ctx context.Context, groupID string) (*domain.Group, error)
	FindGroupBySessionID(ctx context.Context, sessionID string) (*domain.Group, error)
	CampersForGroup(ctx context.Context, groupID string) ([]domain.Camper, error)
	GenerateFreeCode(ctx context.Context, code, createdBy, note string) error
	ListFreeCodes(ctx context.Context) ([]domain.FreeCode, error)
	RevokeFreeCode(ctx context.Context, id string) error
	ListWithBilling(ctx context.Context, f domain.ListFilterBilling) ([]domain.Group, error)
	CampersForGroups(ctx context.Context, groupIDs []string) ([]domain.Camper, error)
}

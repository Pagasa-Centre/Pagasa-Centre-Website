package billing

import (
	"context"
	"time"

	"pagasacentre/backend/internal/registration/domain"
)

type billingRepo interface {
	FindGroupByID(ctx context.Context, groupID string) (*domain.Group, error)
	FindGroupByStripeInvoiceID(ctx context.Context, invoiceID string) (*domain.Group, error)
	FindGroupByStripeCoachInvoiceID(ctx context.Context, invoiceID string) (*domain.Group, error)
	CampersForGroup(ctx context.Context, groupID string) ([]domain.Camper, error)
	ListWithBilling(ctx context.Context, f domain.ListFilterBilling) ([]domain.Group, error)
	ListOverdueInvoiced(ctx context.Context, now time.Time) ([]domain.Group, error)
	GetAccommodationType(ctx context.Context, code string) (*domain.AccommodationType, error)
	GetAccommodationUnit(ctx context.Context, code string) (*domain.AccommodationUnit, error)
	ListAccommodationTypes(ctx context.Context) ([]domain.AccommodationType, error)
	ListAccommodationUnits(ctx context.Context) ([]domain.AccommodationUnit, error)
	SetStripeCustomerID(ctx context.Context, groupID, customerID string) error
	AllocateGroup(ctx context.Context, groupID string, allocs []domain.CamperAllocation, meta domain.ActionMeta, tr domain.BillingTransition) error
	UnallocateGroup(ctx context.Context, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error
	ConfirmFreeMeta(ctx context.Context, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error
	SetInvoiceDetailsMeta(ctx context.Context, groupID, invoiceID string, dueAt time.Time, coachIncluded bool, meta domain.ActionMeta, tr domain.BillingTransition) error
	SetCoachInvoiceMeta(ctx context.Context, groupID, coachInvoiceID string, dueAt time.Time, meta domain.ActionMeta) error
	MarkCoachFeePaidMeta(ctx context.Context, groupID string, meta domain.ActionMeta) error
	WaiveCoachFeeMeta(ctx context.Context, groupID string, meta domain.ActionMeta) error
	UnwaiveCoachFeeMeta(ctx context.Context, groupID string, meta domain.ActionMeta) error
	ClearInvoiceAndReleaseMeta(ctx context.Context, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error
	DeleteCamperMeta(ctx context.Context, groupID, camperID string, meta domain.ActionMeta, tr domain.BillingTransition) error
	ConvertCamperToDayPassMeta(ctx context.Context, groupID, camperID string, dp domain.DayPassFields, depositCreditPence int, meta domain.ActionMeta, tr domain.BillingTransition) error
	UpdateDayPassCamperMeta(ctx context.Context, groupID, camperID string, tshirtOption string, needsCatering bool, shirtSize, dietary *string, meta domain.ActionMeta) error
	UpdateCamperCoachMeta(ctx context.Context, groupID, camperID string, needsCoach bool, clearCoachInvoice bool, meta domain.ActionMeta) error
	DeleteGroupMeta(ctx context.Context, groupID string, meta domain.ActionMeta) error
	MarkBalancePaidMeta(ctx context.Context, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error
	ExtendInvoiceDueAtMeta(ctx context.Context, groupID string, dueAt time.Time, meta domain.ActionMeta, tr domain.BillingTransition) error
	StampOnly(ctx context.Context, groupID string, meta domain.ActionMeta, tr domain.BillingTransition) error
	UpdateCamperDetailsMeta(ctx context.Context, groupID, camperID string, d domain.CamperDetails, meta domain.ActionMeta, tr domain.BillingTransition) error
	AddCamperMeta(ctx context.Context, groupID string, c domain.CamperDTO, depositOwedPence int, meta domain.ActionMeta, tr domain.BillingTransition) (string, error)
	SetMainContactMeta(ctx context.Context, groupID, camperID string, meta domain.ActionMeta) error
	WaiveCamperDepositMeta(ctx context.Context, groupID, camperID string, meta domain.ActionMeta) error
}

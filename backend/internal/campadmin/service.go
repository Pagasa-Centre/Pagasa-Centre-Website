// Package campadmin implements admin use cases: domain service calls plus audit recording.
package campadmin

import (
	"context"
	"time"

	"pagasacentre/backend/internal/adminlog"
	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/registration/domain"
)

type billingActions interface {
	Allocate(ctx context.Context, groupID, actor string, expectedVersion int, req billing.AllocateRequest) error
	Unallocate(ctx context.Context, groupID, actor string, expectedVersion int) error
	SendInvoice(ctx context.Context, groupID, actor string, expectedVersion int) error
	SendInvoicesBulk(ctx context.Context, actor string, groupIDs []string) map[string]error
	SendCoachInvoice(ctx context.Context, groupID, actor string, expectedVersion int) error
	SendCoachInvoicesBulk(ctx context.Context, actor string, groupIDs []string) map[string]error
	WaiveCoachFee(ctx context.Context, groupID, actor string, expectedVersion int) error
	UnwaiveCoachFee(ctx context.Context, groupID, actor string, expectedVersion int) error
	MarkBalancePaidManually(ctx context.Context, groupID, actor string, expectedVersion int) error
	VoidAndRelease(ctx context.Context, groupID, reason, actor string, expectedVersion int, cancelled bool) error
	DeleteRegistration(ctx context.Context, groupID, actor string, expectedVersion int) (billing.DeleteSummary, error)
	RemoveCamper(ctx context.Context, groupID, camperID, actor string, expectedVersion int) (billing.RemoveCamperSummary, error)
	ConvertCamperToDayVisitor(ctx context.Context, groupID, camperID, actor string, expectedVersion int, req billing.ConvertToDayVisitorRequest) (billing.ConvertSummary, error)
	UpdateDayPassCamper(ctx context.Context, groupID, camperID, actor string, expectedVersion int, req billing.UpdateDayPassCamperRequest) (billing.UpdateDayPassSummary, error)
	UpdateCamperCoach(ctx context.Context, groupID, camperID, actor string, expectedVersion int, req billing.UpdateCamperCoachRequest) (billing.UpdateCamperCoachSummary, error)
	EditCamper(ctx context.Context, groupID, camperID, actor string, expectedVersion int, req billing.EditCamperRequest) (billing.EditCamperSummary, error)
	AddCamper(ctx context.Context, groupID, actor string, expectedVersion int, req billing.AddCamperRequest) (billing.AddCamperSummary, error)
	MakeMainContact(ctx context.Context, groupID, camperID, actor string, expectedVersion int) (string, error)
	WaiveCamperDeposit(ctx context.Context, groupID, camperID, actor string, expectedVersion int) (string, error)
	ResendInvoice(ctx context.Context, groupID, actor string, expectedVersion int) error
	ResyncAllSheets(ctx context.Context, actor string) (int, map[string]error)
	ExtendDueAt(ctx context.Context, groupID, actor string, expectedVersion int, dueAt time.Time) error
	SweepOverdue(ctx context.Context) (int, error)
	ConfirmFree(ctx context.Context, groupID, actor string, expectedVersion int) error
}

type registrationActions interface {
	FindGroup(ctx context.Context, id string) (*domain.Group, error)
	CampersForGroup(ctx context.Context, id string) ([]domain.Camper, error)
	SetPaymentStatus(ctx context.Context, groupID, status string) (domain.PaymentTransition, error)
	SetAccommodationAvailability(ctx context.Context, code string, available bool) error
	GenerateFreeCode(ctx context.Context, actor, note string) (string, error)
	RevokeFreeCode(ctx context.Context, id string) error
	ValidateFullPaymentPricing(ctx context.Context) error
}

type campConfig interface {
	SetRegistrationsOpen(ctx context.Context, open bool) error
	SetRegistrationPaymentMode(ctx context.Context, mode string) error
	UpdatePrice(ctx context.Context, code string, amountPence int) error
}

type contactUpdater interface {
	UpdateGroupContact(ctx context.Context, groupID, firstName, lastName, email, phone string, resend bool) error
}

type auditRecorder interface {
	Record(ctx context.Context, actor, action string, groupID *string, summary string, meta any) (adminlog.Event, error)
	List(ctx context.Context, limit int, beforeID int64) ([]adminlog.Event, error)
}

// Service coordinates admin actions and audit logging.
type Service struct {
	bill    billingActions
	reg     registrationActions
	camp    campConfig
	contact contactUpdater
	rec     auditRecorder
}

func NewService(bill billingActions, reg registrationActions, camp campConfig, contact contactUpdater, rec auditRecorder) *Service {
	return &Service{bill: bill, reg: reg, camp: camp, contact: contact, rec: rec}
}

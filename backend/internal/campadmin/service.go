// Package campadmin implements admin use cases: domain service calls plus audit recording.
package campadmin

import (
	"context"
	"time"

	"pagasacentre/backend/internal/adminlog"
	"pagasacentre/backend/internal/billing/allocation"
	"pagasacentre/backend/internal/billing/camperedit"
	"pagasacentre/backend/internal/billing/invoicing"
	"pagasacentre/backend/internal/registration/domain"
)

type allocationActions interface {
	Allocate(ctx context.Context, groupID, actor string, expectedVersion int, req allocation.AllocateRequest) error
	Unallocate(ctx context.Context, groupID, actor string, expectedVersion int) error
	VoidAndRelease(ctx context.Context, groupID, reason, actor string, expectedVersion int, cancelled bool) error
	ConfirmFree(ctx context.Context, groupID, actor string, expectedVersion int) error
}

type invoicingActions interface {
	SendInvoice(ctx context.Context, groupID, actor string, expectedVersion int) error
	SendInvoicesBulk(ctx context.Context, actor string, groupIDs []string) map[string]error
	SendCoachInvoice(ctx context.Context, groupID, actor string, expectedVersion int) error
	SendCoachInvoicesBulk(ctx context.Context, actor string, groupIDs []string) map[string]error
	WaiveCoachFee(ctx context.Context, groupID, actor string, expectedVersion int) error
	UnwaiveCoachFee(ctx context.Context, groupID, actor string, expectedVersion int) error
	MarkBalancePaidManually(ctx context.Context, groupID, actor string, expectedVersion int) error
	DeleteRegistration(ctx context.Context, groupID, actor string, expectedVersion int) (invoicing.DeleteSummary, error)
	ResendInvoice(ctx context.Context, groupID, actor string, expectedVersion int) error
	ExtendDueAt(ctx context.Context, groupID, actor string, expectedVersion int, dueAt time.Time) error
}

type camperActions interface {
	RemoveCamper(ctx context.Context, groupID, camperID, actor string, expectedVersion int) (camperedit.RemoveCamperSummary, error)
	ConvertCamperToDayVisitor(ctx context.Context, groupID, camperID, actor string, expectedVersion int, req camperedit.ConvertToDayVisitorRequest) (camperedit.ConvertSummary, error)
	UpdateDayPassCamper(ctx context.Context, groupID, camperID, actor string, expectedVersion int, req camperedit.UpdateDayPassCamperRequest) (camperedit.UpdateDayPassSummary, error)
	UpdateCamperCoach(ctx context.Context, groupID, camperID, actor string, expectedVersion int, req camperedit.UpdateCamperCoachRequest) (camperedit.UpdateCamperCoachSummary, error)
	EditCamper(ctx context.Context, groupID, camperID, actor string, expectedVersion int, req camperedit.EditCamperRequest) (camperedit.EditCamperSummary, error)
	AddCamper(ctx context.Context, groupID, actor string, expectedVersion int, req camperedit.AddCamperRequest) (camperedit.AddCamperSummary, error)
	MakeMainContact(ctx context.Context, groupID, camperID, actor string, expectedVersion int) (string, error)
	WaiveCamperDeposit(ctx context.Context, groupID, camperID, actor string, expectedVersion int) (string, error)
}

type sweeper interface {
	SweepOverdue(ctx context.Context) (int, error)
}

type sheetResyncer interface {
	ResyncAllSheets(ctx context.Context, actor string) (int, map[string]error)
}

type BillingServices struct {
	Allocation allocationActions
	Invoicing  invoicingActions
	Campers    camperActions
	Sweep      sweeper
	Sheets     sheetResyncer
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

type Service struct {
	bill    BillingServices
	reg     registrationActions
	camp    campConfig
	contact contactUpdater
	rec     auditRecorder
}

func NewService(bill BillingServices, reg registrationActions, camp campConfig, contact contactUpdater, rec auditRecorder) *Service {
	return &Service{bill: bill, reg: reg, camp: camp, contact: contact, rec: rec}
}

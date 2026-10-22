package campadmin

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"pagasacentre/backend/internal/adminlog"
	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

type recordCall struct {
	actor, action, summary string
	groupID                *string
	meta                   any
}

type captureRec struct {
	calls []recordCall
	err   error
}

func (c *captureRec) Record(_ context.Context, actor, action string, groupID *string, summary string, meta any) (adminlog.Event, error) {
	c.calls = append(c.calls, recordCall{actor, action, summary, groupID, meta})
	return adminlog.Event{}, c.err
}

func (c *captureRec) List(context.Context, int, int64) ([]adminlog.Event, error) {
	return nil, nil
}

type fakeReg struct {
	group    *domain.Group
	campers  []domain.Camper
	setTr    domain.PaymentTransition
	setErr   error
	priceErr error
}

func (f *fakeReg) FindGroup(_ context.Context, id string) (*domain.Group, error) {
	if f.group == nil || f.group.ID != id {
		return nil, nil
	}
	g := *f.group
	return &g, nil
}

func (f *fakeReg) CampersForGroup(context.Context, string) ([]domain.Camper, error) {
	return f.campers, nil
}

func (f *fakeReg) SetPaymentStatus(_ context.Context, _, _ string) (domain.PaymentTransition, error) {
	if f.setErr != nil {
		return domain.PaymentTransition{}, f.setErr
	}
	return f.setTr, nil
}

func (f *fakeReg) SetAccommodationAvailability(context.Context, string, bool) error { return nil }
func (f *fakeReg) GenerateFreeCode(context.Context, string, string) (string, error) { return "", nil }
func (f *fakeReg) RevokeFreeCode(context.Context, string) error                     { return nil }
func (f *fakeReg) ValidateFullPaymentPricing(context.Context) error                 { return f.priceErr }

type fakeBill struct {
	allocateErr error
	coachSum    billing.UpdateCamperCoachSummary
	coachErr    error
	bulkErrs    map[string]error
}

func (f *fakeBill) Allocate(context.Context, string, string, int, billing.AllocateRequest) error {
	return f.allocateErr
}
func (f *fakeBill) Unallocate(context.Context, string, string, int) error  { return nil }
func (f *fakeBill) SendInvoice(context.Context, string, string, int) error { return nil }
func (f *fakeBill) SendInvoicesBulk(_ context.Context, _ string, ids []string) map[string]error {
	if f.bulkErrs == nil {
		return map[string]error{}
	}
	return f.bulkErrs
}
func (f *fakeBill) SendCoachInvoice(context.Context, string, string, int) error { return nil }
func (f *fakeBill) SendCoachInvoicesBulk(context.Context, string, []string) map[string]error {
	return nil
}
func (f *fakeBill) WaiveCoachFee(context.Context, string, string, int) error   { return nil }
func (f *fakeBill) UnwaiveCoachFee(context.Context, string, string, int) error { return nil }
func (f *fakeBill) MarkBalancePaidManually(context.Context, string, string, int) error {
	return nil
}
func (f *fakeBill) VoidAndRelease(context.Context, string, string, string, int, bool) error {
	return nil
}
func (f *fakeBill) DeleteRegistration(context.Context, string, string, int) (billing.DeleteSummary, error) {
	return billing.DeleteSummary{}, nil
}
func (f *fakeBill) RemoveCamper(context.Context, string, string, string, int) (billing.RemoveCamperSummary, error) {
	return billing.RemoveCamperSummary{}, nil
}
func (f *fakeBill) ConvertCamperToDayVisitor(context.Context, string, string, string, int, billing.ConvertToDayVisitorRequest) (billing.ConvertSummary, error) {
	return billing.ConvertSummary{}, nil
}
func (f *fakeBill) UpdateDayPassCamper(context.Context, string, string, string, int, billing.UpdateDayPassCamperRequest) (billing.UpdateDayPassSummary, error) {
	return billing.UpdateDayPassSummary{}, nil
}
func (f *fakeBill) UpdateCamperCoach(context.Context, string, string, string, int, billing.UpdateCamperCoachRequest) (billing.UpdateCamperCoachSummary, error) {
	return f.coachSum, f.coachErr
}
func (f *fakeBill) EditCamper(context.Context, string, string, string, int, billing.EditCamperRequest) (billing.EditCamperSummary, error) {
	return billing.EditCamperSummary{}, nil
}
func (f *fakeBill) AddCamper(context.Context, string, string, int, billing.AddCamperRequest) (billing.AddCamperSummary, error) {
	return billing.AddCamperSummary{}, nil
}
func (f *fakeBill) MakeMainContact(context.Context, string, string, string, int) (string, error) {
	return "", nil
}
func (f *fakeBill) WaiveCamperDeposit(context.Context, string, string, string, int) (string, error) {
	return "", nil
}
func (f *fakeBill) ResendInvoice(context.Context, string, string, int) error { return nil }
func (f *fakeBill) ResyncAllSheets(context.Context, string) (int, map[string]error) {
	return 0, nil
}
func (f *fakeBill) ExtendDueAt(context.Context, string, string, int, time.Time) error { return nil }
func (f *fakeBill) SweepOverdue(context.Context) (int, error)                         { return 0, nil }
func (f *fakeBill) ConfirmFree(context.Context, string, string, int) error            { return nil }

type fakeCamp struct {
	modeErr error
	setMode bool
}

func (f *fakeCamp) SetRegistrationsOpen(context.Context, bool) error { return nil }
func (f *fakeCamp) SetRegistrationPaymentMode(context.Context, string) error {
	f.setMode = true
	return f.modeErr
}
func (f *fakeCamp) UpdatePrice(context.Context, string, int) error { return nil }

func lastCall(rec *captureRec) recordCall {
	if len(rec.calls) == 0 {
		return recordCall{}
	}
	return rec.calls[len(rec.calls)-1]
}

func TestAllocate_firstVsEdit(t *testing.T) {
	ctx := context.Background()
	rec := &captureRec{}
	gid := "g1"

	first := &fakeReg{
		group: &domain.Group{ID: gid, ContactFirstName: "Ann", ContactLastName: "Lee", BillingStatus: domain.BillingNone},
	}
	svc := NewService(&fakeBill{}, first, nil, nil, rec)
	if err := svc.Allocate(ctx, gid, "actor", 1, billing.AllocateRequest{}); err != nil {
		t.Fatal(err)
	}
	if c := lastCall(rec); c.action != adminlog.ActionAllocate {
		t.Fatalf("first allocate action = %q", c.action)
	}

	rec.calls = nil
	edit := &fakeReg{
		group:   &domain.Group{ID: gid, ContactFirstName: "Ann", ContactLastName: "Lee", BillingStatus: domain.BillingAllocated},
		campers: []domain.Camper{{GroupID: gid}},
	}
	svc = NewService(&fakeBill{}, edit, nil, nil, rec)
	if err := svc.Allocate(ctx, gid, "actor", 1, billing.AllocateRequest{}); err != nil {
		t.Fatal(err)
	}
	if c := lastCall(rec); c.action != adminlog.ActionAllocationEdited {
		t.Fatalf("edit allocate action = %q", c.action)
	}
}

var errAllocateFailed = errors.New("allocate failed")

func TestAllocate_serviceErrorNoAudit(t *testing.T) {
	rec := &captureRec{}
	reg := &fakeReg{group: &domain.Group{ID: "g1", BillingStatus: domain.BillingNone}}
	svc := NewService(&fakeBill{allocateErr: errAllocateFailed}, reg, nil, nil, rec)
	err := svc.Allocate(context.Background(), "g1", "actor", 1, billing.AllocateRequest{})
	if !errors.Is(err, errAllocateFailed) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	if len(rec.calls) != 0 {
		t.Fatalf("expected no audit, got %d", len(rec.calls))
	}
}

func TestSendInvoicesBulk_partialSuccess(t *testing.T) {
	rec := &captureRec{}
	reg := &fakeReg{group: &domain.Group{ID: "ok", ContactFirstName: "A", ContactLastName: "B"}}
	bill := &fakeBill{bulkErrs: map[string]error{"bad": errors.New("fail")}}
	svc := NewService(bill, reg, nil, nil, rec)
	errs := svc.SendInvoicesBulk(context.Background(), "actor", []string{"ok", "bad"})
	if len(errs) != 1 {
		t.Fatalf("errs = %v", errs)
	}
	if len(rec.calls) != 1 || rec.calls[0].action != adminlog.ActionInvoiceSent {
		t.Fatalf("calls = %+v", rec.calls)
	}
}

func TestUpdateCamperCoach_noOpNoAudit(t *testing.T) {
	rec := &captureRec{}
	svc := NewService(&fakeBill{coachSum: billing.UpdateCamperCoachSummary{NoOp: true}}, nil, nil, nil, rec)
	_, err := svc.UpdateCamperCoach(context.Background(), "g", "c", "actor", 1, billing.UpdateCamperCoachRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.calls) != 0 {
		t.Fatal("expected no audit on NoOp")
	}
}

func TestRelease_cancelled(t *testing.T) {
	rec := &captureRec{}
	reg := &fakeReg{group: &domain.Group{ID: "g1", ContactFirstName: "X", ContactLastName: "Y"}}
	svc := NewService(&fakeBill{}, reg, nil, nil, rec)
	if err := svc.Release(context.Background(), "g1", "actor", 1, true); err != nil {
		t.Fatal(err)
	}
	c := lastCall(rec)
	if c.action != adminlog.ActionCancel || !strings.Contains(c.summary, "Cancelled allocation") {
		t.Fatalf("got %+v", c)
	}
}

func TestSetRegistrationPaymentMode_pricingFirst(t *testing.T) {
	rec := &captureRec{}
	priceErr := apperr.Invalid("missing full price")
	reg := &fakeReg{priceErr: priceErr}
	camp := &fakeCamp{}
	svc := NewService(nil, reg, camp, nil, rec)
	err := svc.SetRegistrationPaymentMode(context.Background(), "actor", domain.PaymentModeFull)
	if !errors.Is(err, priceErr) {
		t.Fatalf("got %v", err)
	}
	if camp.setMode || len(rec.calls) != 0 {
		t.Fatal("should not set mode or audit when pricing fails")
	}
}

func TestSetRegistrationPaymentMode_setModeFailsNoAudit(t *testing.T) {
	rec := &captureRec{}
	camp := &fakeCamp{modeErr: errors.New("db")}
	svc := NewService(nil, &fakeReg{}, camp, nil, rec)
	err := svc.SetRegistrationPaymentMode(context.Background(), "actor", domain.PaymentModeDeposit)
	if err == nil {
		t.Fatal("expected error")
	}
	if len(rec.calls) != 0 {
		t.Fatal("no audit on set failure")
	}
}

func TestSetPaymentStatus_recordsTransition(t *testing.T) {
	rec := &captureRec{}
	reg := &fakeReg{
		group: &domain.Group{ID: "g1", ContactFirstName: "Sam", ContactLastName: "Kay"},
		setTr: domain.PaymentTransition{From: domain.PaymentPending, To: domain.PaymentPaid},
	}
	svc := NewService(nil, reg, nil, nil, rec)
	if err := svc.SetPaymentStatus(context.Background(), "g1", "actor", string(domain.PaymentPaid)); err != nil {
		t.Fatal(err)
	}
	c := lastCall(rec)
	if c.action != adminlog.ActionPaymentStatusUpdated || !strings.Contains(c.summary, "from pending to paid") {
		t.Fatalf("summary = %q action = %q", c.summary, c.action)
	}
}

func TestSetPaymentStatus_unchangedNoAudit(t *testing.T) {
	rec := &captureRec{}
	reg := &fakeReg{
		group: &domain.Group{ID: "g1"},
		setTr: domain.PaymentTransition{From: domain.PaymentPaid, To: domain.PaymentPaid},
	}
	svc := NewService(nil, reg, nil, nil, rec)
	if err := svc.SetPaymentStatus(context.Background(), "g1", "actor", string(domain.PaymentPaid)); err != nil {
		t.Fatal(err)
	}
	if len(rec.calls) != 0 {
		t.Fatal("expected no audit")
	}
}

func TestSetPaymentStatus_errorNoAudit(t *testing.T) {
	rec := &captureRec{}
	reg := &fakeReg{setErr: domain.ErrGroupNotFound}
	svc := NewService(nil, reg, nil, nil, rec)
	err := svc.SetPaymentStatus(context.Background(), "g1", "actor", string(domain.PaymentPaid))
	if !errors.Is(err, domain.ErrGroupNotFound) {
		t.Fatalf("got %v", err)
	}
	if len(rec.calls) != 0 {
		t.Fatal("expected no audit on error")
	}
}

func TestSetPaymentStatus_staleErrorNoAudit(t *testing.T) {
	rec := &captureRec{}
	stale := apperr.Conflict("stale_state", "stale")
	reg := &fakeReg{setErr: stale}
	svc := NewService(nil, reg, nil, nil, rec)
	var ae *apperr.Error
	err := svc.SetPaymentStatus(context.Background(), "g1", "actor", string(domain.PaymentPaid))
	if !errors.As(err, &ae) || ae.Code != "stale_state" {
		t.Fatalf("got %v", err)
	}
	if len(rec.calls) != 0 {
		t.Fatal("expected no audit on stale error")
	}
}

func TestRecorderErrorDoesNotFailAction(t *testing.T) {
	rec := &captureRec{err: errors.New("db down")}
	reg := &fakeReg{group: &domain.Group{ID: "g1", ContactFirstName: "A", ContactLastName: "B", BillingStatus: domain.BillingNone}}
	svc := NewService(&fakeBill{}, reg, nil, nil, rec)
	if err := svc.Allocate(context.Background(), "g1", "actor", 1, billing.AllocateRequest{}); err != nil {
		t.Fatal(err)
	}
}

func TestRecordLogin_nilSafe(t *testing.T) {
	var svc *Service
	svc.RecordLogin(context.Background(), "Pat")
	svc = NewService(nil, nil, nil, nil, nil)
	svc.RecordLogin(context.Background(), "Pat")

	var lr interface {
		RecordLogin(ctx context.Context, name string)
	} = (*Service)(nil)
	lr.RecordLogin(context.Background(), "Pat")
}

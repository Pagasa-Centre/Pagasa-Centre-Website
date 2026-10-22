package invoicing

import (
	"context"
	"errors"
	"testing"
	"time"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/notify"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

type fakeReads struct {
	groups map[string]*domain.Group
}

func (f *fakeReads) FindGroupByID(_ context.Context, id string) (*domain.Group, error) {
	if f.groups == nil {
		return nil, nil
	}
	g := f.groups[id]
	if g == nil {
		return nil, nil
	}
	cp := *g
	return &cp, nil
}

func (f *fakeReads) CampersForGroup(context.Context, string) ([]domain.Camper, error) {
	return nil, nil
}

type readsWithCampers struct {
	*fakeReads
	campers []domain.Camper
}

func (r *readsWithCampers) CampersForGroup(context.Context, string) ([]domain.Camper, error) {
	return r.campers, nil
}

type noopInvRepo struct{}

func (noopInvRepo) SetInvoiceDetailsMeta(context.Context, string, string, time.Time, bool, domain.ActionMeta, domain.BillingTransition) error {
	return nil
}
func (noopInvRepo) SetCoachInvoiceMeta(context.Context, string, string, time.Time, domain.ActionMeta) error {
	return nil
}
func (noopInvRepo) WaiveCoachFeeMeta(context.Context, string, domain.ActionMeta) error { return nil }
func (noopInvRepo) UnwaiveCoachFeeMeta(context.Context, string, domain.ActionMeta) error {
	return nil
}
func (noopInvRepo) ExtendInvoiceDueAtMeta(context.Context, string, time.Time, domain.ActionMeta, domain.BillingTransition) error {
	return nil
}
func (noopInvRepo) DeleteGroupMeta(context.Context, string, domain.ActionMeta) error { return nil }
func (noopInvRepo) MarkBalancePaidMeta(context.Context, string, domain.ActionMeta, domain.BillingTransition) error {
	return nil
}
func (noopInvRepo) StampOnly(context.Context, string, domain.ActionMeta, domain.BillingTransition) error {
	return nil
}
func (noopInvRepo) SetBillingCustomerRef(context.Context, string, string) error { return nil }

func TestSendInvoice_UnknownGroup(t *testing.T) {
	svc := NewService(&fakeReads{}, noopInvRepo{}, nil, nil, nil, billing.NewPricer(nil, billing.Config{}, nil), billing.Config{})
	err := svc.SendInvoice(context.Background(), "missing", "admin", 1)
	if !errors.Is(err, domain.ErrGroupNotFound) {
		t.Fatalf("expected ErrGroupNotFound, got %v", err)
	}
}

func TestSendInvoice_DepositUnpaid(t *testing.T) {
	reads := &fakeReads{groups: map[string]*domain.Group{
		"g1": {ID: "g1", PaymentStatus: domain.PaymentPending, BillingStatus: domain.BillingNone},
	}}
	svc := NewService(reads, noopInvRepo{}, nil, nil, nil, billing.NewPricer(nil, billing.Config{}, nil), billing.Config{})
	err := svc.SendInvoice(context.Background(), "g1", "admin", 1)
	if !errors.Is(err, domain.ErrDepositUnpaid) {
		t.Fatalf("expected ErrDepositUnpaid, got %v", err)
	}
}

func TestExtendDueAt_unknownGroup(t *testing.T) {
	svc := NewService(&fakeReads{}, noopInvRepo{}, nil, nil, nil, billing.NewPricer(nil, billing.Config{}, nil), billing.Config{})
	err := svc.ExtendDueAt(context.Background(), "missing", "admin", 1, time.Now())
	if !errors.Is(err, domain.ErrGroupNotFound) {
		t.Fatalf("expected ErrGroupNotFound, got %v", err)
	}
}

func TestExtendDueAt_notOpenInvoice(t *testing.T) {
	reads := &fakeReads{groups: map[string]*domain.Group{
		"g1": {ID: "g1", BillingStatus: domain.BillingAllocated, PaymentStatus: domain.PaymentPaid},
	}}
	svc := NewService(reads, noopInvRepo{}, nil, nil, nil, billing.NewPricer(nil, billing.Config{}, nil), billing.Config{})
	err := svc.ExtendDueAt(context.Background(), "g1", "admin", 1, time.Now())
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Kind != apperr.KindInvalid {
		t.Fatalf("expected invalid, got %v", err)
	}
	if ae.Message != "invoice is not open" {
		t.Fatalf("message = %q", ae.Message)
	}
}

func TestNewService_constructorDefaults(t *testing.T) {
	svc := NewService(&fakeReads{}, noopInvRepo{}, nil, nil, nil, billing.NewPricer(nil, billing.Config{}, nil), billing.Config{})
	if svc.cfg.InvoiceDueDays != 15 {
		t.Fatalf("InvoiceDueDays = %d, want 15", svc.cfg.InvoiceDueDays)
	}
	if svc.sheets == nil {
		t.Fatal("expected noop sheet sync default")
	}
	if _, ok := svc.sheets.(*notify.NoopSheetSync); !ok {
		t.Fatalf("sheets = %T, want *NoopSync", svc.sheets)
	}
}

func TestNewService_deleteUnpaidGroupSucceeds(t *testing.T) {
	reads := &fakeReads{groups: map[string]*domain.Group{
		"g1": {
			ID:            "g1",
			PaymentStatus: domain.PaymentPending,
			BillingStatus: domain.BillingNone,
			Currency:      "GBP",
		},
	}}
	svc := NewService(reads, noopInvRepo{}, &noopStripe{}, nil, nil, billing.NewPricer(nil, billing.Config{}, nil), billing.Config{InvoiceDueDays: 0})
	if _, err := svc.DeleteRegistration(context.Background(), "g1", "admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("DeleteRegistration: %v", err)
	}
}

func TestNewService_sendInvoiceUsesDefaultDueDays(t *testing.T) {
	lodge := "lodge"
	reads := &fakeReads{
		groups: map[string]*domain.Group{
			"g1": {
				ID: "g1", PaymentStatus: domain.PaymentPaid, BillingStatus: domain.BillingAllocated,
				BillingCustomerRef: strPtr("cus_x"), Currency: "GBP",
			},
		},
	}
	r := &readsWithCampers{
		fakeReads: reads,
		campers: []domain.Camper{{
			ID: "c1", FirstName: "Pat", LastName: "Camper", Age: 25,
			AttendanceType:             domain.AttendanceFullWeek,
			AllocatedAccommodationCode: &lodge,
		}},
	}
	var gotDue int64
	stub := &recordingStripe{
		createInv: func(_ context.Context, _, _ string, _ []billing.InvoiceLine, daysUntilDue int64, _ string) (billing.InvoiceResult, error) {
			gotDue = daysUntilDue
			return billing.InvoiceResult{ID: "inv_test", ProviderEmailed: true, DueAt: time.Now().UTC()}, nil
		},
	}
	pricer := billing.NewPricer(&fakeAccReader{priceID: "price_lodge"}, billing.Config{ChildUnder3PriceRef: "price_u3"}, nil)
	svc := NewService(r, noopInvRepo{}, stub, nil, nil, pricer, billing.Config{InvoiceDueDays: 0, DayPassPriceRef: "price_dp"})
	if err := svc.SendInvoice(context.Background(), "g1", "admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("SendInvoice: %v", err)
	}
	if gotDue != 15 {
		t.Fatalf("CreateInvoice daysUntilDue = %d, want 15", gotDue)
	}
}

func strPtr(s string) *string { return &s }

type noopStripe struct{}

func (noopStripe) EnsureCustomer(context.Context, string, string, string, string) (string, error) {
	return "", nil
}
func (noopStripe) CreateInvoice(context.Context, string, string, []billing.InvoiceLine, int64, string) (billing.InvoiceResult, error) {
	return billing.InvoiceResult{}, nil
}
func (noopStripe) VoidInvoice(context.Context, string) error           { return nil }
func (noopStripe) VoidInvoiceIdempotent(context.Context, string) error { return nil }
func (noopStripe) SendInvoiceEmail(context.Context, string) error      { return nil }
func (noopStripe) GetInvoice(context.Context, string) (billing.InvoiceResult, error) {
	return billing.InvoiceResult{}, nil
}
func (noopStripe) RefundInvoiceKeeping(context.Context, string, int64) (int64, error) {
	return 0, nil
}
func (noopStripe) CreditCustomerBalance(context.Context, string, int64, string, string, string) error {
	return nil
}

type recordingStripe struct {
	createInv func(context.Context, string, string, []billing.InvoiceLine, int64, string) (billing.InvoiceResult, error)
}

func (s *recordingStripe) EnsureCustomer(_ context.Context, existingID, _, _, _ string) (string, error) {
	if existingID != "" {
		return existingID, nil
	}
	return "cus_x", nil
}
func (s *recordingStripe) CreateInvoice(ctx context.Context, customerID, groupID string, lines []billing.InvoiceLine, days int64, invType string) (billing.InvoiceResult, error) {
	if s.createInv != nil {
		return s.createInv(ctx, customerID, groupID, lines, days, invType)
	}
	return billing.InvoiceResult{}, errors.New("unexpected CreateInvoice")
}
func (s *recordingStripe) VoidInvoice(context.Context, string) error           { return nil }
func (s *recordingStripe) VoidInvoiceIdempotent(context.Context, string) error { return nil }
func (s *recordingStripe) SendInvoiceEmail(context.Context, string) error      { return nil }
func (s *recordingStripe) GetInvoice(context.Context, string) (billing.InvoiceResult, error) {
	return billing.InvoiceResult{}, nil
}
func (s *recordingStripe) RefundInvoiceKeeping(context.Context, string, int64) (int64, error) {
	return 0, nil
}
func (s *recordingStripe) CreditCustomerBalance(context.Context, string, int64, string, string, string) error {
	return nil
}

type fakeAccReader struct {
	priceID string
}

func (f *fakeAccReader) GetAccommodationType(context.Context, string) (*domain.AccommodationType, error) {
	p := f.priceID
	return &domain.AccommodationType{Code: "lodge", PriceRef: &p}, nil
}
func (f *fakeAccReader) GetAccommodationUnit(context.Context, string) (*domain.AccommodationUnit, error) {
	return nil, nil
}
func (f *fakeAccReader) ListAccommodationTypes(context.Context) ([]domain.AccommodationType, error) {
	return nil, nil
}
func (f *fakeAccReader) ListAccommodationUnits(context.Context) ([]domain.AccommodationUnit, error) {
	return nil, nil
}

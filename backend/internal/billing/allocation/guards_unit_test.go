package allocation

import (
	"context"
	"errors"
	"testing"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/notify"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

func TestAllocate_guardMessages(t *testing.T) {
	reads := &fakeReads{
		groups: map[string]*domain.Group{"g1": {
			ID: "g1", PaymentStatus: domain.PaymentPaid, BillingStatus: domain.BillingNone,
		}},
		campers: map[string][]domain.Camper{"g1": {{
			ID: "c1", FirstName: "A", LastName: "B", Age: 25, AttendanceType: domain.AttendanceFullWeek,
		}}},
	}
	svc := NewService(reads, &fakeAllocRepo{}, nil, nil, nil, billing.Config{})

	err := svc.Allocate(context.Background(), "g1", "admin", 1, AllocateRequest{
		Campers: []AllocateCamper{{CamperID: "missing", AllocatedAccommodationCode: "tent"}},
	})
	assertInvalidMsg(t, err, "unknown camper_id missing")

	err = svc.Allocate(context.Background(), "g1", "admin", 1, AllocateRequest{
		Campers: []AllocateCamper{{CamperID: "c1", AllocatedAccommodationCode: " "}},
	})
	var ve *apperr.Error
	if !errors.As(err, &ve) || ve.Kind != apperr.KindValidation {
		t.Fatalf("want validation, got %v", err)
	}
	if ve.Fields["c1"] != "allocated_accommodation_code is required for full-week campers" {
		t.Fatalf("fields %+v", ve.Fields)
	}

	reads.campers["g1"] = []domain.Camper{
		{ID: "c1", FirstName: "A", LastName: "B", Age: 25, AttendanceType: domain.AttendanceFullWeek},
		{ID: "c2", FirstName: "C", LastName: "D", Age: 26, AttendanceType: domain.AttendanceFullWeek},
	}
	err = svc.Allocate(context.Background(), "g1", "admin", 1, AllocateRequest{
		Campers: []AllocateCamper{{CamperID: "c1", AllocatedAccommodationCode: "tent", BilledPriceRef: "price_x"}},
	})
	var ve2 *apperr.Error
	if !errors.As(err, &ve2) || ve2.Kind != apperr.KindValidation {
		t.Fatalf("want validation, got %v", err)
	}
	if ve2.Fields["campers"] != "full-week camper C D is not allocated" {
		t.Fatalf("fields %+v", ve2.Fields)
	}
}

type noopAllocMailer struct{}

func (noopAllocMailer) SendAllocationReleased(context.Context, notify.AllocationReleased) error {
	return nil
}
func (noopAllocMailer) SendStaffAllocationReleased(context.Context, notify.StaffAllocationReleased) error {
	return nil
}
func (noopAllocMailer) SendSponsorshipConfirmed(context.Context, notify.SponsorshipConfirmed) error {
	return nil
}

type voidFail struct{}

func (voidFail) VoidInvoice(context.Context, string) error { return errors.New("stripe down") }

type releaseRecorder struct {
	fakeAllocRepo
	cleared bool
}

func (r *releaseRecorder) ClearInvoiceAndReleaseMeta(context.Context, string, domain.ActionMeta, domain.BillingTransition) error {
	r.cleared = true
	return nil
}

func TestVoidAndRelease_voidFailureStillReleases(t *testing.T) {
	ref := "inv_1"
	reads := &fakeReads{
		groups: map[string]*domain.Group{"g1": {
			ID: "g1", PaymentStatus: domain.PaymentPaid, BillingStatus: domain.BillingInvoiced,
			BalanceInvoiceRef: &ref,
		}},
	}
	rec := &releaseRecorder{}
	svc := NewService(reads, rec, voidFail{}, noopAllocMailer{}, nil, billing.Config{})
	if err := svc.VoidAndRelease(context.Background(), "g1", "overdue", "admin", 1, false); err != nil {
		t.Fatal(err)
	}
	if !rec.cleared {
		t.Fatal("expected release to continue after void failure")
	}
}

func assertInvalidMsg(t *testing.T, err error, want string) {
	t.Helper()
	var de *apperr.Error
	if !errors.As(err, &de) || de.Message != want {
		t.Fatalf("message = %q, want %q (%v)", de.Message, want, err)
	}
}

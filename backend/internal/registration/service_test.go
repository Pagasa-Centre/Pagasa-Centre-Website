package registration

import (
	"context"
	"testing"

	"pagasacentre/backend/internal/registration/domain"
)

type fakePriceLookup struct{ amount int }

func (f fakePriceLookup) GetPrice(_ context.Context, code string) (PriceRow, error) {
	if code != domain.PriceDeposit {
		return PriceRow{}, nil
	}
	return PriceRow{AmountPence: f.amount, Currency: "GBP"}, nil
}

func TestDepositPayingCount(t *testing.T) {
	req := domain.SubmitRequest{Campers: []domain.CamperDTO{
		{Age: 30, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek}}, // pays
		{Age: 25, Attendance: domain.AttendanceDTO{Type: domain.AttendanceDayPass}},  // free (day pass)
		{Age: 10, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek}}, // pays
		{Age: 3, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek}},  // free (under 4)
		{Age: 4, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek}},  // pays (4 == threshold)
	}}
	if got := depositPayingCount(req); got != 3 {
		t.Errorf("depositPayingCount = %d, want 3", got)
	}
}

func TestComputeTotal_DepositPerPayingCamper(t *testing.T) {
	svc := &Service{prices: fakePriceLookup{amount: 5000}}
	req := domain.SubmitRequest{Campers: []domain.CamperDTO{
		{Age: 30, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek}}, // £50
		{Age: 30, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek}}, // £50
		{Age: 25, Attendance: domain.AttendanceDTO{Type: domain.AttendanceDayPass}},  // free
	}}
	total, currency, err := svc.computeTotal(context.Background(), req)
	if err != nil {
		t.Fatalf("computeTotal: %v", err)
	}
	if total != 10000 {
		t.Errorf("total = %d, want 10000", total)
	}
	if currency != "GBP" {
		t.Errorf("currency = %q, want GBP", currency)
	}
}

func TestComputeTotal_UnderThreesAreFree(t *testing.T) {
	svc := &Service{prices: fakePriceLookup{amount: 5000}}
	req := domain.SubmitRequest{Campers: []domain.CamperDTO{
		{Age: 30, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek}}, // £50
		{Age: 3, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek}},  // free (under 4)
		{Age: 1, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek}},  // free
	}}
	total, _, err := svc.computeTotal(context.Background(), req)
	if err != nil {
		t.Fatalf("computeTotal: %v", err)
	}
	if total != 5000 {
		t.Errorf("total = %d, want 5000 (only the adult pays)", total)
	}
}

func TestComputeTotal_DayPassOnlyIsZero(t *testing.T) {
	svc := &Service{prices: fakePriceLookup{amount: 5000}}
	req := domain.SubmitRequest{Campers: []domain.CamperDTO{
		{Age: 25, Attendance: domain.AttendanceDTO{Type: domain.AttendanceDayPass}},
		{Age: 25, Attendance: domain.AttendanceDTO{Type: domain.AttendanceDayPass}},
	}}
	total, _, err := svc.computeTotal(context.Background(), req)
	if err != nil {
		t.Fatalf("computeTotal: %v", err)
	}
	if total != 0 {
		t.Errorf("day-pass-only total = %d, want 0", total)
	}
}

func TestComputeTotal_AllUnderThreesIsZero(t *testing.T) {
	// Pathological but worth covering: a family registers only their two
	// toddlers full-week. Total should be £0 and the Submit path should
	// skip Stripe.
	svc := &Service{prices: fakePriceLookup{amount: 5000}}
	req := domain.SubmitRequest{Campers: []domain.CamperDTO{
		{Age: 1, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek}},
		{Age: 2, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek}},
	}}
	total, _, err := svc.computeTotal(context.Background(), req)
	if err != nil {
		t.Fatalf("computeTotal: %v", err)
	}
	if total != 0 {
		t.Errorf("all-toddlers total = %d, want 0", total)
	}
}

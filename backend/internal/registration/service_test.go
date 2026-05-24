package registration

import (
	"context"
	"testing"
)

type fakePriceLookup struct{ amount int }

func (f fakePriceLookup) GetPrice(_ context.Context, code string) (PriceRow, error) {
	if code != PriceDeposit {
		return PriceRow{}, nil
	}
	return PriceRow{AmountPence: f.amount, Currency: "GBP"}, nil
}

func TestFullWeekCount(t *testing.T) {
	req := SubmitRequest{Campers: []CamperDTO{
		{Attendance: AttendanceDTO{Type: AttendanceFullWeek}},
		{Attendance: AttendanceDTO{Type: AttendanceDayPass}},
		{Attendance: AttendanceDTO{Type: AttendanceFullWeek}},
	}}
	if got := fullWeekCount(req); got != 2 {
		t.Errorf("fullWeekCount = %d, want 2", got)
	}
}

func TestComputeTotal_DepositPerFullWeek(t *testing.T) {
	svc := &Service{prices: fakePriceLookup{amount: 5000}}
	req := SubmitRequest{Campers: []CamperDTO{
		{Attendance: AttendanceDTO{Type: AttendanceFullWeek}},
		{Attendance: AttendanceDTO{Type: AttendanceFullWeek}},
		{Attendance: AttendanceDTO{Type: AttendanceDayPass}},
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

func TestComputeTotal_DayPassOnlyIsZero(t *testing.T) {
	svc := &Service{prices: fakePriceLookup{amount: 5000}}
	req := SubmitRequest{Campers: []CamperDTO{
		{Attendance: AttendanceDTO{Type: AttendanceDayPass}},
		{Attendance: AttendanceDTO{Type: AttendanceDayPass}},
	}}
	total, _, err := svc.computeTotal(context.Background(), req)
	if err != nil {
		t.Fatalf("computeTotal: %v", err)
	}
	if total != 0 {
		t.Errorf("day-pass-only total = %d, want 0", total)
	}
}

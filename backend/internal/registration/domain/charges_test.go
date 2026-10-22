package domain

import (
	"strings"
	"testing"
)

func TestComputeCharge_depositMode(t *testing.T) {
	req := Submission{
		Campers: []NewCamper{
			{Attendance: Attendance{Type: AttendanceFullWeek}, Age: 10},
			{Attendance: Attendance{Type: AttendanceFullWeek}, Age: 3},
		},
	}
	ch, err := ComputeCharge(req, ChargeInputs{
		Mode:    PaymentModeDeposit,
		Deposit: PriceRow{AmountPence: 5000, Currency: "GBP"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ch.TotalPence != 5000 || ch.Currency != "GBP" || len(ch.Lines) != 1 {
		t.Fatalf("charge %+v", ch)
	}
}

func TestComputeCharge_fullModeNoCatalog(t *testing.T) {
	_, err := ComputeCharge(Submission{}, ChargeInputs{Mode: PaymentModeFull, HasCatalog: false}, nil)
	if err == nil || !strings.Contains(err.Error(), "stripe price catalog not configured") {
		t.Fatalf("got %v", err)
	}
}

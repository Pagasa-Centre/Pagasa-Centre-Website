package registration

import (
	"sort"
	"testing"

	"pagasacentre/backend/internal/registration/domain"
)

func TestComputeCharge_unit_depositAggregated(t *testing.T) {
	req := domain.Submission{Campers: []domain.NewCamper{
		{Age: 30, Attendance: domain.Attendance{Type: domain.AttendanceFullWeek}},
		{Age: 30, Attendance: domain.Attendance{Type: domain.AttendanceFullWeek}},
	}}
	ch, err := domain.ComputeCharge(req, domain.ChargeInputs{
		Mode:    domain.PaymentModeDeposit,
		Deposit: domain.PriceRow{AmountPence: 5000, Currency: "GBP"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ch.TotalPence != 10000 || len(ch.Lines) != 1 || ch.Lines[0].Quantity != 1 {
		t.Fatalf("charge %+v", ch)
	}
}

func TestComputeCharge_unit_tierCollapse(t *testing.T) {
	req := domain.Submission{Campers: []domain.NewCamper{
		{Age: 30, Attendance: domain.Attendance{Type: domain.AttendanceFullWeek, AccommodationFirstChoice: "lodge"}},
		{Age: 28, Attendance: domain.Attendance{Type: domain.AttendanceFullWeek, AccommodationFirstChoice: "lodge"}},
	}}
	lookup := func(priceRef string) (int64, string, error) {
		if priceRef == "price_lodge" {
			return 25000, "GBP", nil
		}
		return 0, "GBP", nil
	}
	ch, err := domain.ComputeCharge(req, domain.ChargeInputs{
		Mode:       domain.PaymentModeFull,
		Deposit:    domain.PriceRow{AmountPence: 5000, Currency: "GBP"},
		HasCatalog: true,
		Tiers:      []domain.AccommodationType{{Code: "lodge", DisplayName: "Lodge", PriceRef: strPtr("price_lodge")}},
	}, lookup)
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(ch.Lines, func(i, j int) bool { return ch.Lines[i].Description < ch.Lines[j].Description })
	var lodgeQty int
	for _, ln := range ch.Lines {
		if ln.AmountPence == 25000 && ln.Quantity == 2 {
			lodgeQty++
		}
	}
	if lodgeQty != 1 {
		t.Fatalf("lines %+v", ch.Lines)
	}
}

func strPtr(s string) *string { return &s }

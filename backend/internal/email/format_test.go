package email

import (
	"testing"
	"time"

	"pagasacentre/backend/internal/notify"
)

func TestFormatDate_nil(t *testing.T) {
	if formatDate(nil) != "" {
		t.Fatal("want empty")
	}
}

func TestFormatDate_zeroTime(t *testing.T) {
	zero := time.Time{}
	if got := formatDate(&zero); got != "1 Jan 0001" {
		t.Fatalf("got %q", got)
	}
}

func TestItemLine_matchesLegacyStrings(t *testing.T) {
	cases := []struct {
		item notify.InvoiceItem
		want string
	}{
		{notify.InvoiceItem{Kind: notify.ItemAccommodation, CamperName: "Josh Basco", AccommodationName: "Static Caravan", UnitName: "Caravan 5"},
			"Josh Basco — Static Caravan (Caravan 5)"},
		{notify.InvoiceItem{Kind: notify.ItemDayPass, CamperName: "Sam Visitor", Days: 2},
			"Sam Visitor — Day pass (2 days)"},
		{notify.InvoiceItem{Kind: notify.ItemCoach, Count: 2}, "Coach transport (×2)"},
	}
	for _, tc := range cases {
		if got := itemLine(tc.item); got != tc.want {
			t.Fatalf("itemLine = %q, want %q", got, tc.want)
		}
	}
}

func TestAmountLabel(t *testing.T) {
	if amountLabel(notify.Amount{Pence: 0, Currency: "GBP"}) != "" {
		t.Fatal("zero amount empty label")
	}
	if got := amountLabel(notify.Amount{Pence: 25000, Currency: "gbp"}); got != "£250.00" {
		t.Fatalf("got %q", got)
	}
}

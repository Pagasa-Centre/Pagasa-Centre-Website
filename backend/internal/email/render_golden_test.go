package email

import (
	"strings"
	"testing"
	"time"

	"pagasacentre/backend/internal/notify"
	"pagasacentre/backend/internal/testhelper"
)

func assertRenderGolden(t *testing.T, name string, subject, body string) {
	t.Helper()
	testhelper.AssertGolden(t, name+".golden", []byte(subject+"\n\n"+body))
}

func TestGolden_renderBalanceInvoice(t *testing.T) {
	due := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	subject, body, err := renderBalanceInvoice(notify.BalanceInvoice{
		ToEmail: "a@b.com", ToName: "Pat", PayURL: "https://pay.example/x",
		DueAt: &due, Amount: notify.Amount{Pence: 25000, Currency: "gbp"},
		Items: []notify.InvoiceItem{{
			Kind: notify.ItemAccommodation, CamperName: "Alex Camper",
			AccommodationName: "Tent", UnitName: "Field A",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRenderGolden(t, "balance_invoice", subject, body)
}

func TestGolden_renderBalanceInvoice_emptyDue(t *testing.T) {
	subject, body, err := renderBalanceInvoice(notify.BalanceInvoice{
		ToEmail: "a@b.com", ToName: "Pat", PayURL: "https://pay.example/x",
		DueAt: nil, Amount: notify.Amount{Pence: 100, Currency: "EUR"},
		Items: []notify.InvoiceItem{{Kind: notify.ItemNameOnly, CamperName: "Sam Lee"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRenderGolden(t, "balance_invoice_no_due", subject, body)
}

func TestGolden_renderStaffAllocationReleased(t *testing.T) {
	subject, body, err := renderStaffAllocationReleased(notify.StaffAllocationReleased{
		ToEmail: "staff@example.com", GroupID: "grp-1", ContactEmail: "fam@example.com",
		Reason: "overdue", CamperNames: []string{"Alex Camper"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "grp-1") {
		t.Fatal("expected group id in body")
	}
	assertRenderGolden(t, "staff_allocation_released", subject, body)
}

func TestGolden_renderDepositConfirmation(t *testing.T) {
	subject, body, err := renderDepositConfirmation(notify.DepositConfirmation{
		ToName: "Pat", AmountPence: 5000, Currency: "GBP", CamperCount: 2, HasMinor: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRenderGolden(t, "deposit_confirmation", subject, body)
}

func TestGolden_renderCoachInvoice_zeroDue(t *testing.T) {
	zero := time.Time{}
	subject, body, err := renderCoachInvoice(notify.CoachInvoice{
		ToName: "Pat", PayURL: "https://pay.example/coach", DueAt: &zero,
		Amount: notify.Amount{Pence: 4000, Currency: "GBP"}, PassengerCount: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRenderGolden(t, "coach_invoice_zero_due", subject, body)
}

func TestGolden_renderAllocationReleased_unpaid(t *testing.T) {
	subject, body, err := renderAllocationReleased(notify.AllocationReleased{
		ToName: "Pat", Reason: "overdue balance", CamperNames: []string{"Alex Camper"},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRenderGolden(t, "allocation_released_unpaid", subject, body)
}

func TestGolden_renderAllocationReleased_cancelled(t *testing.T) {
	subject, body, err := renderAllocationReleased(notify.AllocationReleased{
		ToName: "Pat", Cancelled: true, CamperNames: []string{"Alex Camper"},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRenderGolden(t, "allocation_released_cancelled", subject, body)
}

func TestGolden_renderAccommodationChanged_tentGuidance(t *testing.T) {
	subject, body, err := renderAccommodationChanged(notify.AccommodationChangedNotice{
		ToName: "Pat", AwaitingPayment: true,
		Items: []notify.AccommodationChange{{
			CamperName: "Alex", FirstChoice: "lodge", Allocated: "tent", TentGuidance: true,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRenderGolden(t, "accommodation_changed_tent", subject, body)
}

func TestGolden_renderAccommodationChanged_noTent(t *testing.T) {
	subject, body, err := renderAccommodationChanged(notify.AccommodationChangedNotice{
		ToName: "Pat",
		Items: []notify.AccommodationChange{{
			CamperName: "Alex", SecondChoice: "caravan", Allocated: "lodge",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRenderGolden(t, "accommodation_changed_no_tent", subject, body)
}

func TestGolden_renderBalanceInvoice_dayPassTwoDays(t *testing.T) {
	subject, body, err := renderBalanceInvoice(notify.BalanceInvoice{
		ToEmail: "a@b.com", ToName: "Pat", PayURL: "https://pay.example/x",
		Amount: notify.Amount{Pence: 3000, Currency: "GBP"},
		Items:  []notify.InvoiceItem{{Kind: notify.ItemDayPass, CamperName: "Sam", Days: 2}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRenderGolden(t, "balance_invoice_day_pass_2", subject, body)
}

func TestGolden_renderBalanceInvoice_accommodationNoUnit(t *testing.T) {
	subject, body, err := renderBalanceInvoice(notify.BalanceInvoice{
		ToEmail: "a@b.com", ToName: "Pat", PayURL: "https://pay.example/x",
		Amount: notify.Amount{Pence: 100, Currency: "GBP"},
		Items: []notify.InvoiceItem{{
			Kind: notify.ItemAccommodation, CamperName: "Alex", AccommodationName: "Tent",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRenderGolden(t, "balance_invoice_acc_no_unit", subject, body)
}

func TestGolden_renderBalanceInvoice_depositAndCoachLines(t *testing.T) {
	due := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	subject, body, err := renderBalanceInvoice(notify.BalanceInvoice{
		ToEmail: "a@b.com", ToName: "Pat", PayURL: "https://pay.example/x",
		DueAt: &due, Amount: notify.Amount{Pence: 0, Currency: "GBP"},
		Items: []notify.InvoiceItem{
			{Kind: notify.ItemDeposit, CamperName: "Alex Camper"},
			{Kind: notify.ItemCoach, Count: 2},
			{Kind: notify.ItemDayPass, CamperName: "Sam", Days: 1},
			{Kind: notify.ItemAccommodation, CamperName: "Alex", AccommodationName: "unknown_code", UnitName: ""},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRenderGolden(t, "balance_invoice_mixed_items", subject, body)
}

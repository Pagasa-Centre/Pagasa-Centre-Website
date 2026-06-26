package email

import (
	"strings"
	"testing"
)

func TestRenderDepositConfirmation_DepositPath(t *testing.T) {
	subject, body, err := renderDepositConfirmation(DepositConfirmation{
		ToEmail:        "parent@example.com",
		ToName:         "Sarah",
		AmountPence:    10000,
		Currency:       "GBP",
		CamperCount:    2,
		HasMinor:       true,
		ConsentFormURL: "https://example.com/api/consent-form",
	})
	if err != nil {
		t.Fatalf("renderDepositConfirmation: %v", err)
	}
	if !strings.Contains(subject, "deposit has been received") {
		t.Errorf("expected deposit subject, got %q", subject)
	}
	wantBody := []string{
		"Sarah",
		"£100.00",
		"2 campers",
		"Bro Ash",
		"Parental consent form",
		"https://example.com/api/consent-form",
		KeyDatesRegistration,
		KeyDatesAllocation,
		KeyDatesFinalPayment,
	}
	for _, want := range wantBody {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestRenderDepositConfirmation_DayPassOnlyPath(t *testing.T) {
	subject, body, err := renderDepositConfirmation(DepositConfirmation{
		ToEmail:     "visitor@example.com",
		ToName:      "Alex",
		AmountPence: 0,
		CamperCount: 1,
		HasMinor:    false,
	})
	if err != nil {
		t.Fatalf("renderDepositConfirmation: %v", err)
	}
	if !strings.Contains(subject, "registration is confirmed") {
		t.Errorf("expected day-pass subject, got %q", subject)
	}
	if strings.Contains(body, "deposit of") {
		t.Errorf("day-pass body should not mention deposit amount: %s", body)
	}
	if strings.Contains(body, "Parental consent form") {
		t.Errorf("body without minors should not show consent block")
	}
	if !strings.Contains(body, "1 camper") {
		t.Errorf("body should say '1 camper' (singular)")
	}
}

func TestRenderDepositConfirmation_SponsoredPath(t *testing.T) {
	subject, body, err := renderDepositConfirmation(DepositConfirmation{
		ToEmail:     "guest@example.com",
		ToName:      "Sam",
		AmountPence: 0,
		CamperCount: 1,
		IsFree:      true,
	})
	if err != nil {
		t.Fatalf("renderDepositConfirmation: %v", err)
	}
	if !strings.Contains(subject, "registration is confirmed") {
		t.Errorf("expected sponsored registration subject, got %q", subject)
	}
	if !strings.Contains(body, "fully sponsored by the church") {
		t.Errorf("sponsored registration body should mention church sponsorship")
	}
	if strings.Contains(body, "Day-pass attendance") {
		t.Errorf("sponsored registration body should not use day-pass wording")
	}
}

func TestRenderBalancePaidConfirmation(t *testing.T) {
	subject, body, err := renderBalancePaidConfirmation(BalancePaidConfirmation{
		ToEmail:     "family@example.com",
		ToName:      "Sam",
		AmountLabel: "£250.00",
		Items:       []string{"Josh Basco — Lodge"},
	})
	if err != nil {
		t.Fatalf("renderBalancePaidConfirmation: %v", err)
	}
	if !strings.Contains(subject, "place is confirmed") {
		t.Errorf("unexpected subject %q", subject)
	}
	if !strings.Contains(body, "£250.00") {
		t.Errorf("body should include the amount paid")
	}
	if !strings.Contains(body, "paid in full") {
		t.Errorf("body should state the balance is paid in full")
	}
	if !strings.Contains(body, "Josh Basco — Lodge") {
		t.Errorf("body should list the camper's accommodation")
	}
}

func TestRenderSponsorshipConfirmed(t *testing.T) {
	subject, body, err := renderSponsorshipConfirmed(SponsorshipConfirmed{
		ToEmail: "guest@example.com",
		ToName:  "Sam",
		Items:   []string{"Josh Basco — Lodge", "Mary Basco — Cabin (Cabin 2)"},
	})
	if err != nil {
		t.Fatalf("renderSponsorshipConfirmed: %v", err)
	}
	if !strings.Contains(subject, "place is confirmed") {
		t.Errorf("unexpected subject %q", subject)
	}
	if !strings.Contains(body, "sponsored by the church") {
		t.Errorf("body should mention church sponsorship")
	}
	if !strings.Contains(body, "Josh Basco — Lodge") ||
		!strings.Contains(body, "Mary Basco — Cabin (Cabin 2)") {
		t.Errorf("body should list each camper's accommodation")
	}
}

func TestRenderSponsorshipConfirmed_withOffPreferenceCallout(t *testing.T) {
	_, body, err := renderSponsorshipConfirmed(SponsorshipConfirmed{
		ToEmail: "guest@example.com",
		ToName:  "Sam",
		Items:   []string{"Alex Test — Tent"},
		Changes: []AccommodationChange{{
			CamperName:   "Alex Test",
			FirstChoice:  "Lodge",
			SecondChoice: "Cabin",
			Allocated:    "Tent",
			TentGuidance: true,
		}},
	})
	if err != nil {
		t.Fatalf("renderSponsorshipConfirmed: %v", err)
	}
	if !strings.Contains(body, "different from what you requested") {
		t.Errorf("body should include off-preference callout")
	}
	if !strings.Contains(body, "bring your own tent") {
		t.Errorf("body should mention tent guidance")
	}
}

func TestRenderAccommodationChanged(t *testing.T) {
	subject, body, err := renderAccommodationChanged(AccommodationChangedNotice{
		ToEmail: "family@example.com",
		ToName:  "Sam",
		Items: []AccommodationChange{{
			CamperName:   "Alex Test",
			FirstChoice:  "Lodge",
			SecondChoice: "Cabin",
			Allocated:    "Tent",
			TentGuidance: true,
		}},
		AwaitingPayment: true,
	})
	if err != nil {
		t.Fatalf("renderAccommodationChanged: %v", err)
	}
	if !strings.Contains(subject, "accommodation") {
		t.Errorf("unexpected subject %q", subject)
	}
	if !strings.Contains(body, "different from what you requested") {
		t.Errorf("body should apologise for the change")
	}
	if !strings.Contains(body, "Lodge (1st) / Cabin (2nd)") {
		t.Errorf("body should show both preferences")
	}
	if !strings.Contains(body, "bring your own tent") {
		t.Errorf("body should mention tent guidance")
	}
	if !strings.Contains(body, "Before you pay") {
		t.Errorf("body should warn before paying when AwaitingPayment")
	}

	_, bodyNoPay, err := renderAccommodationChanged(AccommodationChangedNotice{
		ToEmail: "family@example.com",
		ToName:  "Sam",
		Items: []AccommodationChange{{
			CamperName:  "Alex Test",
			FirstChoice: "Lodge",
			Allocated:   "Cabin",
		}},
		AwaitingPayment: false,
	})
	if err != nil {
		t.Fatalf("renderAccommodationChanged: %v", err)
	}
	if strings.Contains(bodyNoPay, "Before you pay") {
		t.Errorf("body should omit payment note when not awaiting payment")
	}
}

func TestFormatPence(t *testing.T) {
	cases := []struct {
		pence    int
		currency string
		want     string
	}{
		{10000, "GBP", "£100.00"},
		{0, "GBP", "£0.00"},
		{50, "GBP", "£0.50"},
		{5099, "GBP", "£50.99"},
		{12345, "USD", "USD 123.45"},
		{500, "", "£5.00"}, // empty currency defaults to GBP
	}
	for _, c := range cases {
		got := formatPence(c.pence, c.currency)
		if got != c.want {
			t.Errorf("formatPence(%d, %q) = %q, want %q", c.pence, c.currency, got, c.want)
		}
	}
}

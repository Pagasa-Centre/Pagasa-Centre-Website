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

func TestRenderDepositConfirmation_FreePlacePath(t *testing.T) {
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
		t.Errorf("expected free-place subject, got %q", subject)
	}
	if !strings.Contains(body, "fully covered by the church") {
		t.Errorf("free-place body should mention church coverage")
	}
	if strings.Contains(body, "Day-pass attendance") {
		t.Errorf("free-place body should not use day-pass wording")
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

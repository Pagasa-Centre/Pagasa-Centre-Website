package admin

import (
	"strings"
	"testing"

	"pagasacentre/backend/internal/billing"
)

// Deleting a registration is a hard delete, so this line is the only record left
// inside the app of money the church kept. Kept and refunded have to read as two
// separate amounts: rolled into one total, the line stops answering the question
// it exists for.
func TestFormatDeleteAuditSummary(t *testing.T) {
	cases := map[string]struct {
		sum         billing.DeleteSummary
		wantParts   []string
		unwantParts []string
	}{
		"deposit kept and balance refunded": {
			sum: billing.DeleteSummary{
				ContactName:   "Hilda Mensah",
				ContactEmail:  "hilda@example.com",
				RetainedPence: 15000,
				AmountPence:   12000,
			},
			wantParts: []string{
				"Hilda Mensah",
				"hilda@example.com",
				"kept the £150.00 non-refundable deposit",
				"refunded £120.00",
			},
			// The two figures must never be summed into one number.
			unwantParts: []string{"£270.00"},
		},
		"deposit kept with nothing to refund": {
			sum: billing.DeleteSummary{
				ContactName:   "Paid Deposit",
				ContactEmail:  "paid@example.com",
				RetainedPence: 5000,
			},
			wantParts:   []string{"kept the £50.00 non-refundable deposit"},
			unwantParts: []string{"refunded"},
		},
		"nothing paid, nothing kept": {
			sum: billing.DeleteSummary{
				ContactName:  "Never Paid",
				ContactEmail: "never@example.com",
			},
			wantParts:   []string{"Deleted registration for Never Paid"},
			unwantParts: []string{"kept", "refunded"},
		},
		"open invoice voided": {
			sum: billing.DeleteSummary{
				ContactName:   "Open Invoice",
				ContactEmail:  "open@example.com",
				RetainedPence: 5000,
				InvoiceVoided: true,
			},
			wantParts: []string{"kept the £50.00", "voided open invoice"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := formatDeleteAuditSummary(tc.sum)
			for _, want := range tc.wantParts {
				if !strings.Contains(got, want) {
					t.Errorf("summary = %q, want it to contain %q", got, want)
				}
			}
			for _, unwanted := range tc.unwantParts {
				if strings.Contains(got, unwanted) {
					t.Errorf("summary = %q, should not contain %q", got, unwanted)
				}
			}
		})
	}
}

package mapper

import (
	"testing"

	"pagasacentre/backend/internal/registration"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/testhelper"
)

func TestGolden_shirtSizes(t *testing.T) {
	sizes := domain.ListShirtSizes()
	grouped := map[string][]domain.ShirtSize{"adult": {}, "child": {}}
	for _, s := range sizes {
		grouped[s.Category] = append(grouped[s.Category], s)
	}
	testhelper.AssertGoldenJSON(t, "shirt_sizes.json", ShirtSizesToResponse(sizes, grouped, domain.ShirtSizeNotApplicable))
}

func TestGolden_pricing_full(t *testing.T) {
	snap := registration.PricingSnapshot{
		Mode:                   domain.PaymentModeDeposit,
		Currency:               "GBP",
		DepositAmountPence:     5000,
		AccommodationTiers:     []registration.PricingTier{{Code: "tent", DisplayName: "Tent", AmountPence: 12000}},
		ChildUnder3AmountPence: 0,
		DayPassAmountPence:     1500,
		CoachAmountPence:       800,
	}
	testhelper.AssertGoldenJSON(t, "pricing_full.json", PricingToResponse(&snap))
}

func TestGolden_pricing_nilTiers(t *testing.T) {
	snap := registration.PricingSnapshot{Mode: domain.PaymentModeDeposit, Currency: "GBP"}
	testhelper.AssertGoldenJSON(t, "pricing_nil_slices.json", PricingToResponse(&snap))
}

func TestGolden_submitResponse_full(t *testing.T) {
	resp := testhelper.SampleSubmitResponse()
	testhelper.AssertGoldenJSON(t, "submit_response_full.json", SubmitToResponse(&resp))
}

func TestGolden_summary_full(t *testing.T) {
	sum := testhelper.SampleRegistrationSummary()
	testhelper.AssertGoldenJSON(t, "summary_full.json", SummaryToResponse(&sum))
}

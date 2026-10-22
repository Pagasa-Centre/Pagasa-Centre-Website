package registration

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"pagasacentre/backend/internal/api/registration/dto"
	"pagasacentre/backend/internal/api/registration/dto/mapper"
	"pagasacentre/backend/internal/registration"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/testhelper"
	"pagasacentre/backend/pkg/commonlibrary/request"
)

func TestGolden_shirtSizes(t *testing.T) {
	sizes := domain.ListShirtSizes()
	grouped := map[string][]domain.ShirtSize{"adult": {}, "child": {}}
	for _, s := range sizes {
		grouped[s.Category] = append(grouped[s.Category], s)
	}
	testhelper.AssertGoldenJSON(t, "shirt_sizes.json", mapper.ShirtSizesToResponse(sizes, grouped, domain.ShirtSizeNotApplicable))
}

func samplePricingSnapshot() registration.PricingSnapshot {
	return registration.PricingSnapshot{
		Mode:                   domain.PaymentModeDeposit,
		Currency:               "GBP",
		DepositAmountPence:     5000,
		AccommodationTiers:     []registration.PricingTier{{Code: "tent", DisplayName: "Tent", AmountPence: 12000}},
		ChildUnder3AmountPence: 0,
		DayPassAmountPence:     1500,
		CoachAmountPence:       800,
	}
}

func TestGolden_pricing_full(t *testing.T) {
	snap := samplePricingSnapshot()
	testhelper.AssertGoldenJSON(t, "pricing_full.json", mapper.PricingToResponse(&snap))
}

func TestGolden_submitResponse_full(t *testing.T) {
	resp := testhelper.SampleSubmitResponse()
	testhelper.AssertGoldenJSON(t, "submit_response_full.json", mapper.SubmitToResponse(&resp))
}

func TestGolden_summary_full(t *testing.T) {
	sum := testhelper.SampleRegistrationSummary()
	testhelper.AssertGoldenJSON(t, "summary_full.json", mapper.SummaryToResponse(&sum))
}

func TestGolden_submitRequest_roundTrip(t *testing.T) {
	dtoReq := mapper.SubmissionToRequest(testhelper.SampleSubmitRequest())
	testhelper.AssertGoldenMarshal(t, "submit_request_full.json", dtoReq)

	want := mapper.RequestToDomain(dtoReq)
	body, err := json.Marshal(dtoReq)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	var got dto.SubmitRequest
	if err := request.Decode(r, &got); err != nil {
		t.Fatal(err)
	}
	gotDomain := mapper.RequestToDomain(got)
	if gotDomain.Contact.FirstName != want.Contact.FirstName || len(gotDomain.Campers) != len(want.Campers) {
		t.Fatalf("decode mismatch: %+v vs %+v", gotDomain, want)
	}
}

func TestGolden_pricing_nilTiers(t *testing.T) {
	snap := registration.PricingSnapshot{
		Mode:     domain.PaymentModeDeposit,
		Currency: "GBP",
	}
	testhelper.AssertGoldenJSON(t, "pricing_nil_slices.json", mapper.PricingToResponse(&snap))
}

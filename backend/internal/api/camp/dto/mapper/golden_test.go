package mapper

import (
	"testing"

	campdomain "pagasacentre/backend/internal/camp/domain"
	"pagasacentre/backend/internal/testhelper"
)

func TestGolden_configResponse(t *testing.T) {
	testhelper.AssertGoldenJSON(t, "config_response_full.json", ConfigToResponse(testhelper.SampleCampConfig()))
}

func TestGolden_pricesResponse_full(t *testing.T) {
	testhelper.AssertGoldenJSON(t, "prices_response_full.json", PricesToResponse([]campdomain.Price{testhelper.SampleCampPrice()}))
}

func TestGolden_pricesResponse_empty(t *testing.T) {
	testhelper.AssertGoldenJSON(t, "prices_response_empty.json", PricesToResponse(nil))
}

package camp

import (
	"testing"

	"pagasacentre/backend/internal/api/camp/dto/mapper"
	campdomain "pagasacentre/backend/internal/camp/domain"
	"pagasacentre/backend/internal/testhelper"
)

func TestGolden_configResponse(t *testing.T) {
	testhelper.AssertGoldenJSON(t, "config_response_full.json", mapper.ConfigToResponse(testhelper.SampleCampConfig()))
}

func TestGolden_pricesResponse_full(t *testing.T) {
	testhelper.AssertGoldenJSON(t, "prices_response_full.json", mapper.PricesToResponse([]campdomain.Price{testhelper.SampleCampPrice()}))
}

func TestGolden_pricesResponse_empty(t *testing.T) {
	testhelper.AssertGoldenJSON(t, "prices_response_empty.json", mapper.PricesToResponse(nil))
}

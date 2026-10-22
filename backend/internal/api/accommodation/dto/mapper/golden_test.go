package mapper

import (
	"testing"

	accomdomain "pagasacentre/backend/internal/accommodation/domain"
	"pagasacentre/backend/internal/testhelper"
)

func TestGolden_optionsResponse_full(t *testing.T) {
	testhelper.AssertGoldenJSON(t, "options_response_full.json", OptionsToResponse([]accomdomain.Option{testhelper.SamplePublicAccommodationOption()}))
}

func TestGolden_optionsResponse_empty(t *testing.T) {
	testhelper.AssertGoldenJSON(t, "options_response_empty.json", OptionsToResponse([]accomdomain.Option{}))
}

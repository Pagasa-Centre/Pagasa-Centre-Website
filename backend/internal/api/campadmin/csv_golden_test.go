package campadmin

import (
	"bytes"
	"testing"

	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/testhelper"
)

func TestGolden_registrationsCSV(t *testing.T) {
	g := testhelper.SampleGroupFull()
	c := testhelper.SampleCamperFull()
	var buf bytes.Buffer
	if err := WriteCSV(&buf, []domain.Group{g}, map[string][]domain.Camper{
		g.ID: {c},
	}); err != nil {
		t.Fatal(err)
	}
	testhelper.AssertGolden(t, "registrations.csv", buf.Bytes())
}

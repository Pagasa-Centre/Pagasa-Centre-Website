package sheets

import (
	"testing"

	"pagasacentre/backend/internal/notify"
	"pagasacentre/backend/internal/registration/domain"
)

func TestValues_dayPassShirtSizeColumn(t *testing.T) {
	shirt := "adult_l"
	tshirt := domain.TshirtOptionTshirtOnly
	row := notify.SheetRow{
		GroupID:             "grp-1",
		PaymentStatus:       "paid",
		AttendanceType:      domain.AttendanceDayPass,
		ShirtSize:           &shirt,
		DayPassTshirtOption: &tshirt,
	}
	vals := values(row)
	if len(vals) < 19 {
		t.Fatalf("values too short: %d", len(vals))
	}
	if vals[18] != "adult_l" {
		t.Fatalf("shirt_size column = %v, want adult_l", vals[18])
	}
}

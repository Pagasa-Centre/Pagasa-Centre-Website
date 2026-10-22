package invoicing

import (
	"testing"

	"pagasacentre/backend/internal/registration/domain"
)

func TestDayPassLine(t *testing.T) {
	line, ok := dayPassLine(domain.Camper{
		AttendanceType: domain.AttendanceDayPass,
		DayPassDays:    []string{"mon", "tue", "wed"},
	}, "price_day")
	if !ok {
		t.Fatal("expected a day-pass line")
	}
	if line.PriceID != "price_day" || line.Quantity != 3 {
		t.Fatalf("got %+v, want {price_day 3}", line)
	}

	if _, ok := dayPassLine(domain.Camper{AttendanceType: domain.AttendanceFullWeek}, "price_day"); ok {
		t.Error("full-week camper should not produce a day-pass line")
	}
	if _, ok := dayPassLine(domain.Camper{AttendanceType: domain.AttendanceDayPass}, "price_day"); ok {
		t.Error("day-pass camper with no days should not produce a line")
	}
}

func TestCoachLine(t *testing.T) {
	line, ok := coachLine(3, "price_coach")
	if !ok || line.PriceID != "price_coach" || line.Quantity != 3 {
		t.Fatalf("got %+v ok=%v, want {price_coach 3} true", line, ok)
	}
	if _, ok := coachLine(0, "price_coach"); ok {
		t.Error("zero passengers should not produce a coach line")
	}
}

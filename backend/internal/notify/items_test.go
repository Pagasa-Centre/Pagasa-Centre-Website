package notify_test

import (
	"testing"

	"pagasacentre/backend/internal/notify"
	"pagasacentre/backend/internal/registration/domain"
)

func strPtr(s string) *string { return &s }

func TestBalanceItemsIncludesUnit(t *testing.T) {
	unit := "caravan_5"
	items := notify.BalanceItems(
		[]domain.Camper{{
			FirstName:                  "Josh",
			LastName:                   "Basco",
			AttendanceType:             domain.AttendanceFullWeek,
			AllocatedAccommodationCode: strPtr("static_caravan"),
			AllocatedUnitCode:          &unit,
		}},
		map[string]string{"static_caravan": "Static Caravan"},
		map[string]string{"caravan_5": "Caravan 5"},
		0,
	)
	if len(items) != 1 || items[0].Kind != notify.ItemAccommodation {
		t.Fatalf("items = %+v", items)
	}
}

func TestCoachEligibleCountViaDomain(t *testing.T) {
	yes := true
	no := false
	campers := []domain.Camper{
		{AttendanceType: domain.AttendanceFullWeek, Age: 30, NeedsCoach: &yes},
		{AttendanceType: domain.AttendanceFullWeek, Age: 4, NeedsCoach: &yes},
		{AttendanceType: domain.AttendanceFullWeek, Age: 3, NeedsCoach: &yes},
		{AttendanceType: domain.AttendanceFullWeek, Age: 25, NeedsCoach: &no},
		{AttendanceType: domain.AttendanceFullWeek, Age: 25, NeedsCoach: nil},
		{AttendanceType: domain.AttendanceDayPass, Age: 25, NeedsCoach: &yes},
	}
	if n := domain.CoachEligibleCount(campers); n != 2 {
		t.Fatalf("coachEligibleCount = %d, want 2", n)
	}
}

func TestOffPreferenceChangesStructure(t *testing.T) {
	names := map[string]string{"lodge": "Lodge", "cabin": "Cabin", "tent": "Tent"}
	c := domain.Camper{
		FirstName: "C", LastName: "Three",
		AttendanceType:             domain.AttendanceFullWeek,
		AccommodationFirstChoice:   strPtr("lodge"),
		AccommodationSecondChoice:  strPtr("cabin"),
		AllocatedAccommodationCode: strPtr("tent"),
	}
	got := notify.OffPreferenceChanges([]domain.Camper{c}, names)
	if len(got) != 1 || !got[0].TentGuidance {
		t.Fatalf("got = %+v", got)
	}
}

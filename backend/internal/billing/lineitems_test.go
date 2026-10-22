package billing_test

import (
	"testing"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/registration/domain"
)

func strPtr(s string) *string { return &s }

func TestBalanceInvoiceItemsIncludesUnit(t *testing.T) {
	unit := "caravan_5"
	items := billing.BalanceInvoiceItems(
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
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	want := "Josh Basco — Static Caravan (Caravan 5)"
	if items[0] != want {
		t.Fatalf("got %q, want %q", items[0], want)
	}
}

func TestBalanceInvoiceItemsIncludesDayPass(t *testing.T) {
	items := billing.BalanceInvoiceItems(
		[]domain.Camper{
			{
				FirstName:                  "Josh",
				LastName:                   "Basco",
				AttendanceType:             domain.AttendanceFullWeek,
				AllocatedAccommodationCode: strPtr("lodge"),
			},
			{
				FirstName:      "Sam",
				LastName:       "Visitor",
				AttendanceType: domain.AttendanceDayPass,
				DayPassDays:    []string{"mon", "tue"},
			},
			{
				FirstName:      "Solo",
				LastName:       "Day",
				AttendanceType: domain.AttendanceDayPass,
				DayPassDays:    []string{"wed"},
			},
		},
		map[string]string{"lodge": "Lodge"},
		map[string]string{},
		0,
	)
	if len(items) != 3 {
		t.Fatalf("items = %v, want 3", items)
	}
	if items[0] != "Josh Basco — Lodge" {
		t.Errorf("full-week line = %q", items[0])
	}
	if items[1] != "Sam Visitor — Day pass (2 days)" {
		t.Errorf("day-pass line = %q, want plural", items[1])
	}
	if items[2] != "Solo Day — Day pass (1 day)" {
		t.Errorf("day-pass line = %q, want singular", items[2])
	}
}

func TestCoachEligibleCount(t *testing.T) {
	yes := true
	no := false
	campers := []domain.Camper{
		{AttendanceType: domain.AttendanceFullWeek, Age: 30, NeedsCoach: &yes}, // counts
		{AttendanceType: domain.AttendanceFullWeek, Age: 4, NeedsCoach: &yes},  // counts (>= 4)
		{AttendanceType: domain.AttendanceFullWeek, Age: 3, NeedsCoach: &yes},  // free (under 4)
		{AttendanceType: domain.AttendanceFullWeek, Age: 25, NeedsCoach: &no},  // opted out
		{AttendanceType: domain.AttendanceFullWeek, Age: 25, NeedsCoach: nil},  // not set
		{AttendanceType: domain.AttendanceDayPass, Age: 25, NeedsCoach: &yes},  // day-pass excluded
	}
	if n := billing.CoachEligibleCount(campers); n != 2 {
		t.Fatalf("coachEligibleCount = %d, want 2", n)
	}
}

func TestBalanceInvoiceItemsIncludesCoach(t *testing.T) {
	items := billing.BalanceInvoiceItems(
		[]domain.Camper{{
			FirstName:                  "Josh",
			LastName:                   "Basco",
			AttendanceType:             domain.AttendanceFullWeek,
			AllocatedAccommodationCode: strPtr("lodge"),
		}},
		map[string]string{"lodge": "Lodge"},
		map[string]string{},
		2,
	)
	if len(items) != 2 {
		t.Fatalf("items = %v, want 2", items)
	}
	if items[1] != "Coach transport (×2)" {
		t.Fatalf("coach line = %q", items[1])
	}
	// No coach line when count is 0.
	none := billing.BalanceInvoiceItems(
		[]domain.Camper{{
			FirstName:                  "Josh",
			LastName:                   "Basco",
			AttendanceType:             domain.AttendanceFullWeek,
			AllocatedAccommodationCode: strPtr("lodge"),
		}},
		map[string]string{"lodge": "Lodge"},
		map[string]string{},
		0,
	)
	if len(none) != 1 {
		t.Fatalf("items = %v, want 1 (no coach)", none)
	}
}

func TestOffPreferenceChanges(t *testing.T) {
	names := map[string]string{
		"lodge": "Lodge",
		"cabin": "Cabin",
		"tent":  "Tent",
	}
	cases := []struct {
		name  string
		c     domain.Camper
		wantN int
		tent  bool
	}{
		{
			name: "matches first choice",
			c: domain.Camper{
				FirstName: "A", LastName: "One",
				AttendanceType:             domain.AttendanceFullWeek,
				AccommodationFirstChoice:   strPtr("lodge"),
				AccommodationSecondChoice:  strPtr("cabin"),
				AllocatedAccommodationCode: strPtr("lodge"),
			},
			wantN: 0,
		},
		{
			name: "matches second choice",
			c: domain.Camper{
				FirstName: "B", LastName: "Two",
				AttendanceType:             domain.AttendanceFullWeek,
				AccommodationFirstChoice:   strPtr("lodge"),
				AccommodationSecondChoice:  strPtr("cabin"),
				AllocatedAccommodationCode: strPtr("cabin"),
			},
			wantN: 0,
		},
		{
			name: "off preference",
			c: domain.Camper{
				FirstName: "C", LastName: "Three",
				AttendanceType:             domain.AttendanceFullWeek,
				AccommodationFirstChoice:   strPtr("lodge"),
				AccommodationSecondChoice:  strPtr("cabin"),
				AllocatedAccommodationCode: strPtr("tent"),
			},
			wantN: 1,
			tent:  true,
		},
		{
			name: "nil second choice still off preference",
			c: domain.Camper{
				FirstName: "D", LastName: "Four",
				AttendanceType:             domain.AttendanceFullWeek,
				AccommodationFirstChoice:   strPtr("lodge"),
				AllocatedAccommodationCode: strPtr("tent"),
			},
			wantN: 1,
			tent:  true,
		},
		{
			name: "both choices empty skipped",
			c: domain.Camper{
				FirstName: "E", LastName: "Five",
				AttendanceType:             domain.AttendanceFullWeek,
				AllocatedAccommodationCode: strPtr("tent"),
			},
			wantN: 0,
		},
		{
			name: "day pass skipped",
			c: domain.Camper{
				FirstName: "F", LastName: "Six",
				AttendanceType:             domain.AttendanceDayPass,
				AccommodationFirstChoice:   strPtr("lodge"),
				AllocatedAccommodationCode: strPtr("tent"),
			},
			wantN: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := billing.OffPreferenceChanges([]domain.Camper{tc.c}, names)
			if len(got) != tc.wantN {
				t.Fatalf("len = %d, want %d", len(got), tc.wantN)
			}
			if tc.wantN == 1 {
				if got[0].Allocated != "Tent" {
					t.Fatalf("allocated = %q, want Tent", got[0].Allocated)
				}
				if got[0].TentGuidance != tc.tent {
					t.Fatalf("TentGuidance = %v, want %v", got[0].TentGuidance, tc.tent)
				}
			}
		})
	}
}

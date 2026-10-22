package domain

import "testing"

func TestCoachEligible(t *testing.T) {
	yes := true
	no := false
	tests := []struct {
		name  string
		att   string
		needs *bool
		age   int
		want  bool
	}{
		{"full week coach age 4", AttendanceFullWeek, &yes, 4, true},
		{"full week coach age 3", AttendanceFullWeek, &yes, 3, false},
		{"day pass coach", AttendanceDayPass, &yes, 10, false},
		{"needs coach nil", AttendanceFullWeek, nil, 10, false},
		{"needs coach false", AttendanceFullWeek, &no, 10, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := CoachEligible(tc.att, tc.needs, tc.age); got != tc.want {
				t.Fatalf("CoachEligible = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAccommodationPriceID(t *testing.T) {
	under3 := "price_under3"
	ref := "price_tent"
	tier := AccommodationType{Code: AccommodationTent, PriceRef: &ref}
	got, err := AccommodationPriceID(AccommodationChild, 2, &tier, under3)
	if err != nil || got != under3 {
		t.Fatalf("under3: got %q err=%v", got, err)
	}
	got, err = AccommodationPriceID(AccommodationTent, 20, &tier, under3)
	if err != nil || got != ref {
		t.Fatalf("tent: got %q err=%v", got, err)
	}
	if _, err := AccommodationPriceID(AccommodationChild, 2, &tier, ""); err == nil {
		t.Fatal("expected error when under3 ref missing")
	}
}

func TestCamperCoachEligibleAtAge(t *testing.T) {
	yes := true
	c := Camper{AttendanceType: AttendanceFullWeek, NeedsCoach: &yes, Age: 5}
	if !c.CoachEligibleAtAge(4) {
		t.Fatal("expected eligible at age 4")
	}
	if c.CoachEligibleAtAge(3) {
		t.Fatal("expected not eligible at age 3")
	}
}

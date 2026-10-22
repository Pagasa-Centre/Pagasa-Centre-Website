package domain

import (
	"fmt"
	"regexp"
	"strings"
)

const MinDepositAge = 4

const (
	AccommodationChild       = "child"
	AccommodationTent        = "tent"
	MaxChildAccommodationAge = 12
	maxRoommateRequestLen    = 500
)

var emailRE = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// PriceRow is a catalogued camp price (amount + currency).
type PriceRow struct {
	AmountPence int
	Currency    string
}

func ValidEmail(s string) bool {
	return emailRE.MatchString(strings.TrimSpace(s))
}

// CoachEligible reports whether a full-week camper counts as a coach passenger at age.
func CoachEligible(attendance string, needsCoach *bool, age int) bool {
	if attendance != AttendanceFullWeek {
		return false
	}
	if needsCoach == nil || !*needsCoach {
		return false
	}
	return age >= MinDepositAge
}

func (c Camper) CoachEligible() bool {
	return CoachEligible(c.AttendanceType, c.NeedsCoach, c.Age)
}

func (c Camper) CoachEligibleAtAge(age int) bool {
	return CoachEligible(c.AttendanceType, c.NeedsCoach, age)
}

func (c NewCamper) CoachEligible() bool {
	return CoachEligible(c.Attendance.Type, c.Attendance.NeedsCoach, c.Age)
}

// AccommodationPriceID mirrors the billing resolver so both modes price a
// given tier + age from the same Stripe Price.
func AccommodationPriceID(code string, age int, tier *AccommodationType, childUnder3PriceID string) (string, error) {
	if code == AccommodationChild && age < MinDepositAge {
		if childUnder3PriceID == "" {
			return "", fmt.Errorf("STRIPE_PRICE_CHILD_UNDER3 is not configured")
		}
		return childUnder3PriceID, nil
	}
	if tier == nil {
		return "", fmt.Errorf("unknown accommodation %q", code)
	}
	if tier.PriceRef == nil || strings.TrimSpace(*tier.PriceRef) == "" {
		return "", fmt.Errorf("accommodation %q has no stripe_price_id configured", code)
	}
	return strings.TrimSpace(*tier.PriceRef), nil
}

// ValidateCamper applies every rule that must hold for one camper, whoever is
// creating them. The public form runs it per camper in a submission; admin edits
// and admin-added campers run it on a single person, so both paths enforce the
// same thing and cannot drift apart.
func ValidateCamper(prefix string, c NewCamper, fields map[string]string) {
	if strings.TrimSpace(c.FirstName) == "" {
		fields[prefix+".first_name"] = "is required"
	}
	if strings.TrimSpace(c.LastName) == "" {
		fields[prefix+".last_name"] = "is required"
	}
	if c.Gender != GenderMale && c.Gender != GenderFemale {
		fields[prefix+".gender"] = "must be 'male' or 'female'"
	}
	if c.Age <= 0 || c.Age >= 120 {
		fields[prefix+".age"] = "must be between 1 and 119"
	}
	if strings.TrimSpace(c.CellLeaderName) == "" {
		fields[prefix+".cell_leader_name"] = "is required"
	}

	switch c.Attendance.Type {
	case AttendanceFullWeek:
		validateFullWeek(prefix+".attendance", c.Attendance, c.Age, fields)
	case AttendanceDayPass:
		ValidateDayPass(prefix+".attendance", c.Attendance, fields)
	default:
		fields[prefix+".attendance.type"] = "must be 'full_week' or 'day_pass'"
	}
}

// ValidateAllocatedAccommodation applies the child-accommodation age limit to a
// camper's actual placement, not just the preference they picked at signup.
func ValidateAllocatedAccommodation(prefix, code string, age int, fields map[string]string) {
	if strings.TrimSpace(code) == AccommodationChild && age > MaxChildAccommodationAge {
		fields[prefix] = fmt.Sprintf(
			"child accommodation is only available for campers aged %d or under; "+
				"move them to another accommodation first", MaxChildAccommodationAge)
	}
}

func validateFullWeek(prefix string, a Attendance, age int, fields map[string]string) {
	first := strings.TrimSpace(a.AccommodationFirstChoice)
	second := strings.TrimSpace(a.AccommodationSecondChoice)
	if first == "" {
		fields[prefix+".accommodation_first_choice"] = "is required for full week attendance"
	}
	if first == AccommodationChild && age > MaxChildAccommodationAge {
		fields[prefix+".accommodation_first_choice"] = fmt.Sprintf(
			"child accommodation is only available for campers aged %d or under", MaxChildAccommodationAge)
	}
	if second == AccommodationChild && age > MaxChildAccommodationAge {
		fields[prefix+".accommodation_second_choice"] = fmt.Sprintf(
			"child accommodation is only available for campers aged %d or under", MaxChildAccommodationAge)
	}
	if first != AccommodationChild && first != AccommodationTent && second == "" {
		fields[prefix+".accommodation_second_choice"] = "is required for full week attendance"
	}
	if first != "" && second != "" && first == second {
		fields[prefix+".accommodation_second_choice"] = "must be different from the first choice"
	}
	if len(a.RoommateRequests) > maxRoommateRequestLen {
		fields[prefix+".roommate_requests"] = fmt.Sprintf("must be %d characters or fewer", maxRoommateRequestLen)
	}
	switch {
	case strings.TrimSpace(a.ShirtSize) == "":
		fields[prefix+".shirt_size"] = "is required for full week attendance"
	case !IsRealShirtSize(a.ShirtSize):
		fields[prefix+".shirt_size"] = "must be one of the catalogued shirt sizes (see GET /api/shirt-sizes)"
	}
}

func ValidateDayPass(prefix string, a Attendance, fields map[string]string) {
	if len(a.Days) == 0 {
		fields[prefix+".days"] = "select at least one day"
	}
	for _, d := range a.Days {
		if _, ok := ValidDayPassDays[d]; !ok {
			fields[prefix+".days"] = "must be a subset of [mon, tue, wed, thu, fri]"
			break
		}
	}
	switch a.TshirtOption {
	case TshirtOptionTeamActivities, TshirtOptionTshirtOnly:
		switch {
		case strings.TrimSpace(a.ShirtSize) == "":
			fields[prefix+".shirt_size"] = "is required when purchasing a t-shirt"
		case !IsRealShirtSize(a.ShirtSize):
			fields[prefix+".shirt_size"] = "must be one of the catalogued shirt sizes (see GET /api/shirt-sizes)"
		}
	case TshirtOptionNone:
		if s := strings.TrimSpace(a.ShirtSize); s != "" && !strings.EqualFold(s, ShirtSizeNotApplicable) {
			fields[prefix+".shirt_size"] = `must be "n/a" or empty when not purchasing a t-shirt`
		}
	default:
		fields[prefix+".tshirt_option"] = "must be one of team_activities, tshirt_only, none"
	}
	if a.NeedsCatering == nil {
		fields[prefix+".needs_catering"] = "is required for day pass attendance"
	}
}

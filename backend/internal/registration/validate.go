package registration

import (
	"fmt"
	"regexp"
	"strings"

	"pagasacentre/backend/internal/httpx"
)

var emailRE = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// Validate enforces the structural and business rules from the camp form.
// Returns httpx.APIError with per-field messages, or nil on success.
func Validate(req SubmitRequest) error {
	fields := map[string]string{}

	// Contact
	if strings.TrimSpace(req.Contact.FirstName) == "" {
		fields["contact.first_name"] = "is required"
	}
	if strings.TrimSpace(req.Contact.LastName) == "" {
		fields["contact.last_name"] = "is required"
	}
	if !emailRE.MatchString(req.Contact.Email) {
		fields["contact.email"] = "must be a valid email"
	}
	if strings.TrimSpace(req.Contact.Phone) == "" {
		fields["contact.phone"] = "is required"
	}

	// Campers
	if len(req.Campers) == 0 {
		fields["campers"] = "at least one camper is required"
	}

	mainContactCount := 0
	for i, c := range req.Campers {
		prefix := fmt.Sprintf("campers[%d]", i)
		if c.IsMainContact {
			mainContactCount++
		}
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
			validateDayPass(prefix+".attendance", c.Attendance, fields)
		default:
			fields[prefix+".attendance.type"] = "must be 'full_week' or 'day_pass'"
		}
	}

	if len(req.Campers) > 0 && mainContactCount != 1 {
		fields["campers"] = "exactly one camper must be marked as the main contact"
	}

	if len(fields) > 0 {
		return httpx.ValidationFailed(fields)
	}
	return nil
}

// maxRoommateRequestLen caps free-text roommate request input. Anything longer
// is almost certainly malicious / accidental paste — committee reads these
// manually so we don't want walls of text.
const maxRoommateRequestLen = 500

// AccommodationChild is the code for "Child accommodation (sharing with
// parent)". When a camper picks this as their 1st choice, a 2nd choice is
// meaningless — they're inherently with their parent — so we skip the
// usual "2nd choice required + must differ" rules. Kept in sync with the
// 'child' row seeded in migration 0001.
const AccommodationChild = "child"

// MaxChildAccommodationAge caps the child-with-parent accommodation option.
// Anyone older sleeps in their own bed in a regular tier (lodge/cabin/etc.).
// Mirrors frontend MAX_CHILD_ACCOMMODATION_AGE.
const MaxChildAccommodationAge = 12

func validateFullWeek(prefix string, a AttendanceDTO, age int, fields map[string]string) {
	first := strings.TrimSpace(a.AccommodationFirstChoice)
	second := strings.TrimSpace(a.AccommodationSecondChoice)
	if first == "" {
		fields[prefix+".accommodation_first_choice"] = "is required for full week attendance"
	}
	// Child-with-parent option is only valid for actual children. The age
	// threshold is enforced server-side because the frontend toggle is
	// trivially bypassable.
	if first == AccommodationChild && age > MaxChildAccommodationAge {
		fields[prefix+".accommodation_first_choice"] = fmt.Sprintf(
			"child accommodation is only available for campers aged %d or under", MaxChildAccommodationAge)
	}
	if second == AccommodationChild && age > MaxChildAccommodationAge {
		fields[prefix+".accommodation_second_choice"] = fmt.Sprintf(
			"child accommodation is only available for campers aged %d or under", MaxChildAccommodationAge)
	}
	// 2nd choice is required for everything EXCEPT the child-with-parent
	// option, which by definition has no meaningful fallback.
	if first != AccommodationChild && second == "" {
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

func validateDayPass(prefix string, a AttendanceDTO, fields map[string]string) {
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
		// Day-pass holders not buying a t-shirt should submit "n/a" (or leave
		// the field empty). Anything else is suspicious — reject it.
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

// HasMinor returns true if any camper is under 18.
func HasMinor(req SubmitRequest) bool {
	for _, c := range req.Campers {
		if c.Age < 18 {
			return true
		}
	}
	return false
}

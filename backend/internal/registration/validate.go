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
			validateFullWeek(prefix+".attendance", c.Attendance, fields)
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

func validateFullWeek(prefix string, a AttendanceDTO, fields map[string]string) {
	if strings.TrimSpace(a.AccommodationCode) == "" {
		fields[prefix+".accommodation_code"] = "is required for full week attendance"
	}
	if strings.TrimSpace(a.ShirtSize) == "" {
		fields[prefix+".shirt_size"] = "is required for full week attendance"
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
		if strings.TrimSpace(a.ShirtSize) == "" {
			fields[prefix+".shirt_size"] = "is required when purchasing a t-shirt"
		}
	case TshirtOptionNone:
		// ok
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

package registration

import (
	"fmt"
	"regexp"
	"strings"

	"pagasacentre/backend/internal/registration/domain"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
)

var emailRE = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

func ValidEmail(s string) bool {
	return emailRE.MatchString(strings.TrimSpace(s))
}

func Validate(req domain.SubmitRequest) error {
	fields := map[string]string{}

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

	if len(req.Campers) == 0 {
		fields["campers"] = "at least one camper is required"
	}

	mainContactCount := 0
	for i, c := range req.Campers {
		prefix := fmt.Sprintf("campers[%d]", i)
		if c.IsMainContact {
			mainContactCount++
		}
		ValidateCamper(prefix, c, fields)
	}

	if len(req.Campers) > 0 && mainContactCount != 1 {
		fields["campers"] = "exactly one camper must be marked as the main contact"
	}

	if len(fields) > 0 {
		return commonerrors.ValidationFailed(fields)
	}
	return nil
}

const maxRoommateRequestLen = 500

const AccommodationChild = "child"
const AccommodationTent = "tent"
const MaxChildAccommodationAge = 12

// ValidateCamper applies every rule that must hold for one camper, whoever is
// creating them. The public form runs it per camper in a submission; admin edits
// and admin-added campers run it on a single person, so both paths enforce the
// same thing and cannot drift apart.
func ValidateCamper(prefix string, c domain.CamperDTO, fields map[string]string) {
	if strings.TrimSpace(c.FirstName) == "" {
		fields[prefix+".first_name"] = "is required"
	}
	if strings.TrimSpace(c.LastName) == "" {
		fields[prefix+".last_name"] = "is required"
	}
	if c.Gender != domain.GenderMale && c.Gender != domain.GenderFemale {
		fields[prefix+".gender"] = "must be 'male' or 'female'"
	}
	if c.Age <= 0 || c.Age >= 120 {
		fields[prefix+".age"] = "must be between 1 and 119"
	}
	if strings.TrimSpace(c.CellLeaderName) == "" {
		fields[prefix+".cell_leader_name"] = "is required"
	}

	switch c.Attendance.Type {
	case domain.AttendanceFullWeek:
		validateFullWeek(prefix+".attendance", c.Attendance, c.Age, fields)
	case domain.AttendanceDayPass:
		ValidateDayPass(prefix+".attendance", c.Attendance, fields)
	default:
		fields[prefix+".attendance.type"] = "must be 'full_week' or 'day_pass'"
	}
}

// ValidateAllocatedAccommodation applies the child-accommodation age limit to a
// camper's actual placement, not just the preference they picked at signup.
// Registration only ever sees preferences, so this rule has no reason to exist
// there — but an admin can age a camper past the limit while they sit in a child
// room, which is exactly how a teenager ends up in "sharing with parent".
func ValidateAllocatedAccommodation(prefix, code string, age int, fields map[string]string) {
	if strings.TrimSpace(code) == AccommodationChild && age > MaxChildAccommodationAge {
		fields[prefix] = fmt.Sprintf(
			"child accommodation is only available for campers aged %d or under; "+
				"move them to another accommodation first", MaxChildAccommodationAge)
	}
}

func validateFullWeek(prefix string, a domain.AttendanceDTO, age int, fields map[string]string) {
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
	case !domain.IsRealShirtSize(a.ShirtSize):
		fields[prefix+".shirt_size"] = "must be one of the catalogued shirt sizes (see GET /api/shirt-sizes)"
	}
}

func ValidateDayPass(prefix string, a domain.AttendanceDTO, fields map[string]string) {
	if len(a.Days) == 0 {
		fields[prefix+".days"] = "select at least one day"
	}
	for _, d := range a.Days {
		if _, ok := domain.ValidDayPassDays[d]; !ok {
			fields[prefix+".days"] = "must be a subset of [mon, tue, wed, thu, fri]"
			break
		}
	}
	switch a.TshirtOption {
	case domain.TshirtOptionTeamActivities, domain.TshirtOptionTshirtOnly:
		switch {
		case strings.TrimSpace(a.ShirtSize) == "":
			fields[prefix+".shirt_size"] = "is required when purchasing a t-shirt"
		case !domain.IsRealShirtSize(a.ShirtSize):
			fields[prefix+".shirt_size"] = "must be one of the catalogued shirt sizes (see GET /api/shirt-sizes)"
		}
	case domain.TshirtOptionNone:
		if s := strings.TrimSpace(a.ShirtSize); s != "" && !strings.EqualFold(s, domain.ShirtSizeNotApplicable) {
			fields[prefix+".shirt_size"] = `must be "n/a" or empty when not purchasing a t-shirt`
		}
	default:
		fields[prefix+".tshirt_option"] = "must be one of team_activities, tshirt_only, none"
	}
	if a.NeedsCatering == nil {
		fields[prefix+".needs_catering"] = "is required for day pass attendance"
	}
}

func HasMinor(req domain.SubmitRequest) bool {
	for _, c := range req.Campers {
		if c.Age < 18 {
			return true
		}
	}
	return false
}

// Re-export shirt helpers for API handlers.
func ListShirtSizes() []domain.ShirtSize { return domain.ListShirtSizes() }

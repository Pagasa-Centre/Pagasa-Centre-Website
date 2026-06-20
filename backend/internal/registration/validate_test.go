package registration

import (
	"errors"
	"testing"

	"pagasacentre/backend/internal/registration/domain"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
)

func boolPtr(b bool) *bool { return &b }

func validFullWeekCamper(main bool) domain.CamperDTO {
	return domain.CamperDTO{
		FirstName:      "Jane",
		LastName:       "Doe",
		Gender:         "female",
		Age:            30,
		CellLeaderName: "Pastor Bob",
		IsCellLeader:   false,
		IsMainContact:  main,
		Attendance: domain.AttendanceDTO{
			Type:                      domain.AttendanceFullWeek,
			ShirtSize:                 "adult_m",
			AccommodationFirstChoice:  "lodge",
			AccommodationSecondChoice: "cabin",
		},
	}
}

func validRequest() domain.SubmitRequest {
	return domain.SubmitRequest{
		Contact: domain.ContactDTO{
			FirstName: "Jane", LastName: "Doe",
			Email: "jane@example.com", Phone: "+44 1234 567890",
		},
		Campers: []domain.CamperDTO{validFullWeekCamper(true)},
	}
}

func assertFieldError(t *testing.T, err error, field string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error for field %s, got nil", field)
	}
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if _, ok := apiErr.Fields[field]; !ok {
		t.Fatalf("expected error on %s, got fields %v", field, apiErr.Fields)
	}
}

func TestValidate_HappyPath(t *testing.T) {
	if err := Validate(validRequest()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestValidate_MissingMainContact(t *testing.T) {
	req := validRequest()
	req.Campers[0].IsMainContact = false
	assertFieldError(t, Validate(req), "campers")
}

func TestValidate_TwoMainContacts(t *testing.T) {
	req := validRequest()
	req.Campers = append(req.Campers, validFullWeekCamper(true))
	assertFieldError(t, Validate(req), "campers")
}

func TestValidate_InvalidGender(t *testing.T) {
	req := validRequest()
	req.Campers[0].Gender = "other"
	assertFieldError(t, Validate(req), "campers[0].gender")
}

func TestValidate_InvalidAge(t *testing.T) {
	req := validRequest()
	req.Campers[0].Age = 0
	assertFieldError(t, Validate(req), "campers[0].age")
}

func TestValidate_FullWeekMissingFirstChoice(t *testing.T) {
	req := validRequest()
	req.Campers[0].Attendance.AccommodationFirstChoice = ""
	assertFieldError(t, Validate(req), "campers[0].attendance.accommodation_first_choice")
}

func TestValidate_FullWeekMissingSecondChoice(t *testing.T) {
	req := validRequest()
	req.Campers[0].Attendance.AccommodationSecondChoice = ""
	assertFieldError(t, Validate(req), "campers[0].attendance.accommodation_second_choice")
}

func TestValidate_FullWeekChildAccommodationDoesNotRequireSecondChoice(t *testing.T) {
	req := validRequest()
	// Use an age within the child-accommodation limit so we exercise the
	// "child code skips 2nd choice" branch rather than the new age gate.
	req.Campers[0].Age = MaxChildAccommodationAge
	req.Campers[0].Attendance.AccommodationFirstChoice = AccommodationChild
	req.Campers[0].Attendance.AccommodationSecondChoice = ""
	if err := Validate(req); err != nil {
		t.Fatalf("expected nil error when child + empty 2nd choice, got %v", err)
	}
}

func TestValidate_FullWeekTentDoesNotRequireSecondChoice(t *testing.T) {
	req := validRequest()
	req.Campers[0].Attendance.AccommodationFirstChoice = AccommodationTent
	req.Campers[0].Attendance.AccommodationSecondChoice = ""
	if err := Validate(req); err != nil {
		t.Fatalf("expected nil error when tent + empty 2nd choice, got %v", err)
	}
}

func TestValidate_FullWeekChildAccommodationRejectedOverAgeLimit(t *testing.T) {
	req := validRequest()
	req.Campers[0].Age = MaxChildAccommodationAge + 1
	req.Campers[0].Attendance.AccommodationFirstChoice = AccommodationChild
	req.Campers[0].Attendance.AccommodationSecondChoice = ""
	assertFieldError(t, Validate(req), "campers[0].attendance.accommodation_first_choice")
}

func TestValidate_FullWeekChildAccommodationRejectedAsSecondChoiceOverAgeLimit(t *testing.T) {
	req := validRequest()
	req.Campers[0].Age = MaxChildAccommodationAge + 1
	req.Campers[0].Attendance.AccommodationFirstChoice = "lodge"
	req.Campers[0].Attendance.AccommodationSecondChoice = AccommodationChild
	assertFieldError(t, Validate(req), "campers[0].attendance.accommodation_second_choice")
}

func TestValidate_FullWeekSecondChoiceMatchesFirst(t *testing.T) {
	req := validRequest()
	req.Campers[0].Attendance.AccommodationSecondChoice = req.Campers[0].Attendance.AccommodationFirstChoice
	assertFieldError(t, Validate(req), "campers[0].attendance.accommodation_second_choice")
}

func TestValidate_RoommateRequestsTooLong(t *testing.T) {
	req := validRequest()
	long := make([]byte, maxRoommateRequestLen+1)
	for i := range long {
		long[i] = 'x'
	}
	req.Campers[0].Attendance.RoommateRequests = string(long)
	assertFieldError(t, Validate(req), "campers[0].attendance.roommate_requests")
}

func TestValidate_DayPassEmptyDays(t *testing.T) {
	req := validRequest()
	req.Campers[0].Attendance = domain.AttendanceDTO{
		Type:          domain.AttendanceDayPass,
		Days:          nil,
		TshirtOption:  domain.TshirtOptionNone,
		NeedsCatering: boolPtr(false),
	}
	assertFieldError(t, Validate(req), "campers[0].attendance.days")
}

func TestValidate_DayPassTeamActivitiesRequiresShirtSize(t *testing.T) {
	req := validRequest()
	req.Campers[0].Attendance = domain.AttendanceDTO{
		Type:          domain.AttendanceDayPass,
		Days:          []string{"mon", "tue"},
		TshirtOption:  domain.TshirtOptionTeamActivities,
		ShirtSize:     "",
		NeedsCatering: boolPtr(true),
	}
	assertFieldError(t, Validate(req), "campers[0].attendance.shirt_size")
}

func TestValidate_FullWeekInvalidShirtSize(t *testing.T) {
	req := validRequest()
	req.Campers[0].Attendance.ShirtSize = "potato"
	assertFieldError(t, Validate(req), "campers[0].attendance.shirt_size")
}

func TestValidate_FullWeekRejectsNotApplicable(t *testing.T) {
	req := validRequest()
	req.Campers[0].Attendance.ShirtSize = domain.ShirtSizeNotApplicable
	assertFieldError(t, Validate(req), "campers[0].attendance.shirt_size")
}

func TestValidate_DayPassNoneAcceptsNotApplicable(t *testing.T) {
	req := validRequest()
	req.Campers[0].Attendance = domain.AttendanceDTO{
		Type:          domain.AttendanceDayPass,
		Days:          []string{"mon"},
		TshirtOption:  domain.TshirtOptionNone,
		ShirtSize:     domain.ShirtSizeNotApplicable,
		NeedsCatering: boolPtr(false),
	}
	if err := Validate(req); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidate_DayPassNoneAcceptsEmptyShirt(t *testing.T) {
	req := validRequest()
	req.Campers[0].Attendance = domain.AttendanceDTO{
		Type:          domain.AttendanceDayPass,
		Days:          []string{"mon"},
		TshirtOption:  domain.TshirtOptionNone,
		ShirtSize:     "",
		NeedsCatering: boolPtr(false),
	}
	if err := Validate(req); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidate_DayPassNoneRejectsRealSize(t *testing.T) {
	req := validRequest()
	req.Campers[0].Attendance = domain.AttendanceDTO{
		Type:          domain.AttendanceDayPass,
		Days:          []string{"mon"},
		TshirtOption:  domain.TshirtOptionNone,
		ShirtSize:     "adult_m",
		NeedsCatering: boolPtr(false),
	}
	assertFieldError(t, Validate(req), "campers[0].attendance.shirt_size")
}

func TestValidate_DayPassTeamActivitiesRejectsNotApplicable(t *testing.T) {
	req := validRequest()
	req.Campers[0].Attendance = domain.AttendanceDTO{
		Type:          domain.AttendanceDayPass,
		Days:          []string{"mon"},
		TshirtOption:  domain.TshirtOptionTeamActivities,
		ShirtSize:     domain.ShirtSizeNotApplicable,
		NeedsCatering: boolPtr(true),
	}
	assertFieldError(t, Validate(req), "campers[0].attendance.shirt_size")
}

func TestIsRealShirtSize(t *testing.T) {
	cases := map[string]bool{
		"adult_m":         true,
		"ADULT_M":         true, // case-insensitive
		"child_3_4y":      true,
		"child_12_18m":    true,
		domain.ShirtSizeNotApplicable: false, // n/a is not a "real" size
		"potato":          false,
		"":                false,
	}
	for in, want := range cases {
		if got := domain.IsRealShirtSize(in); got != want {
			t.Errorf("IsRealShirtSize(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestValidate_DayPassInvalidDay(t *testing.T) {
	req := validRequest()
	req.Campers[0].Attendance = domain.AttendanceDTO{
		Type:          domain.AttendanceDayPass,
		Days:          []string{"sat"},
		TshirtOption:  domain.TshirtOptionNone,
		NeedsCatering: boolPtr(false),
	}
	assertFieldError(t, Validate(req), "campers[0].attendance.days")
}

func TestValidate_ContactEmailInvalid(t *testing.T) {
	req := validRequest()
	req.Contact.Email = "not-an-email"
	assertFieldError(t, Validate(req), "contact.email")
}

func TestHasMinor(t *testing.T) {
	req := validRequest()
	if HasMinor(req) {
		t.Fatalf("expected no minor")
	}
	req.Campers[0].Age = 12
	if !HasMinor(req) {
		t.Fatalf("expected minor present")
	}
}

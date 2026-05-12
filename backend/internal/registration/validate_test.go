package registration

import (
	"errors"
	"testing"

	"pagasacentre/backend/internal/httpx"
)

func boolPtr(b bool) *bool { return &b }

func validFullWeekCamper(main bool) CamperDTO {
	return CamperDTO{
		FirstName:      "Jane",
		LastName:       "Doe",
		Gender:         "female",
		Age:            30,
		CellLeaderName: "Pastor Bob",
		IsCellLeader:   false,
		IsMainContact:  main,
		Attendance: AttendanceDTO{
			Type:              AttendanceFullWeek,
			ShirtSize:         "M",
			AccommodationCode: "lodge",
		},
	}
}

func validRequest() SubmitRequest {
	return SubmitRequest{
		Contact: ContactDTO{
			FirstName: "Jane", LastName: "Doe",
			Email: "jane@example.com", Phone: "+44 1234 567890",
		},
		Campers: []CamperDTO{validFullWeekCamper(true)},
	}
}

func assertFieldError(t *testing.T, err error, field string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error for field %s, got nil", field)
	}
	var apiErr httpx.APIError
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

func TestValidate_FullWeekMissingAccommodation(t *testing.T) {
	req := validRequest()
	req.Campers[0].Attendance.AccommodationCode = ""
	assertFieldError(t, Validate(req), "campers[0].attendance.accommodation_code")
}

func TestValidate_DayPassEmptyDays(t *testing.T) {
	req := validRequest()
	req.Campers[0].Attendance = AttendanceDTO{
		Type:          AttendanceDayPass,
		Days:          nil,
		TshirtOption:  TshirtOptionNone,
		NeedsCatering: boolPtr(false),
	}
	assertFieldError(t, Validate(req), "campers[0].attendance.days")
}

func TestValidate_DayPassTeamActivitiesRequiresShirtSize(t *testing.T) {
	req := validRequest()
	req.Campers[0].Attendance = AttendanceDTO{
		Type:          AttendanceDayPass,
		Days:          []string{"mon", "tue"},
		TshirtOption:  TshirtOptionTeamActivities,
		ShirtSize:     "",
		NeedsCatering: boolPtr(true),
	}
	assertFieldError(t, Validate(req), "campers[0].attendance.shirt_size")
}

func TestValidate_DayPassInvalidDay(t *testing.T) {
	req := validRequest()
	req.Campers[0].Attendance = AttendanceDTO{
		Type:          AttendanceDayPass,
		Days:          []string{"sat"},
		TshirtOption:  TshirtOptionNone,
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

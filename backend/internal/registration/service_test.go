package registration

import (
	"testing"

	"pagasacentre/backend/internal/accommodation"
)

func TestCollectSoldOut_HasRoom(t *testing.T) {
	cap20 := 20
	avail := []accommodation.Availability{
		{Code: "lodge", Capacity: &cap20, Taken: 5},
	}
	req := SubmitRequest{Campers: []CamperDTO{
		{Attendance: AttendanceDTO{Type: AttendanceFullWeek, AccommodationCode: "lodge"}},
	}}
	got := collectSoldOut(req, avail)
	if len(got) != 0 {
		t.Fatalf("expected no sold-out, got %v", got)
	}
}

func TestCollectSoldOut_Exhausted(t *testing.T) {
	cap5 := 5
	avail := []accommodation.Availability{
		{Code: "lodge", Capacity: &cap5, Taken: 5},
	}
	req := SubmitRequest{Campers: []CamperDTO{
		{Attendance: AttendanceDTO{Type: AttendanceFullWeek, AccommodationCode: "lodge"}},
	}}
	got := collectSoldOut(req, avail)
	if len(got) != 1 || got[0] != "lodge" {
		t.Fatalf("expected [lodge], got %v", got)
	}
}

func TestCollectSoldOut_GroupExceedsRemaining(t *testing.T) {
	cap5 := 5
	avail := []accommodation.Availability{
		{Code: "cabin", Capacity: &cap5, Taken: 4}, // 1 remaining
	}
	// two campers want cabin -> sold out
	req := SubmitRequest{Campers: []CamperDTO{
		{Attendance: AttendanceDTO{Type: AttendanceFullWeek, AccommodationCode: "cabin"}},
		{Attendance: AttendanceDTO{Type: AttendanceFullWeek, AccommodationCode: "cabin"}},
	}}
	got := collectSoldOut(req, avail)
	if len(got) != 1 || got[0] != "cabin" {
		t.Fatalf("expected [cabin], got %v", got)
	}
}

func TestCollectSoldOut_UnlimitedAccommodation(t *testing.T) {
	avail := []accommodation.Availability{
		{Code: "tent", Capacity: nil, Taken: 100},
	}
	req := SubmitRequest{Campers: []CamperDTO{
		{Attendance: AttendanceDTO{Type: AttendanceFullWeek, AccommodationCode: "tent"}},
	}}
	got := collectSoldOut(req, avail)
	if len(got) != 0 {
		t.Fatalf("expected no sold-out for unlimited, got %v", got)
	}
}

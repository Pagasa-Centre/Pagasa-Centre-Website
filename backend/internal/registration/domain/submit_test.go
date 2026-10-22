package domain

import (
	"testing"
	"time"
)

func TestSubmission_HasMinor(t *testing.T) {
	if (Submission{Campers: []NewCamper{{Age: 18}}}).HasMinor() {
		t.Fatal("18 not minor")
	}
	if !(Submission{Campers: []NewCamper{{Age: 17}}}).HasMinor() {
		t.Fatal("17 is minor")
	}
}

func TestSubmission_ToGroup_mapsContactAndStatus(t *testing.T) {
	req := Submission{
		Contact: Contact{FirstName: "A", LastName: "B", Email: "a@b.com", Phone: "1"},
		Campers: []NewCamper{{FirstName: "C", LastName: "D", Attendance: Attendance{Type: AttendanceFullWeek}}},
	}
	ts := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	g, campers := req.ToGroup("g1", PaymentPending, 100, "GBP", ts, nil)
	if g.ID != "g1" || g.PaymentStatus != PaymentPending || g.ContactEmail != "a@b.com" {
		t.Fatalf("group %+v", g)
	}
	if len(campers) != 1 || campers[0].AttendanceType != AttendanceFullWeek {
		t.Fatalf("campers %+v", campers)
	}
}

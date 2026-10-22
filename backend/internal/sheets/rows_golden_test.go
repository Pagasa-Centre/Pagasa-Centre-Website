package sheets

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"testing"

	"pagasacentre/backend/internal/notify"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/testhelper"
)

func TestGolden_rowsForGroup(t *testing.T) {
	g := testhelper.SampleGroupFull()
	c := testhelper.SampleCamperFull()
	rows := notify.RowsForGroup(&g, []domain.Camper{c})
	if len(rows) != 1 {
		t.Fatalf("rows = %d", len(rows))
	}
	testhelper.AssertGolden(t, "rows_for_group.csv", encodeRows(t, rows))
}

func TestGolden_rowsFromSubmission_fullWeekPending(t *testing.T) {
	yes := true
	req := domain.Submission{
		Contact: domain.Contact{FirstName: "Jane", LastName: "Doe", Email: "jane@example.com", Phone: "+44123"},
		Campers: []domain.NewCamper{{
			FirstName: "Alex", LastName: "Camper", Age: 25, IsMainContact: true,
			Attendance: domain.Attendance{
				Type: domain.AttendanceFullWeek, ShirtSize: "adult_m",
				AccommodationFirstChoice: "tent", NeedsCoach: &yes,
			},
		}},
	}
	g, campers := req.ToGroup("grp-fw", domain.PaymentPending, 5000, "GBP", testhelper.FixedTime, nil)
	testhelper.AssertGolden(t, "rows_submission_full_week_pending.csv", encodeRows(t, notify.RowsForGroup(&g, campers)))
}

func TestGolden_rowsFromSubmission_dayPassPaid(t *testing.T) {
	req := domain.Submission{
		Contact: domain.Contact{FirstName: "Day", LastName: "Passer", Email: "day@example.com", Phone: "07"},
		Campers: []domain.NewCamper{{
			FirstName: "Day", LastName: "Passer",
			Attendance: domain.Attendance{
				Type: domain.AttendanceDayPass, Days: []string{"mon", "tue"},
				TshirtOption: domain.TshirtOptionTshirtOnly, ShirtSize: "adult_l",
			},
		}},
	}
	paid := testhelper.FixedTime
	g, campers := req.ToGroup("grp-dp", domain.PaymentPaid, 3000, "GBP", testhelper.FixedTime, &paid)
	testhelper.AssertGolden(t, "rows_submission_day_pass_paid.csv", encodeRows(t, notify.RowsForGroup(&g, campers)))
}

func encodeRows(t *testing.T, rows []notify.SheetRow) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	for _, r := range rows {
		line := make([]string, len(Headers))
		vals := values(r)
		for i, v := range vals {
			line[i] = fmt.Sprint(v)
		}
		_ = w.Write(line)
	}
	w.Flush()
	return buf.Bytes()
}

package admin

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
	"time"

	"pagasacentre/backend/internal/registration"
)

var csvHeader = []string{
	"group_id", "payment_status", "paid_at", "total_amount_pence", "currency",
	"contact_first_name", "contact_last_name", "contact_email", "contact_phone",
	"is_main_contact", "first_name", "last_name", "gender", "age",
	"cell_leader_name", "is_cell_leader", "attendance_type",
	"shirt_size", "dietary_requirements", "needs_coach",
	"accommodation_first_choice", "accommodation_second_choice", "roommate_requests",
	"day_pass_days", "day_pass_tshirt_option", "day_pass_needs_catering",
}

// WriteCSV emits a flat CSV — one row per camper — combining group fields and
// camper fields. Groups without campers are skipped.
func WriteCSV(w io.Writer, groups []registration.Group, campersByGroup map[string][]registration.Camper) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()
	if err := cw.Write(csvHeader); err != nil {
		return err
	}
	for _, g := range groups {
		campers := campersByGroup[g.ID]
		for _, c := range campers {
			row := []string{
				g.ID,
				g.PaymentStatus,
				formatTimePtr(g.PaidAt),
				strconv.Itoa(g.TotalAmountPence),
				g.Currency,
				g.ContactFirstName,
				g.ContactLastName,
				g.ContactEmail,
				g.ContactPhone,
				strconv.FormatBool(c.IsMainContact),
				c.FirstName,
				c.LastName,
				c.Gender,
				strconv.Itoa(c.Age),
				c.CellLeaderName,
				strconv.FormatBool(c.IsCellLeader),
				c.AttendanceType,
				deref(c.ShirtSize),
				deref(c.DietaryRequirements),
				formatBoolPtr(c.NeedsCoach),
				deref(c.AccommodationFirstChoice),
				deref(c.AccommodationSecondChoice),
				deref(c.RoommateRequests),
				strings.Join(c.DayPassDays, "|"),
				deref(c.DayPassTshirtOption),
				formatBoolPtr(c.DayPassNeedsCatering),
			}
			if err := cw.Write(row); err != nil {
				return err
			}
		}
	}
	return cw.Error()
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func formatBoolPtr(b *bool) string {
	if b == nil {
		return ""
	}
	return strconv.FormatBool(*b)
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

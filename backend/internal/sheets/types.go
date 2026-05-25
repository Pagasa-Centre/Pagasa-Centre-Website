package sheets

import (
	"strconv"
	"strings"
	"time"
)

// Row is a single camper row written to the Google Sheet. Schema mirrors
// admin/csv.go (one row per camper, group fields denormalised onto each row
// so leaders can filter without joining).
//
// IMPORTANT: keep this struct's field order in sync with Headers + Values
// below — they are positional.
type Row struct {
	GroupID                   string
	PaymentStatus             string
	SubmittedAt               time.Time
	PaidAt                    *time.Time
	TotalAmountPence          int
	Currency                  string
	ContactFirstName          string
	ContactLastName           string
	ContactEmail              string
	ContactPhone              string
	IsMainContact             bool
	FirstName                 string
	LastName                  string
	Gender                    string
	Age                       int
	CellLeaderName            string
	IsCellLeader              bool
	AttendanceType            string
	ShirtSize                 *string
	DietaryRequirements       *string
	NeedsCoach                *bool
	AccommodationFirstChoice  *string
	AccommodationSecondChoice *string
	RoommateRequests          *string
	DayPassDays               []string
	DayPassTshirtOption       *string
	DayPassNeedsCatering      *bool
}

// Headers is the first-row column labels written to the sheet when a tab is
// empty. Must stay in lock-step with Row.Values below.
var Headers = []any{
	"group_id", "payment_status", "submitted_at", "paid_at",
	"total_amount_pence", "currency",
	"contact_first_name", "contact_last_name", "contact_email", "contact_phone",
	"is_main_contact", "first_name", "last_name", "gender", "age",
	"cell_leader_name", "is_cell_leader", "attendance_type",
	"shirt_size", "dietary_requirements", "needs_coach",
	"accommodation_first_choice", "accommodation_second_choice", "roommate_requests",
	"day_pass_days", "day_pass_tshirt_option", "day_pass_needs_catering",
}

// Values returns the row as a flat []any in column order, ready to hand to
// sheets.ValueRange.
func (r Row) Values() []any {
	return []any{
		r.GroupID,
		r.PaymentStatus,
		r.SubmittedAt.UTC().Format(time.RFC3339),
		formatTimePtr(r.PaidAt),
		r.TotalAmountPence,
		r.Currency,
		r.ContactFirstName,
		r.ContactLastName,
		r.ContactEmail,
		r.ContactPhone,
		strconv.FormatBool(r.IsMainContact),
		r.FirstName,
		r.LastName,
		r.Gender,
		r.Age,
		r.CellLeaderName,
		strconv.FormatBool(r.IsCellLeader),
		r.AttendanceType,
		derefString(r.ShirtSize),
		derefString(r.DietaryRequirements),
		formatBoolPtr(r.NeedsCoach),
		derefString(r.AccommodationFirstChoice),
		derefString(r.AccommodationSecondChoice),
		derefString(r.RoommateRequests),
		strings.Join(r.DayPassDays, "|"),
		derefString(r.DayPassTshirtOption),
		formatBoolPtr(r.DayPassNeedsCatering),
	}
}

func derefString(s *string) string {
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

package sheets

import (
	"strconv"
	"strings"
	"time"

	"pagasacentre/backend/internal/notify"
)

func values(r notify.SheetRow) []any {
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

package campadmin

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
	"time"

	"pagasacentre/backend/internal/registration/domain"
)

var csvHeader = []string{
	"group_id", "payment_status", "billing_status", "paid_at", "balance_paid_at",
	"invoice_due_at", "total_amount_pence", "currency", "paid_in_full_at_registration",
	"contact_first_name", "contact_last_name", "contact_email", "contact_phone",
	"is_main_contact", "first_name", "last_name", "gender", "age",
	"cell_leader_name", "is_cell_leader", "attendance_type",
	"shirt_size", "dietary_requirements", "needs_coach",
	"accommodation_first_choice", "accommodation_second_choice", "roommate_requests",
	"allocated_accommodation_code", "billed_stripe_price_id",
	"day_pass_days", "day_pass_tshirt_option", "day_pass_needs_catering",
	"deposit_owed_pence",
}

func WriteCSV(w io.Writer, groups []domain.Group, campersByGroup map[string][]domain.Camper) error {
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
				string(g.PaymentStatus),
				string(g.BillingStatus),
				formatTimePtr(g.PaidAt),
				formatTimePtr(g.BalancePaidAt),
				formatTimePtr(g.InvoiceDueAt),
				strconv.Itoa(g.TotalAmountPence),
				g.Currency,
				strconv.FormatBool(g.PaidInFullAtRegistration),
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
				derefCSV(c.ShirtSize),
				derefCSV(c.DietaryRequirements),
				formatBoolPtrCSV(c.NeedsCoach),
				derefCSV(c.AccommodationFirstChoice),
				derefCSV(c.AccommodationSecondChoice),
				derefCSV(c.RoommateRequests),
				derefCSV(c.AllocatedAccommodationCode),
				derefCSV(c.BilledPriceRef),
				strings.Join(c.DayPassDays, "|"),
				derefCSV(c.DayPassTshirtOption),
				formatBoolPtrCSV(c.DayPassNeedsCatering),
				strconv.Itoa(c.DepositOwedPence),
			}
			if err := cw.Write(row); err != nil {
				return err
			}
		}
	}
	return cw.Error()
}

func derefCSV(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func formatBoolPtrCSV(b *bool) string {
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

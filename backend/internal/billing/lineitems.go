package billing

import (
	"fmt"
	"strings"

	"pagasacentre/backend/internal/email"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/sheets"
)

func SheetRowsForGroup(g *domain.Group, campers []domain.Camper) []sheets.Row {
	rows := make([]sheets.Row, 0, len(campers))
	for _, c := range campers {
		rows = append(rows, sheets.Row{
			GroupID:                   g.ID,
			PaymentStatus:             string(g.PaymentStatus),
			SubmittedAt:               g.CreatedAt,
			PaidAt:                    g.PaidAt,
			TotalAmountPence:          g.TotalAmountPence,
			Currency:                  g.Currency,
			ContactFirstName:          g.ContactFirstName,
			ContactLastName:           g.ContactLastName,
			ContactEmail:              g.ContactEmail,
			ContactPhone:              g.ContactPhone,
			IsMainContact:             c.IsMainContact,
			FirstName:                 c.FirstName,
			LastName:                  c.LastName,
			Gender:                    c.Gender,
			Age:                       c.Age,
			CellLeaderName:            c.CellLeaderName,
			IsCellLeader:              c.IsCellLeader,
			AttendanceType:            c.AttendanceType,
			ShirtSize:                 c.ShirtSize,
			DietaryRequirements:       c.DietaryRequirements,
			NeedsCoach:                c.NeedsCoach,
			AccommodationFirstChoice:  c.AccommodationFirstChoice,
			AccommodationSecondChoice: c.AccommodationSecondChoice,
			RoommateRequests:          c.RoommateRequests,
			DayPassDays:               c.DayPassDays,
			DayPassTshirtOption:       c.DayPassTshirtOption,
			DayPassNeedsCatering:      c.DayPassNeedsCatering,
		})
	}
	return rows
}

func CoachEligibleCount(campers []domain.Camper) int64 {
	var n int64
	for _, c := range campers {
		if c.AttendanceType != domain.AttendanceFullWeek {
			continue
		}
		if c.CoachEligible() {
			n++
		}
	}
	return n
}

func BalanceInvoiceItems(campers []domain.Camper, accNames, unitNames map[string]string, coachCount int64) []string {
	items := RosterItems(campers, accNames, unitNames)
	for _, c := range campers {
		if c.DepositOwedPence > 0 {
			items = append(items, fmt.Sprintf("%s %s — Camp deposit", c.FirstName, c.LastName))
		}
	}
	if coachCount > 0 {
		items = append(items, fmt.Sprintf("Coach transport (×%d)", coachCount))
	}
	return items
}

func RosterItems(campers []domain.Camper, accNames, unitNames map[string]string) []string {
	var items []string
	for _, c := range campers {
		if c.AttendanceType == domain.AttendanceDayPass {
			if n := len(c.DayPassDays); n > 0 {
				unit := "day"
				if n != 1 {
					unit = "days"
				}
				items = append(items, fmt.Sprintf("%s %s — Day pass (%d %s)",
					c.FirstName, c.LastName, n, unit))
			}
			continue
		}
		if c.AttendanceType != domain.AttendanceFullWeek {
			continue
		}
		name := c.FirstName + " " + c.LastName
		if c.AllocatedAccommodationCode != nil {
			code := strings.TrimSpace(*c.AllocatedAccommodationCode)
			if code != "" {
				display := accNames[code]
				if display == "" {
					display = code
				}
				line := name + " — " + display
				if c.AllocatedUnitCode != nil {
					unitCode := strings.TrimSpace(*c.AllocatedUnitCode)
					if unitCode != "" {
						unitDisplay := unitNames[unitCode]
						if unitDisplay == "" {
							unitDisplay = unitCode
						}
						line += " (" + unitDisplay + ")"
					}
				}
				items = append(items, line)
				continue
			}
		}
		items = append(items, name)
	}
	return items
}

func FormatAmountLabel(pence int64, currency string) string {
	if pence <= 0 {
		return ""
	}
	symbol := "£"
	if cur := strings.ToUpper(currency); cur != "GBP" && cur != "" {
		symbol = cur + " "
	}
	return fmt.Sprintf("%s%d.%02d", symbol, pence/100, pence%100)
}

func StrVal(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

func accDisplayName(accNames map[string]string, code string) string {
	if code == "" {
		return ""
	}
	if display := accNames[code]; display != "" {
		return display
	}
	return code
}

func OffPreferenceChanges(campers []domain.Camper, accNames map[string]string) []email.AccommodationChange {
	var out []email.AccommodationChange
	for _, c := range campers {
		if c.AttendanceType != domain.AttendanceFullWeek {
			continue
		}
		alloc := StrVal(c.AllocatedAccommodationCode)
		if alloc == "" {
			continue
		}
		first := StrVal(c.AccommodationFirstChoice)
		second := StrVal(c.AccommodationSecondChoice)
		if first == "" && second == "" {
			continue
		}
		if alloc == first || alloc == second {
			continue
		}
		out = append(out, email.AccommodationChange{
			CamperName:   c.FirstName + " " + c.LastName,
			FirstChoice:  accDisplayName(accNames, first),
			SecondChoice: accDisplayName(accNames, second),
			Allocated:    accDisplayName(accNames, alloc),
			TentGuidance: alloc == domain.AccommodationTent,
		})
	}
	return out
}

package notify

import (
	"strings"

	"pagasacentre/backend/internal/registration/domain"
)

type ItemKind int

const (
	ItemNameOnly ItemKind = iota
	ItemAccommodation
	ItemDayPass
	ItemDeposit
	ItemCoach
)

type InvoiceItem struct {
	Kind              ItemKind
	CamperName        string
	AccommodationName string
	UnitName          string
	Days              int
	Count             int64
}

func CamperNames(campers []domain.Camper) []string {
	var names []string
	for _, c := range campers {
		names = append(names, c.FirstName+" "+c.LastName)
	}
	return names
}

func RosterItems(campers []domain.Camper, accNames, unitNames map[string]string) []InvoiceItem {
	var items []InvoiceItem
	for _, c := range campers {
		if c.AttendanceType == domain.AttendanceDayPass {
			if n := len(c.DayPassDays); n > 0 {
				items = append(items, InvoiceItem{
					Kind:       ItemDayPass,
					CamperName: c.FirstName + " " + c.LastName,
					Days:       n,
				})
			}
			continue
		}
		if c.AttendanceType != domain.AttendanceFullWeek {
			continue
		}
		name := c.FirstName + " " + c.LastName
		if c.AllocatedAccommodationCode != nil {
			code := strVal(c.AllocatedAccommodationCode)
			if code != "" {
				display := accNames[code]
				if display == "" {
					display = code
				}
				item := InvoiceItem{
					Kind:              ItemAccommodation,
					CamperName:        name,
					AccommodationName: display,
				}
				if c.AllocatedUnitCode != nil {
					unitCode := strVal(c.AllocatedUnitCode)
					if unitCode != "" {
						unitDisplay := unitNames[unitCode]
						if unitDisplay == "" {
							unitDisplay = unitCode
						}
						item.UnitName = unitDisplay
					}
				}
				items = append(items, item)
				continue
			}
		}
		items = append(items, InvoiceItem{Kind: ItemNameOnly, CamperName: name})
	}
	return items
}

func BalanceItems(campers []domain.Camper, accNames, unitNames map[string]string, coachCount int64) []InvoiceItem {
	items := RosterItems(campers, accNames, unitNames)
	for _, c := range campers {
		if c.DepositOwedPence > 0 {
			items = append(items, InvoiceItem{
				Kind:       ItemDeposit,
				CamperName: c.FirstName + " " + c.LastName,
			})
		}
	}
	if coachCount > 0 {
		items = append(items, InvoiceItem{Kind: ItemCoach, Count: coachCount})
	}
	return items
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

func OffPreferenceChanges(campers []domain.Camper, accNames map[string]string) []AccommodationChange {
	var out []AccommodationChange
	for _, c := range campers {
		first, second, alloc, ok := c.OffPreference()
		if !ok {
			continue
		}
		out = append(out, AccommodationChange{
			CamperName:   c.FirstName + " " + c.LastName,
			FirstChoice:  accDisplayName(accNames, first),
			SecondChoice: accDisplayName(accNames, second),
			Allocated:    accDisplayName(accNames, alloc),
			TentGuidance: alloc == domain.AccommodationTent,
		})
	}
	return out
}

func strVal(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

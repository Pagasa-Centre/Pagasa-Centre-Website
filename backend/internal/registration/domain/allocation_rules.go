package domain

import (
	"fmt"
	"strings"

	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

func AllocationTarget(campers []Camper, camperID, code string) (c Camper, skip bool, err error) {
	for _, camper := range campers {
		if camper.ID == camperID {
			c = camper
			break
		}
	}
	if c.ID == "" {
		return Camper{}, false, apperr.Invalid("unknown camper_id " + camperID)
	}
	if c.AttendanceType != AttendanceFullWeek {
		return c, true, nil
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return Camper{}, false, apperr.Validation(map[string]string{
			camperID: "allocated_accommodation_code is required for full-week campers",
		})
	}
	return c, false, nil
}

func (g *Group) AllocationPriceRef(requested string) (ref string, needsLookup bool) {
	if g.IsFree {
		return "", false
	}
	return strings.TrimSpace(requested), strings.TrimSpace(requested) == ""
}

func CheckAllCampersAllocated(campers []Camper, allocs []CamperAllocation) error {
	for _, c := range campers {
		if c.AttendanceType != AttendanceFullWeek {
			continue
		}
		found := false
		for _, a := range allocs {
			if a.CamperID == c.ID {
				found = true
				break
			}
		}
		if !found {
			return apperr.Validation(map[string]string{
				"campers": fmt.Sprintf("full-week camper %s %s is not allocated", c.FirstName, c.LastName),
			})
		}
	}
	return nil
}

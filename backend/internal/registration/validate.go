package registration

import (
	"fmt"
	"strings"

	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

func Validate(req domain.Submission) error {
	fields := map[string]string{}

	if strings.TrimSpace(req.Contact.FirstName) == "" {
		fields["contact.first_name"] = "is required"
	}
	if strings.TrimSpace(req.Contact.LastName) == "" {
		fields["contact.last_name"] = "is required"
	}
	if !domain.ValidEmail(req.Contact.Email) {
		fields["contact.email"] = "must be a valid email"
	}
	if strings.TrimSpace(req.Contact.Phone) == "" {
		fields["contact.phone"] = "is required"
	}

	if len(req.Campers) == 0 {
		fields["campers"] = "at least one camper is required"
	}

	mainContactCount := 0
	for i, c := range req.Campers {
		prefix := fmt.Sprintf("campers[%d]", i)
		if c.IsMainContact {
			mainContactCount++
		}
		domain.ValidateCamper(prefix, c, fields)
	}

	if len(req.Campers) > 0 && mainContactCount != 1 {
		fields["campers"] = "exactly one camper must be marked as the main contact"
	}

	if len(fields) > 0 {
		return apperr.Validation(fields)
	}
	return nil
}

func HasMinor(req domain.Submission) bool {
	for _, c := range req.Campers {
		if c.Age < 18 {
			return true
		}
	}
	return false
}

// Re-export shirt helpers for API handlers.
func ListShirtSizes() []domain.ShirtSize { return domain.ListShirtSizes() }

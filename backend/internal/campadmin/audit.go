package campadmin

import (
	"context"
	"fmt"
	"log"

	"pagasacentre/backend/internal/billing/invoicing"
	"pagasacentre/backend/internal/registration/domain"
)

func (s *Service) record(ctx context.Context, actor, action string, groupID *string, summary string, meta any) {
	if s == nil || s.rec == nil {
		return
	}
	if _, err := s.rec.Record(ctx, actor, action, groupID, summary, meta); err != nil {
		log.Printf("admin audit: %v", err)
	}
}

func groupSummary(g *domain.Group) string {
	if g == nil {
		return "a registration group"
	}
	return g.ContactFirstName + " " + g.ContactLastName
}

func formatPence(pence int) string {
	return fmt.Sprintf("£%d.%02d", pence/100, pence%100)
}

// formatDeleteAuditSummary writes the one line that outlives a deleted registration.
func formatDeleteAuditSummary(sum invoicing.DeleteSummary) string {
	text := "Deleted registration for " + sum.ContactName + " (" + sum.ContactEmail + ")"
	if sum.RetainedPence > 0 {
		text += fmt.Sprintf("; kept the %s non-refundable deposit",
			formatPence(int(sum.RetainedPence)))
	}
	if sum.AmountPence > 0 {
		text += fmt.Sprintf("; refunded %s", formatPence(int(sum.AmountPence)))
	}
	if sum.InvoiceVoided {
		text += "; voided open invoice"
	}
	return text
}

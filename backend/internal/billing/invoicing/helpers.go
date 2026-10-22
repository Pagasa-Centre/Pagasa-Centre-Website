package invoicing

import (
	"time"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/notify"
	"pagasacentre/backend/internal/registration/domain"
)

func notifyAmount(pence int64, currency string) notify.Amount {
	return notify.Amount{Pence: pence, Currency: currency}
}

func dueAtRef(t time.Time) *time.Time {
	return &t
}

func resendDueAt(g *domain.Group, res billing.InvoiceResult) *time.Time {
	if g.InvoiceDueAt != nil {
		t := *g.InvoiceDueAt
		return &t
	}
	if !res.DueAt.IsZero() {
		t := res.DueAt
		return &t
	}
	return nil
}

package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/registration/storage"
	"pagasacentre/backend/internal/testhelper"
)

func TestSetInvoiceDetailsMeta_billingCASMismatch(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	reads := storage.NewRepository(pool)
	repo := NewRepository(pool)

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status, version
		) VALUES ('T', 'U', 't@example.com', '07000000000', 'paid', 1000, 'GBP', 'none', 1)
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	tr := domain.BillingTransition{From: domain.BillingAllocated, To: domain.BillingInvoiced}
	meta := domain.ActionMeta{
		Actor:           "Admin",
		Action:          "invoice_sent",
		ExpectedVersion: domain.SkipVersionCheck,
	}
	err = repo.SetInvoiceDetailsMeta(ctx, groupID, "in_test", time.Now().UTC(), false, meta, tr)
	if !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict, got %v", err)
	}

	g, err := reads.FindGroupByID(ctx, groupID)
	if err != nil || g == nil {
		t.Fatalf("FindGroupByID: %v", err)
	}
	if g.BillingStatus != domain.BillingNone {
		t.Fatalf("billing_status = %q, want none", g.BillingStatus)
	}
}

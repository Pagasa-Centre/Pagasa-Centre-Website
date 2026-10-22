package storage

import (
	"context"
	"errors"
	"testing"

	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/testhelper"
)

func TestApplySettlement_paymentCASMismatch(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
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

	s := domain.Settlement{
		Payment: domain.PaymentTransition{From: domain.PaymentPending, To: domain.PaymentPaid},
	}
	err = repo.ApplySettlement(ctx, groupID, "pi_x", false, s)
	if !errors.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict, got %v", err)
	}
}

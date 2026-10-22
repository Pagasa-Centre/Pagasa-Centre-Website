package reconcile_test

import (
	"context"
	"testing"
	"time"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/billing/billingtest"
	invstorage "pagasacentre/backend/internal/billing/invoicing/storage"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/testhelper"
)

func TestHandleInvoicePaid_emailsFamily(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	mailer := &billingtest.RecordingMailer{}
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, mailer, nil, cfg)
	repo := env.Repo

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status, stripe_invoice_id
		) VALUES ('Pay', 'Family', 'family@example.com', '07000000000', 'paid', 25000, 'GBP', 'invoiced', 'in_test_123')
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	if err := env.Reconcile.HandleInvoicePaid(ctx, "in_test_123", groupID, 25000, "GBP"); err != nil {
		t.Fatalf("HandleInvoicePaid: %v", err)
	}
	g, err := repo.FindGroupByID(ctx, groupID)
	if err != nil || g == nil {
		t.Fatalf("find group: %v", err)
	}
	if g.BillingStatus != domain.BillingBalancePaid {
		t.Fatalf("billing_status = %q, want balance_paid", g.BillingStatus)
	}
	if len(mailer.BalancePaid) != 1 {
		t.Fatalf("expected 1 balance-paid confirmation email, got %d", len(mailer.BalancePaid))
	}
	if mailer.BalancePaid[0].ToEmail != "family@example.com" {
		t.Fatalf("email sent to %q, want family@example.com", mailer.BalancePaid[0].ToEmail)
	}
}

func TestHandleCoachInvoicePaid(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)
	repo := env.Repo

	groupID := billingtest.InsertCoachGroup(ctx, t, pool, domain.BillingBalancePaid, "lodge", false)
	if err := invstorage.NewRepository(pool).SetCoachInvoiceMeta(ctx, groupID, "in_coach_paid", time.Now().Add(15*24*time.Hour), domain.ActionMeta{
		Actor:           "Admin",
		Action:          "coach_invoice_sent",
		ExpectedVersion: domain.SkipVersionCheck,
	}); err != nil {
		t.Fatalf("seed coach invoice: %v", err)
	}

	if err := env.Reconcile.HandleCoachInvoicePaid(ctx, "in_coach_paid", groupID); err != nil {
		t.Fatalf("HandleCoachInvoicePaid: %v", err)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil || g.CoachFeePaidAt == nil {
		t.Fatalf("coach_fee_paid_at not set: %+v", g)
	}
	// Idempotent on repeat.
	if err := env.Reconcile.HandleCoachInvoicePaid(ctx, "in_coach_paid", groupID); err != nil {
		t.Fatalf("HandleCoachInvoicePaid repeat: %v", err)
	}
}

package allocation_test

import (
	"context"
	"errors"
	"testing"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/billing/allocation"
	"pagasacentre/backend/internal/billing/billingtest"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/testhelper"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

func strPtr(s string) *string { return &s }

func TestAllocate_persistsUnit(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{StripePriceChildUnder3: "price_child"}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)
	repo := env.Repo
	// Allocation auto-resolves the tier's Stripe price, so lodge must have one.
	if err := repo.UpdateAccommodationStripePrice(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("seed lodge price: %v", err)
	}

	// Seed a paid group with one full-week camper via raw SQL.
	var groupID, camperID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status
		) VALUES ('Test', 'Family', 'test@example.com', '07000000000', 'paid', 5000, 'GBP', 'none')
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type
		) VALUES ($1, true, 'Alex', 'Test', 'male', 25, 'Leader', false, 'full_week')
		RETURNING id`, groupID).Scan(&camperID)
	if err != nil {
		t.Fatalf("insert camper: %v", err)
	}

	if err := env.Allocation.Allocate(ctx, groupID, "Test Admin", domain.SkipVersionCheck, allocation.AllocateRequest{
		Campers: []allocation.AllocateCamper{{
			CamperID:                   camperID,
			AllocatedAccommodationCode: "lodge",
			AllocatedUnitCode:          "lodge_1",
		}},
	}); err != nil {
		t.Fatalf("allocate: %v", err)
	}

	campers, err := repo.CampersForGroup(ctx, groupID)
	if err != nil {
		t.Fatalf("campers: %v", err)
	}
	if len(campers) != 1 {
		t.Fatalf("campers len = %d", len(campers))
	}
	c := campers[0]
	if c.AllocatedAccommodationCode == nil || *c.AllocatedAccommodationCode != "lodge" {
		t.Fatalf("tier = %v, want lodge", c.AllocatedAccommodationCode)
	}
	if c.AllocatedUnitCode == nil || *c.AllocatedUnitCode != "lodge_1" {
		t.Fatalf("unit = %v, want lodge_1", c.AllocatedUnitCode)
	}
}

func TestAllocate_versionConflict(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{StripePriceChildUnder3: "price_child"}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)
	repo := env.Repo
	if err := repo.UpdateAccommodationStripePrice(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("seed lodge price: %v", err)
	}

	var groupID, camperID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status, version
		) VALUES ('Test', 'Family', 'test@example.com', '07000000000', 'paid', 5000, 'GBP', 'none', 3)
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type
		) VALUES ($1, true, 'Alex', 'Test', 'male', 25, 'Leader', false, 'full_week')
		RETURNING id`, groupID).Scan(&camperID)
	if err != nil {
		t.Fatalf("insert camper: %v", err)
	}

	req := allocation.AllocateRequest{Campers: []allocation.AllocateCamper{{
		CamperID:                   camperID,
		AllocatedAccommodationCode: "lodge",
	}}}
	// Stale version should 409-style conflict.
	err = env.Allocation.Allocate(ctx, groupID, "Aliyah", 1, req)
	var de *apperr.Error
	if !errors.As(err, &de) || de.Kind != apperr.KindConflict || de.Code != "stale_state" {
		t.Fatalf("expected stale_state conflict, got %v", err)
	}
}

func TestConfirmFree_allocatedFreeGroup(t *testing.T) {
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
			payment_status, total_amount_pence, currency, billing_status, is_free
		) VALUES ('Free', 'Guest', 'free@example.com', '07000000000', 'paid', 0, 'GBP', 'allocated', true)
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	if err := env.Allocation.ConfirmFree(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("ConfirmFree: %v", err)
	}
	g, err := repo.FindGroupByID(ctx, groupID)
	if err != nil || g == nil {
		t.Fatalf("find group: %v", err)
	}
	if g.BillingStatus != domain.BillingFreeConfirmed {
		t.Fatalf("billing_status = %q, want free_confirmed", g.BillingStatus)
	}
	if len(mailer.Sponsorships) != 1 {
		t.Fatalf("expected 1 sponsorship-confirmed email, got %d", len(mailer.Sponsorships))
	}
	if mailer.Sponsorships[0].ToEmail != "free@example.com" {
		t.Fatalf("email sent to %q, want free@example.com", mailer.Sponsorships[0].ToEmail)
	}
}

func TestConfirmFree_rejectsNonFreeGroup(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status, is_free
		) VALUES ('Pay', 'Guest', 'pay@example.com', '07000000000', 'paid', 5000, 'GBP', 'allocated', false)
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	err = env.Allocation.ConfirmFree(ctx, groupID, "Admin", domain.SkipVersionCheck)
	var de *apperr.Error
	if !errors.As(err, &de) || de.Kind != apperr.KindInvalid {
		t.Fatalf("expected bad_request, got %v", err)
	}
}

func TestConfirmFree_rejectsNonAllocatedGroup(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status, is_free
		) VALUES ('Free', 'Guest', 'free-none@example.com', '07000000000', 'paid', 0, 'GBP', 'none', true)
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	err = env.Allocation.ConfirmFree(ctx, groupID, "Admin", domain.SkipVersionCheck)
	var de *apperr.Error
	if !errors.As(err, &de) || de.Kind != apperr.KindInvalid {
		t.Fatalf("expected bad_request for non-allocated free group, got %v", err)
	}
}

func TestAllocate_prepaidGroupKeepsBalancePaid(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)
	repo := env.Repo
	if err := repo.UpdateAccommodationStripePrice(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("seed lodge price: %v", err)
	}

	var groupID, camperID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status, paid_in_full_at_registration
		) VALUES ('Pre', 'Paid', 'prepaid@example.com', '07000000000', 'paid', 70000, 'GBP', 'balance_paid', true)
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type,
			accommodation_first_choice
		) VALUES ($1, true, 'Alex', 'Test', 'male', 25, 'Leader', false, 'full_week', 'lodge')
		RETURNING id`, groupID).Scan(&camperID)
	if err != nil {
		t.Fatalf("insert camper: %v", err)
	}

	if err := env.Allocation.Allocate(ctx, groupID, "Admin", domain.SkipVersionCheck, allocation.AllocateRequest{
		Campers: []allocation.AllocateCamper{{
			CamperID:                   camperID,
			AllocatedAccommodationCode: "lodge",
			BilledStripePriceID:        "price_lodge",
		}},
	}); err != nil {
		t.Fatalf("Allocate prepaid: %v", err)
	}

	g, err := repo.FindGroupByID(ctx, groupID)
	if err != nil || g == nil {
		t.Fatalf("find group: %v", err)
	}
	if g.BillingStatus != domain.BillingBalancePaid {
		t.Fatalf("billing_status = %q, want balance_paid", g.BillingStatus)
	}
}

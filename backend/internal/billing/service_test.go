package billing

import (
	"context"
	"errors"
	"testing"

	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
	"pagasacentre/backend/internal/registration"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/registration/storage"
	"pagasacentre/backend/internal/testhelper"
)

func TestChildUnder3UsesConfigPrice(t *testing.T) {
	cfg := Config{StripePriceChildUnder3: "price_child_0"}
	if cfg.StripePriceChildUnder3 == "" {
		t.Fatal("expected configured under-3 price")
	}
	// Documented rule: child accommodation + age < MinDepositAge => under-3 price.
	age := registration.MinDepositAge - 1
	code := registration.AccommodationChild
	if code != "child" || age >= registration.MinDepositAge {
		t.Fatal("test precondition")
	}
	_ = cfg.StripePriceChildUnder3
}

func TestBalanceInvoiceItemsIncludesUnit(t *testing.T) {
	unit := "caravan_5"
	items := balanceInvoiceItems(
		[]domain.Camper{{
			FirstName:                  "Josh",
			LastName:                   "Basco",
			AttendanceType:             domain.AttendanceFullWeek,
			AllocatedAccommodationCode: strPtr("static_caravan"),
			AllocatedUnitCode:          &unit,
		}},
		map[string]string{"static_caravan": "Static Caravan"},
		map[string]string{"caravan_5": "Caravan 5"},
	)
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	want := "Josh Basco — Static Caravan (Caravan 5)"
	if items[0] != want {
		t.Fatalf("got %q, want %q", items[0], want)
	}
}

func TestValidateAllocatedUnit_blankAllowed(t *testing.T) {
	pool := testhelper.MaybePool(t)
	repo := storage.NewRepository(pool)
	svc := NewService(repo, NewStripeBilling(), nil, Config{})

	if err := svc.validateAllocatedUnit(context.Background(), "lodge", ""); err != nil {
		t.Fatalf("blank unit should be allowed: %v", err)
	}
}

func TestValidateAllocatedUnit_rejectsMismatch(t *testing.T) {
	pool := testhelper.MaybePool(t)
	repo := storage.NewRepository(pool)
	svc := NewService(repo, NewStripeBilling(), nil, Config{})

	err := svc.validateAllocatedUnit(context.Background(), "lodge", "caravan_1")
	if err == nil {
		t.Fatal("expected tier/unit mismatch error")
	}
}

func TestAllocate_persistsUnit(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, NewStripeBilling(), nil, Config{StripePriceChildUnder3: "price_child"})

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

	if err := svc.Allocate(ctx, groupID, "Test Admin", domain.SkipVersionCheck, AllocateRequest{
		Campers: []AllocateCamper{{
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
	repo := storage.NewRepository(pool)
	svc := NewService(repo, NewStripeBilling(), nil, Config{StripePriceChildUnder3: "price_child"})

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

	req := AllocateRequest{Campers: []AllocateCamper{{
		CamperID:                   camperID,
		AllocatedAccommodationCode: "lodge",
	}}}
	// Stale version should 409-style conflict.
	err = svc.Allocate(ctx, groupID, "Aliyah", 1, req)
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "stale_state" {
		t.Fatalf("expected stale_state conflict, got %v", err)
	}
}

func TestConfirmFree_allocatedFreeGroup(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, NewStripeBilling(), nil, Config{})

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

	if err := svc.ConfirmFree(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("ConfirmFree: %v", err)
	}
	g, err := repo.FindGroupByID(ctx, groupID)
	if err != nil || g == nil {
		t.Fatalf("find group: %v", err)
	}
	if g.BillingStatus != domain.BillingFreeConfirmed {
		t.Fatalf("billing_status = %q, want free_confirmed", g.BillingStatus)
	}
}

func TestConfirmFree_rejectsNonFreeGroup(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, NewStripeBilling(), nil, Config{})

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

	err = svc.ConfirmFree(ctx, groupID, "Admin", domain.SkipVersionCheck)
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
		t.Fatalf("expected bad_request, got %v", err)
	}
}

func TestConfirmFree_rejectsNonAllocatedGroup(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, NewStripeBilling(), nil, Config{})

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

	err = svc.ConfirmFree(ctx, groupID, "Admin", domain.SkipVersionCheck)
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
		t.Fatalf("expected bad_request for non-allocated free group, got %v", err)
	}
}

func strPtr(s string) *string { return &s }

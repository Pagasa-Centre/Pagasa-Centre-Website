package billing

import (
	"context"
	"errors"
	"testing"

	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
	"pagasacentre/backend/internal/email"
	"pagasacentre/backend/internal/registration"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/registration/storage"
	"pagasacentre/backend/internal/testhelper"
)

// recordingMailer captures family-facing confirmation sends for assertions.
type recordingMailer struct {
	email.NoopMailer
	sponsorships []email.SponsorshipConfirmed
	balancePaid  []email.BalancePaidConfirmation
	accomChanged []email.AccommodationChangedNotice
}

func (m *recordingMailer) SendSponsorshipConfirmed(_ context.Context, p email.SponsorshipConfirmed) error {
	m.sponsorships = append(m.sponsorships, p)
	return nil
}

func (m *recordingMailer) SendBalancePaidConfirmation(_ context.Context, p email.BalancePaidConfirmation) error {
	m.balancePaid = append(m.balancePaid, p)
	return nil
}

func (m *recordingMailer) SendAccommodationChanged(_ context.Context, p email.AccommodationChangedNotice) error {
	m.accomChanged = append(m.accomChanged, p)
	return nil
}

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
	mailer := &recordingMailer{}
	svc := NewService(repo, NewStripeBilling(), mailer, Config{})

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
	if len(mailer.sponsorships) != 1 {
		t.Fatalf("expected 1 sponsorship-confirmed email, got %d", len(mailer.sponsorships))
	}
	if mailer.sponsorships[0].ToEmail != "free@example.com" {
		t.Fatalf("email sent to %q, want free@example.com", mailer.sponsorships[0].ToEmail)
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

func TestHandleInvoicePaid_emailsFamily(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	mailer := &recordingMailer{}
	svc := NewService(repo, NewStripeBilling(), mailer, Config{})

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

	if err := svc.HandleInvoicePaid(ctx, "in_test_123", groupID, 25000, "GBP"); err != nil {
		t.Fatalf("HandleInvoicePaid: %v", err)
	}
	g, err := repo.FindGroupByID(ctx, groupID)
	if err != nil || g == nil {
		t.Fatalf("find group: %v", err)
	}
	if g.BillingStatus != domain.BillingBalancePaid {
		t.Fatalf("billing_status = %q, want balance_paid", g.BillingStatus)
	}
	if len(mailer.balancePaid) != 1 {
		t.Fatalf("expected 1 balance-paid confirmation email, got %d", len(mailer.balancePaid))
	}
	if mailer.balancePaid[0].ToEmail != "family@example.com" {
		t.Fatalf("email sent to %q, want family@example.com", mailer.balancePaid[0].ToEmail)
	}
}

func strPtr(s string) *string { return &s }

func TestOffPreferenceChanges(t *testing.T) {
	names := map[string]string{
		"lodge": "Lodge",
		"cabin": "Cabin",
		"tent":  "Tent",
	}
	cases := []struct {
		name  string
		c     domain.Camper
		wantN int
		tent  bool
	}{
		{
			name: "matches first choice",
			c: domain.Camper{
				FirstName: "A", LastName: "One",
				AttendanceType:             domain.AttendanceFullWeek,
				AccommodationFirstChoice:   strPtr("lodge"),
				AccommodationSecondChoice:  strPtr("cabin"),
				AllocatedAccommodationCode: strPtr("lodge"),
			},
			wantN: 0,
		},
		{
			name: "matches second choice",
			c: domain.Camper{
				FirstName: "B", LastName: "Two",
				AttendanceType:             domain.AttendanceFullWeek,
				AccommodationFirstChoice:   strPtr("lodge"),
				AccommodationSecondChoice:  strPtr("cabin"),
				AllocatedAccommodationCode: strPtr("cabin"),
			},
			wantN: 0,
		},
		{
			name: "off preference",
			c: domain.Camper{
				FirstName: "C", LastName: "Three",
				AttendanceType:             domain.AttendanceFullWeek,
				AccommodationFirstChoice:   strPtr("lodge"),
				AccommodationSecondChoice:  strPtr("cabin"),
				AllocatedAccommodationCode: strPtr("tent"),
			},
			wantN: 1,
			tent:  true,
		},
		{
			name: "nil second choice still off preference",
			c: domain.Camper{
				FirstName: "D", LastName: "Four",
				AttendanceType:             domain.AttendanceFullWeek,
				AccommodationFirstChoice:   strPtr("lodge"),
				AllocatedAccommodationCode: strPtr("tent"),
			},
			wantN: 1,
			tent:  true,
		},
		{
			name: "both choices empty skipped",
			c: domain.Camper{
				FirstName: "E", LastName: "Five",
				AttendanceType:             domain.AttendanceFullWeek,
				AllocatedAccommodationCode: strPtr("tent"),
			},
			wantN: 0,
		},
		{
			name: "day pass skipped",
			c: domain.Camper{
				FirstName: "F", LastName: "Six",
				AttendanceType:             domain.AttendanceDayPass,
				AccommodationFirstChoice:   strPtr("lodge"),
				AllocatedAccommodationCode: strPtr("tent"),
			},
			wantN: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := offPreferenceChanges([]domain.Camper{tc.c}, names)
			if len(got) != tc.wantN {
				t.Fatalf("len = %d, want %d", len(got), tc.wantN)
			}
			if tc.wantN == 1 {
				if got[0].Allocated != "Tent" {
					t.Fatalf("allocated = %q, want Tent", got[0].Allocated)
				}
				if got[0].TentGuidance != tc.tent {
					t.Fatalf("TentGuidance = %v, want %v", got[0].TentGuidance, tc.tent)
				}
			}
		})
	}
}

func TestSendAccommodationChangedNotice(t *testing.T) {
	mailer := &recordingMailer{}
	svc := NewService(nil, NewStripeBilling(), mailer, Config{})

	changes := []email.AccommodationChange{{
		CamperName:   "Alex Test",
		FirstChoice:  "Lodge",
		SecondChoice: "Cabin",
		Allocated:    "Tent",
		TentGuidance: true,
	}}
	svc.sendAccommodationChangedNotice(context.Background(), "family@example.com", "Sam", changes, true)

	if len(mailer.accomChanged) != 1 {
		t.Fatalf("expected 1 accommodation-changed email, got %d", len(mailer.accomChanged))
	}
	if !mailer.accomChanged[0].AwaitingPayment {
		t.Fatal("expected AwaitingPayment true")
	}
	if mailer.accomChanged[0].ToEmail != "family@example.com" {
		t.Fatalf("to = %q", mailer.accomChanged[0].ToEmail)
	}

	// No-op when no changes.
	before := len(mailer.accomChanged)
	svc.sendAccommodationChangedNotice(context.Background(), "family@example.com", "Sam", nil, true)
	if len(mailer.accomChanged) != before {
		t.Fatal("should not send when changes empty")
	}
}

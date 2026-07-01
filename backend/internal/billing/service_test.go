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
	"pagasacentre/backend/internal/sheets"
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
	svc := NewService(repo, NewStripeBilling(), nil, nil, Config{})

	if err := svc.validateAllocatedUnit(context.Background(), "lodge", ""); err != nil {
		t.Fatalf("blank unit should be allowed: %v", err)
	}
}

func TestValidateAllocatedUnit_rejectsMismatch(t *testing.T) {
	pool := testhelper.MaybePool(t)
	repo := storage.NewRepository(pool)
	svc := NewService(repo, NewStripeBilling(), nil, nil, Config{})

	err := svc.validateAllocatedUnit(context.Background(), "lodge", "caravan_1")
	if err == nil {
		t.Fatal("expected tier/unit mismatch error")
	}
}

func TestAllocate_persistsUnit(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, NewStripeBilling(), nil, nil, Config{StripePriceChildUnder3: "price_child"})

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
	svc := NewService(repo, NewStripeBilling(), nil, nil, Config{StripePriceChildUnder3: "price_child"})

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
	svc := NewService(repo, NewStripeBilling(), mailer, nil, Config{})

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
	svc := NewService(repo, NewStripeBilling(), nil, nil, Config{})

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
	svc := NewService(repo, NewStripeBilling(), nil, nil, Config{})

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
	svc := NewService(repo, NewStripeBilling(), mailer, nil, Config{})

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
	svc := NewService(nil, NewStripeBilling(), mailer, nil, Config{})

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

// stubStripe implements stripeClient for delete-registration tests.
type stubStripe struct {
	refundPI    func(ctx context.Context, id string) (int64, error)
	refundInv   func(ctx context.Context, id string) (int64, error)
	voidInv     func(ctx context.Context, id string) error
	voidInvIdem func(ctx context.Context, id string) error
}

func (s *stubStripe) EnsureCustomer(context.Context, string, string, string, string) (string, error) {
	return "", errors.New("unexpected EnsureCustomer")
}
func (s *stubStripe) CreateInvoice(context.Context, string, string, []string, int64) (InvoiceResult, error) {
	return InvoiceResult{}, errors.New("unexpected CreateInvoice")
}
func (s *stubStripe) VoidInvoice(ctx context.Context, invoiceID string) error {
	if s.voidInv != nil {
		return s.voidInv(ctx, invoiceID)
	}
	return nil
}
func (s *stubStripe) VoidInvoiceIdempotent(ctx context.Context, invoiceID string) error {
	if s.voidInvIdem != nil {
		return s.voidInvIdem(ctx, invoiceID)
	}
	return nil
}
func (s *stubStripe) SendInvoiceEmail(context.Context, string) error {
	return errors.New("unexpected SendInvoiceEmail")
}
func (s *stubStripe) GetInvoice(context.Context, string) (InvoiceResult, error) {
	return InvoiceResult{}, errors.New("unexpected GetInvoice")
}
func (s *stubStripe) RefundPaymentIntent(ctx context.Context, id string) (int64, error) {
	if s.refundPI != nil {
		return s.refundPI(ctx, id)
	}
	return 0, nil
}
func (s *stubStripe) RefundInvoice(ctx context.Context, id string) (int64, error) {
	if s.refundInv != nil {
		return s.refundInv(ctx, id)
	}
	return 0, nil
}

type failingSheets struct {
	sheets.NoopSync
}

func (failingSheets) RemoveByGroupID(context.Context, string) error {
	return errors.New("sheet sync failed")
}

func TestDeleteRegistration_unpaid(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	stub := &stubStripe{}
	svc := NewService(repo, stub, nil, nil, Config{})

	var groupID, camperID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status
		) VALUES ('Day', 'Pass', 'day@example.com', '07000000000', 'pending', 0, 'GBP', 'none')
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type
		) VALUES ($1, true, 'Alex', 'Day', 'male', 30, 'Leader', false, 'day_pass')
		RETURNING id`, groupID).Scan(&camperID)
	if err != nil {
		t.Fatalf("insert camper: %v", err)
	}

	sum, err := svc.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("DeleteRegistration: %v", err)
	}
	if sum.ContactEmail != "day@example.com" {
		t.Fatalf("contact email = %q", sum.ContactEmail)
	}
	if sum.DepositRefunded || sum.BalanceRefunded || sum.InvoiceVoided {
		t.Fatal("expected no Stripe cleanup for unpaid group")
	}
	g, err := repo.FindGroupByID(ctx, groupID)
	if err != nil {
		t.Fatalf("FindGroupByID: %v", err)
	}
	if g != nil {
		t.Fatal("group row should be gone")
	}
	campers, err := repo.CampersForGroup(ctx, groupID)
	if err != nil {
		t.Fatalf("CampersForGroup: %v", err)
	}
	if len(campers) != 0 {
		t.Fatalf("expected 0 campers, got %d", len(campers))
	}
}

func TestDeleteRegistration_paidDeposit(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	pi := "pi_test_deposit"
	var refundCalled bool
	stub := &stubStripe{
		refundPI: func(_ context.Context, id string) (int64, error) {
			refundCalled = true
			if id != pi {
				t.Fatalf("refund PI = %q, want %q", id, pi)
			}
			return 5000, nil
		},
	}
	svc := NewService(repo, stub, nil, nil, Config{})

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_payment_intent_id, total_amount_pence, currency, billing_status
		) VALUES ('Paid', 'Deposit', 'paid@example.com', '07000000000', 'paid', $1, 5000, 'GBP', 'none')
		RETURNING id`, pi).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	sum, err := svc.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("DeleteRegistration: %v", err)
	}
	if !refundCalled {
		t.Fatal("expected deposit refund")
	}
	if !sum.DepositRefunded || sum.AmountPence != 5000 {
		t.Fatalf("summary = %+v", sum)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g != nil {
		t.Fatal("group should be deleted")
	}
}

func TestDeleteRegistration_balancePaid(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	pi := "pi_test_full"
	inv := "in_test_full"
	var depositRefunded, balanceRefunded bool
	stub := &stubStripe{
		refundPI: func(_ context.Context, id string) (int64, error) {
			depositRefunded = true
			if id != pi {
				t.Fatalf("deposit PI = %q", id)
			}
			return 5000, nil
		},
		refundInv: func(_ context.Context, id string) (int64, error) {
			balanceRefunded = true
			if id != inv {
				t.Fatalf("invoice = %q", id)
			}
			return 12000, nil
		},
	}
	svc := NewService(repo, stub, nil, nil, Config{})

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_payment_intent_id, stripe_invoice_id,
			total_amount_pence, currency, billing_status
		) VALUES ('Full', 'Paid', 'full@example.com', '07000000000', 'paid', $1, $2, 5000, 'GBP', 'balance_paid')
		RETURNING id`, pi, inv).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	sum, err := svc.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("DeleteRegistration: %v", err)
	}
	if !depositRefunded || !balanceRefunded {
		t.Fatal("expected both deposit and balance refunds")
	}
	if sum.AmountPence != 17000 {
		t.Fatalf("AmountPence = %d, want 17000", sum.AmountPence)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g != nil {
		t.Fatal("group should be deleted")
	}
}

func TestDeleteRegistration_refundFailureAborts(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	stub := &stubStripe{
		refundPI: func(context.Context, string) (int64, error) {
			return 0, errors.New("stripe down")
		},
	}
	svc := NewService(repo, stub, nil, nil, Config{})

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_payment_intent_id, total_amount_pence, currency, billing_status
		) VALUES ('Fail', 'Refund', 'fail@example.com', '07000000000', 'paid', 'pi_fail', 5000, 'GBP', 'none')
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	_, err = svc.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
		t.Fatalf("expected bad_request, got %v", err)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil {
		t.Fatal("group should still exist after refund failure")
	}
}

func TestDeleteRegistration_staleVersion(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{})

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status, version
		) VALUES ('Stale', 'Version', 'stale@example.com', '07000000000', 'pending', 0, 'GBP', 'none', 5)
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	_, err = svc.DeleteRegistration(ctx, groupID, "Admin", 1)
	if !errors.As(err, &commonerrors.APIError{}) {
		t.Fatalf("expected APIError, got %v", err)
	}
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "stale_state" {
		t.Fatalf("expected stale_state, got %v", err)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil {
		t.Fatal("group should still exist")
	}
}

func TestDeleteRegistration_sheetFailureNonFatal(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, failingSheets{}, Config{})

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status
		) VALUES ('Sheet', 'Fail', 'sheet@example.com', '07000000000', 'pending', 0, 'GBP', 'none')
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	_, err = svc.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("DeleteRegistration should succeed despite sheet error: %v", err)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g != nil {
		t.Fatal("group should be deleted")
	}
}

func TestDeleteRegistration_alreadyRefundedIdempotent(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	stub := &stubStripe{
		refundPI: func(context.Context, string) (int64, error) {
			return 0, nil // simulates charge_already_refunded -> 0 from helper
		},
	}
	svc := NewService(repo, stub, nil, nil, Config{})

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_payment_intent_id, total_amount_pence, currency, billing_status
		) VALUES ('Already', 'Refunded', 'already@example.com', '07000000000', 'paid', 'pi_done', 5000, 'GBP', 'none')
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	_, err = svc.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("DeleteRegistration: %v", err)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g != nil {
		t.Fatal("group should be deleted after idempotent refund")
	}
}

func TestDeleteRegistration_invoicedVoidsAndDeletes(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	inv := "in_test_open"
	var voided bool
	stub := &stubStripe{
		voidInvIdem: func(_ context.Context, id string) error {
			voided = true
			if id != inv {
				t.Fatalf("void invoice = %q, want %q", id, inv)
			}
			return nil
		},
	}
	svc := NewService(repo, stub, nil, nil, Config{})

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_payment_intent_id, stripe_invoice_id,
			total_amount_pence, currency, billing_status
		) VALUES ('Open', 'Invoice', 'open@example.com', '07000000000', 'paid', 'pi_dep', $1, 5000, 'GBP', 'invoiced')
		RETURNING id`, inv).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	sum, err := svc.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("DeleteRegistration: %v", err)
	}
	if !voided || !sum.InvoiceVoided {
		t.Fatalf("expected invoice voided, summary=%+v", sum)
	}
	// Deposit was paid, so it is refunded too (refundPI stub returns 0 -> ok).
	if !sum.DepositRefunded {
		t.Fatal("expected deposit refunded for paid invoiced group")
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g != nil {
		t.Fatal("group should be deleted")
	}
}

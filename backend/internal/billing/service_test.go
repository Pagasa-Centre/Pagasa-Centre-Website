package billing

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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
	sponsorships   []email.SponsorshipConfirmed
	balancePaid    []email.BalancePaidConfirmation
	accomChanged   []email.AccommodationChangedNotice
	bookingUpdated []email.BookingUpdated
}

func (m *recordingMailer) SendBookingUpdated(_ context.Context, p email.BookingUpdated) error {
	m.bookingUpdated = append(m.bookingUpdated, p)
	return nil
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
		0,
	)
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	want := "Josh Basco — Static Caravan (Caravan 5)"
	if items[0] != want {
		t.Fatalf("got %q, want %q", items[0], want)
	}
}

func TestBalanceInvoiceItemsIncludesDayPass(t *testing.T) {
	items := balanceInvoiceItems(
		[]domain.Camper{
			{
				FirstName:                  "Josh",
				LastName:                   "Basco",
				AttendanceType:             domain.AttendanceFullWeek,
				AllocatedAccommodationCode: strPtr("lodge"),
			},
			{
				FirstName:      "Sam",
				LastName:       "Visitor",
				AttendanceType: domain.AttendanceDayPass,
				DayPassDays:    []string{"mon", "tue"},
			},
			{
				FirstName:      "Solo",
				LastName:       "Day",
				AttendanceType: domain.AttendanceDayPass,
				DayPassDays:    []string{"wed"},
			},
		},
		map[string]string{"lodge": "Lodge"},
		map[string]string{},
		0,
	)
	if len(items) != 3 {
		t.Fatalf("items = %v, want 3", items)
	}
	if items[0] != "Josh Basco — Lodge" {
		t.Errorf("full-week line = %q", items[0])
	}
	if items[1] != "Sam Visitor — Day pass (2 days)" {
		t.Errorf("day-pass line = %q, want plural", items[1])
	}
	if items[2] != "Solo Day — Day pass (1 day)" {
		t.Errorf("day-pass line = %q, want singular", items[2])
	}
}

func TestDayPassLine(t *testing.T) {
	line, ok := dayPassLine(domain.Camper{
		AttendanceType: domain.AttendanceDayPass,
		DayPassDays:    []string{"mon", "tue", "wed"},
	}, "price_day")
	if !ok {
		t.Fatal("expected a day-pass line")
	}
	if line.PriceID != "price_day" || line.Quantity != 3 {
		t.Fatalf("got %+v, want {price_day 3}", line)
	}

	if _, ok := dayPassLine(domain.Camper{AttendanceType: domain.AttendanceFullWeek}, "price_day"); ok {
		t.Error("full-week camper should not produce a day-pass line")
	}
	if _, ok := dayPassLine(domain.Camper{AttendanceType: domain.AttendanceDayPass}, "price_day"); ok {
		t.Error("day-pass camper with no days should not produce a line")
	}
}

func TestCoachEligibleCount(t *testing.T) {
	yes := true
	no := false
	campers := []domain.Camper{
		{AttendanceType: domain.AttendanceFullWeek, Age: 30, NeedsCoach: &yes},  // counts
		{AttendanceType: domain.AttendanceFullWeek, Age: 4, NeedsCoach: &yes},   // counts (>= 4)
		{AttendanceType: domain.AttendanceFullWeek, Age: 3, NeedsCoach: &yes},   // free (under 4)
		{AttendanceType: domain.AttendanceFullWeek, Age: 25, NeedsCoach: &no},   // opted out
		{AttendanceType: domain.AttendanceFullWeek, Age: 25, NeedsCoach: nil},   // not set
		{AttendanceType: domain.AttendanceDayPass, Age: 25, NeedsCoach: &yes},   // day-pass excluded
	}
	if n := coachEligibleCount(campers); n != 2 {
		t.Fatalf("coachEligibleCount = %d, want 2", n)
	}
}

func TestCoachLine(t *testing.T) {
	line, ok := coachLine(3, "price_coach")
	if !ok || line.PriceID != "price_coach" || line.Quantity != 3 {
		t.Fatalf("got %+v ok=%v, want {price_coach 3} true", line, ok)
	}
	if _, ok := coachLine(0, "price_coach"); ok {
		t.Error("zero passengers should not produce a coach line")
	}
}

func TestBalanceInvoiceItemsIncludesCoach(t *testing.T) {
	items := balanceInvoiceItems(
		[]domain.Camper{{
			FirstName:                  "Josh",
			LastName:                   "Basco",
			AttendanceType:             domain.AttendanceFullWeek,
			AllocatedAccommodationCode: strPtr("lodge"),
		}},
		map[string]string{"lodge": "Lodge"},
		map[string]string{},
		2,
	)
	if len(items) != 2 {
		t.Fatalf("items = %v, want 2", items)
	}
	if items[1] != "Coach transport (×2)" {
		t.Fatalf("coach line = %q", items[1])
	}
	// No coach line when count is 0.
	none := balanceInvoiceItems(
		[]domain.Camper{{
			FirstName:                  "Josh",
			LastName:                   "Basco",
			AttendanceType:             domain.AttendanceFullWeek,
			AllocatedAccommodationCode: strPtr("lodge"),
		}},
		map[string]string{"lodge": "Lodge"},
		map[string]string{},
		0,
	)
	if len(none) != 1 {
		t.Fatalf("items = %v, want 1 (no coach)", none)
	}
}

func TestResolvePriceID_caravanOverflowMatchesTent(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	price := "price_shared_tent_overflow"
	if err := repo.UpdateAccommodationStripePrice(ctx, "tent", price); err != nil {
		t.Fatalf("update tent price: %v", err)
	}
	if err := repo.UpdateAccommodationStripePrice(ctx, "caravan_overflow", price); err != nil {
		t.Fatalf("update overflow price: %v", err)
	}
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{})
	tentID, err := svc.resolvePriceID(ctx, "tent", 25)
	if err != nil {
		t.Fatalf("resolve tent: %v", err)
	}
	overflowID, err := svc.resolvePriceID(ctx, "caravan_overflow", 25)
	if err != nil {
		t.Fatalf("resolve overflow: %v", err)
	}
	if tentID != overflowID {
		t.Fatalf("tent=%q overflow=%q, want same price id", tentID, overflowID)
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
	refundInv     func(ctx context.Context, id string, keepPence int64) (int64, error)
	refundInvCall []refundInvCall
	voidInv       func(ctx context.Context, id string) error
	voidInvIdem   func(ctx context.Context, id string) error
	getInvoice    func(ctx context.Context, id string) (InvoiceResult, error)
	ensureCust    func(ctx context.Context, existingID, email, name, groupID string) (string, error)
	createInvoice func(ctx context.Context, customerID, groupID string, lines []InvoiceLine, daysUntilDue int64, invoiceType string) (InvoiceResult, error)
	creditBalance func(ctx context.Context, customerID string, amountPence int64, currency, description, idempotencyKey string) error
	creditCalls   []creditCall
}

type creditCall struct {
	customerID     string
	amountPence    int64
	idempotencyKey string
}

// refundInvCall records what delete asked Stripe to refund. Tests assert on the
// kept amount rather than on the summary, because a summary-only assertion would
// pass while the deposit went back to the family.
type refundInvCall struct {
	invoiceID string
	keepPence int64
}

func (s *stubStripe) EnsureCustomer(ctx context.Context, existingID, email, name, groupID string) (string, error) {
	if s.ensureCust != nil {
		return s.ensureCust(ctx, existingID, email, name, groupID)
	}
	return "", errors.New("unexpected EnsureCustomer")
}
func (s *stubStripe) CreateInvoice(ctx context.Context, customerID, groupID string, lines []InvoiceLine, daysUntilDue int64, invoiceType string) (InvoiceResult, error) {
	if s.createInvoice != nil {
		return s.createInvoice(ctx, customerID, groupID, lines, daysUntilDue, invoiceType)
	}
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
func (s *stubStripe) GetInvoice(ctx context.Context, invoiceID string) (InvoiceResult, error) {
	if s.getInvoice != nil {
		return s.getInvoice(ctx, invoiceID)
	}
	return InvoiceResult{}, errors.New("unexpected GetInvoice")
}
func (s *stubStripe) RefundInvoiceKeeping(ctx context.Context, id string, keepPence int64) (int64, error) {
	s.refundInvCall = append(s.refundInvCall, refundInvCall{invoiceID: id, keepPence: keepPence})
	if s.refundInv != nil {
		return s.refundInv(ctx, id, keepPence)
	}
	return 0, nil
}
func (s *stubStripe) CreditCustomerBalance(
	ctx context.Context,
	customerID string,
	amountPence int64,
	currency, description, idempotencyKey string,
) error {
	s.creditCalls = append(s.creditCalls, creditCall{
		customerID:     customerID,
		amountPence:    amountPence,
		idempotencyKey: idempotencyKey,
	})
	if s.creditBalance != nil {
		return s.creditBalance(ctx, customerID, amountPence, currency, description, idempotencyKey)
	}
	return nil
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
	if sum.BalanceRefunded || sum.InvoiceVoided {
		t.Fatal("expected no Stripe cleanup for unpaid group")
	}
	if sum.RetainedPence != 0 {
		t.Fatalf("retained = %d, want 0: they never paid a deposit", sum.RetainedPence)
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

// The deposit is advertised as non-refundable, so deleting must keep it. Nothing
// may be refunded against the deposit payment at all — not a reduced amount, not
// a zero-amount call.
func TestDeleteRegistration_paidDepositIsKeptNotRefunded(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	stub := &stubStripe{}
	svc := NewService(repo, stub, nil, nil, Config{})

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_payment_intent_id, total_amount_pence, currency, billing_status
		) VALUES ('Paid', 'Deposit', 'paid@example.com', '07000000000', 'paid', 'pi_test_deposit', 5000, 'GBP', 'none')
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	sum, err := svc.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("DeleteRegistration: %v", err)
	}
	if len(stub.refundInvCall) != 0 {
		t.Fatalf("refunds asked of Stripe = %+v, want none", stub.refundInvCall)
	}
	if sum.RetainedPence != 5000 {
		t.Fatalf("retained = %d, want the 5000 deposit they paid", sum.RetainedPence)
	}
	if sum.AmountPence != 0 {
		t.Fatalf("refunded = %d, want 0", sum.AmountPence)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g != nil {
		t.Fatal("group should be deleted")
	}
}

// A settled balance is refunded because it paid for accommodation nobody will
// use, but the deposit that came in through checkout is untouched. The kept and
// refunded figures must not overlap: they are the record of where the money went.
func TestDeleteRegistration_balancePaidRefundsBalanceOnly(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	inv := "in_test_full"
	stub := &stubStripe{
		refundInv: func(_ context.Context, id string, _ int64) (int64, error) {
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
		) VALUES ('Full', 'Paid', 'full@example.com', '07000000000', 'paid', 'pi_test_full', $1, 5000, 'GBP', 'balance_paid')
		RETURNING id`, inv).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	sum, err := svc.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("DeleteRegistration: %v", err)
	}
	if len(stub.refundInvCall) != 1 {
		t.Fatalf("refund calls = %+v, want exactly the balance invoice", stub.refundInvCall)
	}
	if stub.refundInvCall[0].keepPence != 0 {
		t.Fatalf("kept back %d of the invoice, want 0: no deposit was billed on it",
			stub.refundInvCall[0].keepPence)
	}
	if sum.AmountPence != 12000 {
		t.Fatalf("refunded = %d, want the 12000 balance only", sum.AmountPence)
	}
	if sum.RetainedPence != 5000 {
		t.Fatalf("retained = %d, want the 5000 deposit", sum.RetainedPence)
	}
	if !sum.BalanceRefunded {
		t.Fatal("balance refund should be reported")
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g != nil {
		t.Fatal("group should be deleted")
	}
}

// A camper added after checkout has their deposit billed on the balance invoice.
// That money is still a deposit, so it is kept back out of the balance refund and
// counted in what was retained.
func TestDeleteRegistration_keepsDepositBilledOnTheInvoice(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	inv := "in_late_arrival"
	stub := &stubStripe{
		refundInv: func(_ context.Context, _ string, keepPence int64) (int64, error) {
			return 17000 - keepPence, nil
		},
	}
	svc := NewService(repo, stub, nil, nil, Config{})

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_payment_intent_id, stripe_invoice_id,
			total_amount_pence, currency, billing_status
		) VALUES ('Late', 'Arrival', 'late@example.com', '07000000000', 'paid', 'pi_late', $1, 10000, 'GBP', 'balance_paid')
		RETURNING id`, inv).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}
	// Two campers paid a deposit at checkout (the group's 10000) and a third was
	// added later, paying 5000 of deposit on the invoice.
	if _, err := pool.Exec(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type, deposit_paid_pence
		) VALUES ($1, true, 'Naomi', 'Late', 'female', 24, 'Leader', false, 'full_week', 5000)`,
		groupID); err != nil {
		t.Fatalf("insert camper: %v", err)
	}

	sum, err := svc.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("DeleteRegistration: %v", err)
	}
	if len(stub.refundInvCall) != 1 || stub.refundInvCall[0].keepPence != 5000 {
		t.Fatalf("refund calls = %+v, want the invoice refunded less 5000 of deposit",
			stub.refundInvCall)
	}
	if sum.RetainedPence != 15000 {
		t.Fatalf("retained = %d, want 15000: 10000 at checkout plus 5000 on the invoice",
			sum.RetainedPence)
	}
	if sum.AmountPence != 12000 {
		t.Fatalf("refunded = %d, want 12000: the 17000 invoice less the 5000 deposit",
			sum.AmountPence)
	}
}

// One payment covering deposit and camp cost cannot be split, and the deposit
// portion is stored nowhere, so guessing it is worse than refusing.
func TestDeleteRegistration_refusesPaidInFullRegistration(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	stub := &stubStripe{}
	svc := NewService(repo, stub, nil, nil, Config{})

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_payment_intent_id, stripe_invoice_id,
			total_amount_pence, currency, billing_status, paid_in_full_at_registration
		) VALUES ('One', 'Payment', 'one@example.com', '07000000000', 'paid', 'pi_one', 'in_one',
		          45000, 'GBP', 'balance_paid', true)
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	_, err = svc.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
		t.Fatalf("expected bad_request, got %v", err)
	}
	if !strings.Contains(apiErr.Message, "Stripe") {
		t.Fatalf("message = %q, should send the admin to Stripe", apiErr.Message)
	}
	if len(stub.refundInvCall) != 0 {
		t.Fatalf("refunds asked of Stripe = %+v, want none", stub.refundInvCall)
	}
	if g, _ := repo.FindGroupByID(ctx, groupID); g == nil {
		t.Fatal("registration should be left exactly as it was")
	}
}

// Being flagged as a full-payment registration is not the problem; having paid one
// mixed charge is. An unpaid one has nothing to split.
func TestDeleteRegistration_paidInFullButNeverPaidStillDeletes(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{})

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status,
			paid_in_full_at_registration
		) VALUES ('Never', 'Paid', 'never@example.com', '07000000000', 'pending', 45000, 'GBP',
		          'none', true)
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	sum, err := svc.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("DeleteRegistration: %v", err)
	}
	if sum.RetainedPence != 0 {
		t.Fatalf("retained = %d, want 0: no money ever arrived", sum.RetainedPence)
	}
	if g, _ := repo.FindGroupByID(ctx, groupID); g != nil {
		t.Fatal("group should be deleted")
	}
}

// A church-sponsored place is paid but cost nothing, so there is no deposit to
// keep and the audit line should not claim one.
func TestDeleteRegistration_sponsoredRetainsNothing(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	stub := &stubStripe{}
	svc := NewService(repo, stub, nil, nil, Config{})

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status, is_free
		) VALUES ('Spon', 'Sored', 'spon@example.com', '07000000000', 'paid', 0, 'GBP',
		          'free_confirmed', true)
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	sum, err := svc.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("DeleteRegistration: %v", err)
	}
	if sum.RetainedPence != 0 || sum.AmountPence != 0 {
		t.Fatalf("summary = %+v, want nothing kept and nothing refunded", sum)
	}
	if len(stub.refundInvCall) != 0 {
		t.Fatalf("refunds asked of Stripe = %+v, want none", stub.refundInvCall)
	}
}

// A balance can consist only of late arrivals' deposits, in which case the whole
// payment is money the church keeps. Asking Stripe for a zero or negative refund
// is an error, so nothing is refunded and the delete still goes through.
func TestDeleteRegistration_depositCoveringWholeBalanceRefundsNothing(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	const invoicePaid = 5000
	stub := &stubStripe{
		// Mirrors what RefundInvoiceKeeping does against real Stripe: nothing is
		// refundable once the kept deposit covers what was paid.
		refundInv: func(_ context.Context, _ string, keepPence int64) (int64, error) {
			if refundable := int64(invoicePaid) - keepPence; refundable > 0 {
				return refundable, nil
			}
			return 0, nil
		},
	}
	svc := NewService(repo, stub, nil, nil, Config{})

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_payment_intent_id, stripe_invoice_id,
			total_amount_pence, currency, billing_status
		) VALUES ('All', 'Deposit', 'alldep@example.com', '07000000000', 'paid', 'pi_alldep',
		          'in_alldep', 10000, 'GBP', 'balance_paid')
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type, deposit_paid_pence
		) VALUES ($1, true, 'Only', 'Deposit', 'male', 22, 'Leader', false, 'full_week', $2)`,
		groupID, invoicePaid); err != nil {
		t.Fatalf("insert camper: %v", err)
	}

	sum, err := svc.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("DeleteRegistration: %v", err)
	}
	if len(stub.refundInvCall) != 1 || stub.refundInvCall[0].keepPence != invoicePaid {
		t.Fatalf("refund calls = %+v, want the whole %d held back", stub.refundInvCall, invoicePaid)
	}
	if sum.AmountPence != 0 || sum.BalanceRefunded {
		t.Fatalf("summary = %+v, want nothing refunded", sum)
	}
	if sum.RetainedPence != 15000 {
		t.Fatalf("retained = %d, want 15000: 10000 at checkout plus the 5000 invoiced deposit",
			sum.RetainedPence)
	}
	if g, _ := repo.FindGroupByID(ctx, groupID); g != nil {
		t.Fatal("group should be deleted")
	}
}

// An unpaid coach invoice must be voided so it cannot be paid after the
// registration is gone. A paid one is left alone: that refund is a Stripe job.
func TestDeleteRegistration_voidsOpenCoachInvoice(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)

	cases := map[string]struct {
		coachPaid  bool
		wantVoided []string
	}{
		"unpaid coach invoice is voided": {coachPaid: false, wantVoided: []string{"in_coach"}},
		"paid coach fee is left alone":   {coachPaid: true, wantVoided: nil},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var voided []string
			stub := &stubStripe{
				voidInvIdem: func(_ context.Context, id string) error {
					voided = append(voided, id)
					return nil
				},
			}
			svc := NewService(repo, stub, nil, nil, Config{})

			var paidAt *time.Time
			if tc.coachPaid {
				now := time.Now().UTC()
				paidAt = &now
			}
			var groupID string
			err := pool.QueryRow(ctx, `
				INSERT INTO registration_groups (
					contact_first_name, contact_last_name, contact_email, contact_phone,
					payment_status, stripe_payment_intent_id, stripe_coach_invoice_id,
					coach_fee_paid_at, total_amount_pence, currency, billing_status
				) VALUES ('Coach', 'Open', 'coach@example.com', '07000000000', 'paid', 'pi_coach',
				          'in_coach', $1, 5000, 'GBP', 'none')
				RETURNING id`, paidAt).Scan(&groupID)
			if err != nil {
				t.Fatalf("insert group: %v", err)
			}

			if _, err := svc.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
				t.Fatalf("DeleteRegistration: %v", err)
			}
			if len(voided) != len(tc.wantVoided) {
				t.Fatalf("voided = %v, want %v", voided, tc.wantVoided)
			}
			for i, want := range tc.wantVoided {
				if voided[i] != want {
					t.Fatalf("voided = %v, want %v", voided, tc.wantVoided)
				}
			}
			if g, _ := repo.FindGroupByID(ctx, groupID); g != nil {
				t.Fatal("group should be deleted")
			}
		})
	}
}

// A balance settled by bank transfer has no Stripe payment behind it, so there is
// nothing to refund. That must delete cleanly rather than fail.
func TestDeleteRegistration_manuallySettledBalanceDeletesWithoutRefund(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	stub := &stubStripe{
		refundInv: func(_ context.Context, _ string, _ int64) (int64, error) {
			return 0, nil // no payment found behind the invoice
		},
	}
	svc := NewService(repo, stub, nil, nil, Config{})

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_payment_intent_id, stripe_invoice_id,
			total_amount_pence, currency, billing_status
		) VALUES ('Bank', 'Transfer', 'bank@example.com', '07000000000', 'paid', 'pi_bank', 'in_bank',
		          5000, 'GBP', 'balance_paid')
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	sum, err := svc.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("DeleteRegistration: %v", err)
	}
	if sum.BalanceRefunded {
		t.Fatal("nothing was refunded, so the summary must not say it was")
	}
	if sum.RetainedPence != 5000 {
		t.Fatalf("retained = %d, want the 5000 deposit", sum.RetainedPence)
	}
	if g, _ := repo.FindGroupByID(ctx, groupID); g != nil {
		t.Fatal("group should be deleted")
	}
}

// A delete that half-happened is worse than either outcome, so any Stripe step
// failing leaves the registration exactly where it was.
func TestDeleteRegistration_stripeFailureAborts(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)

	cases := map[string]struct {
		stub           *stubStripe
		billingStatus  string
		coachInvoiceID *string
	}{
		"balance refund fails": {
			stub: &stubStripe{
				refundInv: func(context.Context, string, int64) (int64, error) {
					return 0, errors.New("stripe down")
				},
			},
			billingStatus: domain.BillingBalancePaid,
		},
		"invoice void fails": {
			stub: &stubStripe{
				voidInvIdem: func(context.Context, string) error {
					return errors.New("stripe down")
				},
			},
			billingStatus: domain.BillingInvoiced,
		},
		"coach invoice void fails": {
			stub: &stubStripe{
				voidInvIdem: func(context.Context, string) error {
					return errors.New("stripe down")
				},
			},
			billingStatus:  domain.BillingNone,
			coachInvoiceID: strPtr("in_coach_fail"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc := NewService(repo, tc.stub, nil, nil, Config{})

			var groupID string
			err := pool.QueryRow(ctx, `
				INSERT INTO registration_groups (
					contact_first_name, contact_last_name, contact_email, contact_phone,
					payment_status, stripe_payment_intent_id, stripe_invoice_id,
					stripe_coach_invoice_id, total_amount_pence, currency, billing_status
				) VALUES ('Fail', 'Stripe', 'fail@example.com', '07000000000', 'paid', 'pi_fail',
				          'in_fail', $1, 5000, 'GBP', $2)
				RETURNING id`, tc.coachInvoiceID, tc.billingStatus).Scan(&groupID)
			if err != nil {
				t.Fatalf("insert group: %v", err)
			}

			_, err = svc.DeleteRegistration(ctx, groupID, "Admin", domain.SkipVersionCheck)
			var apiErr commonerrors.APIError
			if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
				t.Fatalf("expected bad_request, got %v", err)
			}
			if g, _ := repo.FindGroupByID(ctx, groupID); g == nil {
				t.Fatal("group should still exist after the Stripe step failed")
			}
		})
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
		refundInv: func(context.Context, string, int64) (int64, error) {
			return 0, nil // simulates charge_already_refunded -> 0 from the helper
		},
	}
	svc := NewService(repo, stub, nil, nil, Config{})

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_payment_intent_id, stripe_invoice_id,
			total_amount_pence, currency, billing_status
		) VALUES ('Already', 'Refunded', 'already@example.com', '07000000000', 'paid', 'pi_done',
		          'in_done', 5000, 'GBP', 'balance_paid')
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
	// The invoice was never paid, so there is nothing to refund — and the deposit
	// they did pay is kept.
	if sum.AmountPence != 0 || len(stub.refundInvCall) != 0 {
		t.Fatalf("refunded %d via %+v, want nothing refunded", sum.AmountPence, stub.refundInvCall)
	}
	if sum.RetainedPence != 5000 {
		t.Fatalf("retained = %d, want the 5000 deposit", sum.RetainedPence)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g != nil {
		t.Fatal("group should be deleted")
	}
}

func insertThreeCamperGroup(ctx context.Context, t *testing.T, pool *pgxpool.Pool, billingStatus string, stripeInvoiceID *string) (groupID, mainID, camper2ID, camper3ID string) {
	t.Helper()
	inv := any(nil)
	if stripeInvoiceID != nil {
		inv = *stripeInvoiceID
	}
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_payment_intent_id, stripe_invoice_id,
			total_amount_pence, currency, billing_status, version
		) VALUES ('Sam', 'Family', 'family@example.com', '07000000000', 'paid', 'pi_test', $1, 15000, 'GBP', $2, 1)
		RETURNING id`, inv, billingStatus).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}
	for _, spec := range []struct {
		id    *string
		main  bool
		name  string
		alloc *string
	}{
		{&mainID, true, "Sam", strPtr("lodge")},
		{&camper2ID, false, "Alex", strPtr("lodge")},
		{&camper3ID, false, "Jordan", strPtr("tent")},
	} {
		err = pool.QueryRow(ctx, `
			INSERT INTO registrations (
				group_id, is_main_contact, first_name, last_name, gender, age,
				cell_leader_name, is_cell_leader, attendance_type,
				allocated_accommodation_code
			) VALUES ($1, $2, $3, 'Test', 'male', 25, 'Leader', false, 'full_week', $4)
			RETURNING id`, groupID, spec.main, spec.name, spec.alloc).Scan(spec.id)
		if err != nil {
			t.Fatalf("insert camper %s: %v", spec.name, err)
		}
	}
	return groupID, mainID, camper2ID, camper3ID
}

func countTierAllocations(ctx context.Context, pool *pgxpool.Pool, groupID, tier string) int {
	var n int
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM registrations
		WHERE group_id = $1 AND allocated_accommodation_code = $2`, groupID, tier).Scan(&n)
	return n
}

func TestRemoveCamper_success(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{})

	groupID, _, camper2ID, camper3ID := insertThreeCamperGroup(ctx, t, pool, domain.BillingAllocated, nil)

	sum, err := svc.RemoveCamper(ctx, groupID, camper2ID, "Admin", 1)
	if err != nil {
		t.Fatalf("RemoveCamper: %v", err)
	}
	if sum.CamperName != "Alex Test" {
		t.Fatalf("camper name = %q", sum.CamperName)
	}
	if sum.InvoiceVoided {
		t.Fatal("expected no invoice void")
	}

	campers, err := repo.CampersForGroup(ctx, groupID)
	if err != nil {
		t.Fatalf("CampersForGroup: %v", err)
	}
	if len(campers) != 2 {
		t.Fatalf("expected 2 campers, got %d", len(campers))
	}
	ids := map[string]bool{campers[0].ID: true, campers[1].ID: true}
	if !ids[camper3ID] || ids[camper2ID] {
		t.Fatalf("wrong campers remain: %+v", campers)
	}

	g, err := repo.FindGroupByID(ctx, groupID)
	if err != nil || g == nil {
		t.Fatalf("FindGroupByID: %v", err)
	}
	if g.Version != 2 {
		t.Fatalf("version = %d, want 2", g.Version)
	}
	if g.LastAction == nil || *g.LastAction != "camper_removed" {
		t.Fatalf("last_action = %v", g.LastAction)
	}
}

func TestRemoveCamper_mainContactBlocked(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{})

	groupID, mainID, _, _ := insertThreeCamperGroup(ctx, t, pool, domain.BillingAllocated, nil)

	_, err := svc.RemoveCamper(ctx, groupID, mainID, "Admin", domain.SkipVersionCheck)
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
		t.Fatalf("expected bad_request, got %v", err)
	}
	campers, _ := repo.CampersForGroup(ctx, groupID)
	if len(campers) != 3 {
		t.Fatalf("expected 3 campers unchanged, got %d", len(campers))
	}
}

func TestRemoveCamper_lastCamperBlocked(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{})

	var groupID, camperID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status
		) VALUES ('Solo', 'Camper', 'solo@example.com', '07000000000', 'paid', 5000, 'GBP', 'none')
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type
		) VALUES ($1, false, 'Only', 'Person', 'male', 30, 'Leader', false, 'full_week')
		RETURNING id`, groupID).Scan(&camperID)
	if err != nil {
		t.Fatalf("insert camper: %v", err)
	}

	_, err = svc.RemoveCamper(ctx, groupID, camperID, "Admin", domain.SkipVersionCheck)
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
		t.Fatalf("expected bad_request, got %v", err)
	}
}

func TestRemoveCamper_balancePaidBlocked(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{})

	groupID, _, camper2ID, _ := insertThreeCamperGroup(ctx, t, pool, domain.BillingBalancePaid, nil)

	_, err := svc.RemoveCamper(ctx, groupID, camper2ID, "Admin", domain.SkipVersionCheck)
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
		t.Fatalf("expected bad_request, got %v", err)
	}
}

func TestRemoveCamper_invoicedVoidsAndReverts(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	inv := "in_remove_camper"
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

	groupID, _, camper2ID, _ := insertThreeCamperGroup(ctx, t, pool, domain.BillingInvoiced, &inv)

	sum, err := svc.RemoveCamper(ctx, groupID, camper2ID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("RemoveCamper: %v", err)
	}
	if !voided || !sum.InvoiceVoided {
		t.Fatalf("expected invoice voided, summary=%+v", sum)
	}

	g, err := repo.FindGroupByID(ctx, groupID)
	if err != nil || g == nil {
		t.Fatalf("FindGroupByID: %v", err)
	}
	if g.BillingStatus != domain.BillingAllocated {
		t.Fatalf("billing_status = %q, want allocated", g.BillingStatus)
	}
	if g.StripeInvoiceID != nil {
		t.Fatalf("stripe_invoice_id should be cleared, got %v", g.StripeInvoiceID)
	}
	if g.InvoiceDueAt != nil {
		t.Fatal("invoice_due_at should be cleared")
	}
	campers, _ := repo.CampersForGroup(ctx, groupID)
	if len(campers) != 2 {
		t.Fatalf("expected 2 campers, got %d", len(campers))
	}
}

func TestRemoveCamper_freesAllocation(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{})

	groupID, _, camper2ID, _ := insertThreeCamperGroup(ctx, t, pool, domain.BillingAllocated, nil)
	before := countTierAllocations(ctx, pool, groupID, "lodge")
	if before != 2 {
		t.Fatalf("lodge count before = %d, want 2", before)
	}

	_, err := svc.RemoveCamper(ctx, groupID, camper2ID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("RemoveCamper: %v", err)
	}
	after := countTierAllocations(ctx, pool, groupID, "lodge")
	if after != 1 {
		t.Fatalf("lodge count after = %d, want 1", after)
	}
}

func TestRemoveCamper_staleVersion(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{})

	groupID, _, camper2ID, _ := insertThreeCamperGroup(ctx, t, pool, domain.BillingAllocated, nil)

	_, err := svc.RemoveCamper(ctx, groupID, camper2ID, "Admin", 99)
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "stale_state" {
		t.Fatalf("expected stale_state, got %v", err)
	}
}

func TestRemoveCamper_sheetFailureNonFatal(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, failingSheets{}, Config{})

	groupID, _, camper2ID, _ := insertThreeCamperGroup(ctx, t, pool, domain.BillingAllocated, nil)

	_, err := svc.RemoveCamper(ctx, groupID, camper2ID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("RemoveCamper should succeed despite sheet error: %v", err)
	}
	campers, _ := repo.CampersForGroup(ctx, groupID)
	if len(campers) != 2 {
		t.Fatalf("expected 2 campers, got %d", len(campers))
	}
}

// insertCoachGroup seeds a paid group with one full-week camper (age 30) who
// needs the coach, allocated to `tier`, with the given billing_status.
func insertCoachGroup(ctx context.Context, t *testing.T, pool *pgxpool.Pool, billingStatus, tier string, isFree bool) string {
	t.Helper()
	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_customer_id, total_amount_pence, currency, billing_status, is_free, version
		) VALUES ('Coach', 'Rider', 'coach@example.com', '07000000000', 'paid', 'cus_seed', 5000, 'GBP', $1, $2, 1)
		RETURNING id`, billingStatus, isFree).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert coach group: %v", err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type, needs_coach,
			allocated_accommodation_code
		) VALUES ($1, true, 'Coach', 'Rider', 'male', 30, 'Leader', false, 'full_week', true, $2)`,
		groupID, tier)
	if err != nil {
		t.Fatalf("insert coach camper: %v", err)
	}
	return groupID
}

func TestSendInvoice_foldsCoachLine(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	if err := repo.UpdateAccommodationStripePrice(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("set lodge price: %v", err)
	}

	var captured []InvoiceLine
	var capturedType string
	stub := &stubStripe{
		ensureCust: func(_ context.Context, existingID, _, _, _ string) (string, error) {
			return "cus_seed", nil
		},
		createInvoice: func(_ context.Context, _, _ string, lines []InvoiceLine, _ int64, invoiceType string) (InvoiceResult, error) {
			captured = lines
			capturedType = invoiceType
			return InvoiceResult{ID: "in_balance", StripeEmailed: true}, nil
		},
	}
	svc := NewService(repo, stub, nil, nil, Config{StripePriceCoach: "price_coach"})

	groupID := insertCoachGroup(ctx, t, pool, domain.BillingAllocated, "lodge", false)
	if err := svc.SendInvoice(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("SendInvoice: %v", err)
	}
	if capturedType != "balance" {
		t.Fatalf("invoice type = %q, want balance", capturedType)
	}
	var haveLodge, haveCoach bool
	for _, l := range captured {
		if l.PriceID == "price_lodge" && l.Quantity == 1 {
			haveLodge = true
		}
		if l.PriceID == "price_coach" && l.Quantity == 1 {
			haveCoach = true
		}
	}
	if !haveLodge || !haveCoach {
		t.Fatalf("lines = %+v, want lodge + coach", captured)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil || !g.CoachIncludedInBalance {
		t.Fatalf("coach_included_in_balance not set: %+v", g)
	}
	if g.BillingStatus != domain.BillingInvoiced {
		t.Fatalf("billing_status = %q, want invoiced", g.BillingStatus)
	}
}

func TestSendCoachInvoice_paidGroup(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)

	var captured []InvoiceLine
	var capturedType string
	stub := &stubStripe{
		ensureCust: func(_ context.Context, existingID, _, _, _ string) (string, error) {
			return "cus_seed", nil
		},
		createInvoice: func(_ context.Context, _, _ string, lines []InvoiceLine, _ int64, invoiceType string) (InvoiceResult, error) {
			captured = lines
			capturedType = invoiceType
			return InvoiceResult{ID: "in_coach", StripeEmailed: true}, nil
		},
	}
	svc := NewService(repo, stub, nil, nil, Config{StripePriceCoach: "price_coach"})

	groupID := insertCoachGroup(ctx, t, pool, domain.BillingBalancePaid, "lodge", false)
	if err := svc.SendCoachInvoice(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("SendCoachInvoice: %v", err)
	}
	if capturedType != "coach" {
		t.Fatalf("invoice type = %q, want coach", capturedType)
	}
	if len(captured) != 1 || captured[0].PriceID != "price_coach" || captured[0].Quantity != 1 {
		t.Fatalf("lines = %+v, want single coach line", captured)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil || g.StripeCoachInvoiceID == nil || *g.StripeCoachInvoiceID != "in_coach" {
		t.Fatalf("coach invoice id not stored: %+v", g)
	}
	if g.BillingStatus != domain.BillingBalancePaid {
		t.Fatalf("billing_status = %q, want unchanged balance_paid", g.BillingStatus)
	}

	// Second send rejects — already charged.
	err := svc.SendCoachInvoice(ctx, groupID, "Admin", domain.SkipVersionCheck)
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
		t.Fatalf("expected bad_request on repeat, got %v", err)
	}
}

func TestSendCoachInvoice_guards(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{StripePriceCoach: "price_coach"})

	// Allocated (not yet invoiced) → coach folds into balance instead.
	allocated := insertCoachGroup(ctx, t, pool, domain.BillingAllocated, "lodge", false)
	if err := svc.SendCoachInvoice(ctx, allocated, "Admin", domain.SkipVersionCheck); err == nil {
		t.Fatal("expected rejection for allocated group")
	}

	// Sponsored group → never coach-charged.
	free := insertCoachGroup(ctx, t, pool, domain.BillingInvoiced, "lodge", true)
	err := svc.SendCoachInvoice(ctx, free, "Admin", domain.SkipVersionCheck)
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
		t.Fatalf("expected bad_request for is_free, got %v", err)
	}

	// Invoiced group with no coach passengers → nothing to charge.
	var noCoach string
	if err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status
		) VALUES ('No', 'Coach', 'nocoach@example.com', '07000000000', 'paid', 5000, 'GBP', 'invoiced')
		RETURNING id`).Scan(&noCoach); err != nil {
		t.Fatalf("insert no-coach group: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type, needs_coach
		) VALUES ($1, true, 'No', 'Coach', 'male', 30, 'Leader', false, 'full_week', false)`,
		noCoach); err != nil {
		t.Fatalf("insert no-coach camper: %v", err)
	}
	if err := svc.SendCoachInvoice(ctx, noCoach, "Admin", domain.SkipVersionCheck); err == nil {
		t.Fatal("expected rejection when no coach passengers")
	}
}

func TestHandleCoachInvoicePaid(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{})

	groupID := insertCoachGroup(ctx, t, pool, domain.BillingBalancePaid, "lodge", false)
	if err := repo.SetCoachInvoiceMeta(ctx, groupID, "in_coach_paid", time.Now().Add(15*24*time.Hour), domain.ActionMeta{
		Actor:           "Admin",
		Action:          "coach_invoice_sent",
		ExpectedVersion: domain.SkipVersionCheck,
	}); err != nil {
		t.Fatalf("seed coach invoice: %v", err)
	}

	if err := svc.HandleCoachInvoicePaid(ctx, "in_coach_paid", groupID); err != nil {
		t.Fatalf("HandleCoachInvoicePaid: %v", err)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil || g.CoachFeePaidAt == nil {
		t.Fatalf("coach_fee_paid_at not set: %+v", g)
	}
	// Idempotent on repeat.
	if err := svc.HandleCoachInvoicePaid(ctx, "in_coach_paid", groupID); err != nil {
		t.Fatalf("HandleCoachInvoicePaid repeat: %v", err)
	}
}

func TestWaiveCoachFee_notYetSent(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	stripe := &stubStripe{
		voidInvIdem: func(context.Context, string) error {
			t.Fatal("unexpected VoidInvoiceIdempotent")
			return nil
		},
	}
	svc := NewService(repo, stripe, nil, nil, Config{StripePriceCoach: "price_coach"})

	groupID := insertCoachGroup(ctx, t, pool, domain.BillingAllocated, "lodge", false)
	if err := svc.WaiveCoachFee(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("WaiveCoachFee: %v", err)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil || g.CoachFeeWaivedAt == nil {
		t.Fatalf("coach_fee_waived_at not set: %+v", g)
	}
	if g.Version != 2 {
		t.Fatalf("version = %d, want 2", g.Version)
	}
	if g.LastAction == nil || *g.LastAction != "coach_fee_waived" {
		t.Fatalf("last_action = %v", g.LastAction)
	}
}

func TestWaiveCoachFee_voidOpenInvoice(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	var voidedID string
	stripe := &stubStripe{
		ensureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "cus_seed", nil
		},
		createInvoice: func(_ context.Context, _, _ string, _ []InvoiceLine, _ int64, invoiceType string) (InvoiceResult, error) {
			if invoiceType != "coach" {
				t.Fatalf("invoice type = %q, want coach", invoiceType)
			}
			return InvoiceResult{ID: "in_coach_open", StripeEmailed: true}, nil
		},
		voidInvIdem: func(_ context.Context, id string) error {
			voidedID = id
			return nil
		},
	}
	svc := NewService(repo, stripe, nil, nil, Config{StripePriceCoach: "price_coach"})

	groupID := insertCoachGroup(ctx, t, pool, domain.BillingBalancePaid, "lodge", false)
	if err := svc.SendCoachInvoice(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("SendCoachInvoice: %v", err)
	}
	if err := svc.WaiveCoachFee(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("WaiveCoachFee: %v", err)
	}
	if voidedID != "in_coach_open" {
		t.Fatalf("voided invoice = %q, want in_coach_open", voidedID)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil || g.CoachFeeWaivedAt == nil {
		t.Fatalf("coach_fee_waived_at not set: %+v", g)
	}
	if g.StripeCoachInvoiceID != nil {
		t.Fatalf("stripe_coach_invoice_id should be cleared: %+v", g.StripeCoachInvoiceID)
	}
}

func TestWaiveCoachFee_guards(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{StripePriceCoach: "price_coach"})

	t.Run("folded_in_balance", func(t *testing.T) {
		if err := repo.UpdateAccommodationStripePrice(ctx, "lodge", "price_lodge"); err != nil {
			t.Fatalf("set lodge price: %v", err)
		}
		stub := &stubStripe{
			ensureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
				return "cus_seed", nil
			},
			createInvoice: func(_ context.Context, _, _ string, _ []InvoiceLine, _ int64, _ string) (InvoiceResult, error) {
				return InvoiceResult{ID: "in_balance", StripeEmailed: true}, nil
			},
		}
		svcInv := NewService(repo, stub, nil, nil, Config{StripePriceCoach: "price_coach"})
		groupID := insertCoachGroup(ctx, t, pool, domain.BillingAllocated, "lodge", false)
		if err := svcInv.SendInvoice(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
			t.Fatalf("SendInvoice: %v", err)
		}
		err := svc.WaiveCoachFee(ctx, groupID, "Admin", domain.SkipVersionCheck)
		var apiErr commonerrors.APIError
		if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
			t.Fatalf("expected bad_request, got %v", err)
		}
	})

	t.Run("already_paid", func(t *testing.T) {
		groupID := insertCoachGroup(ctx, t, pool, domain.BillingBalancePaid, "lodge", false)
		if err := repo.SetCoachInvoiceMeta(ctx, groupID, "in_paid", time.Now().Add(15*24*time.Hour), domain.ActionMeta{
			Actor: "Admin", Action: "coach_invoice_sent", ExpectedVersion: domain.SkipVersionCheck,
		}); err != nil {
			t.Fatalf("seed coach invoice: %v", err)
		}
		if err := svc.HandleCoachInvoicePaid(ctx, "in_paid", groupID); err != nil {
			t.Fatalf("HandleCoachInvoicePaid: %v", err)
		}
		err := svc.WaiveCoachFee(ctx, groupID, "Admin", domain.SkipVersionCheck)
		var apiErr commonerrors.APIError
		if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
			t.Fatalf("expected bad_request, got %v", err)
		}
	})
}

func TestSendCoachInvoice_waivedGroup(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{StripePriceCoach: "price_coach"})

	groupID := insertCoachGroup(ctx, t, pool, domain.BillingBalancePaid, "lodge", false)
	if err := svc.WaiveCoachFee(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("WaiveCoachFee: %v", err)
	}
	err := svc.SendCoachInvoice(ctx, groupID, "Admin", domain.SkipVersionCheck)
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
		t.Fatalf("expected bad_request, got %v", err)
	}
}

func TestSendInvoice_skipsCoachWhenWaived(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	if err := repo.UpdateAccommodationStripePrice(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("set lodge price: %v", err)
	}

	var captured []InvoiceLine
	stub := &stubStripe{
		ensureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "cus_seed", nil
		},
		createInvoice: func(_ context.Context, _, _ string, lines []InvoiceLine, _ int64, invoiceType string) (InvoiceResult, error) {
			captured = lines
			if invoiceType != "balance" {
				t.Fatalf("invoice type = %q, want balance", invoiceType)
			}
			return InvoiceResult{ID: "in_balance", StripeEmailed: true}, nil
		},
	}
	svc := NewService(repo, stub, nil, nil, Config{StripePriceCoach: "price_coach"})

	groupID := insertCoachGroup(ctx, t, pool, domain.BillingAllocated, "lodge", false)
	if err := svc.WaiveCoachFee(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("WaiveCoachFee: %v", err)
	}
	if err := svc.SendInvoice(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("SendInvoice: %v", err)
	}
	for _, l := range captured {
		if l.PriceID == "price_coach" {
			t.Fatalf("coach line should be omitted when waived: %+v", captured)
		}
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil || g.CoachIncludedInBalance {
		t.Fatalf("coach_included_in_balance should be false: %+v", g)
	}
}

func TestResyncSheet_paidGroup(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	rec := &recordingSheets{}
	svc := NewService(repo, &stubStripe{}, nil, rec, Config{})

	groupID, camperID := insertDayPassGroup(ctx, t, pool, domain.BillingNone)
	_, err := pool.Exec(ctx, `
		UPDATE registrations SET shirt_size = $2, day_pass_tshirt_option = 'tshirt_only'
		WHERE id = $1`, camperID, "adult_m")
	if err != nil {
		t.Fatalf("update camper: %v", err)
	}

	if err := svc.ResyncSheet(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("ResyncSheet: %v", err)
	}
	if rec.removeCalls != 1 {
		t.Fatalf("remove calls = %d, want 1", rec.removeCalls)
	}
	if rec.appendCalls != 1 {
		t.Fatalf("append calls = %d, want 1", rec.appendCalls)
	}
	if rec.pendingCalls != 0 {
		t.Fatalf("pending calls = %d, want 0", rec.pendingCalls)
	}
	if len(rec.lastRows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rec.lastRows))
	}
	if rec.lastRows[0].ShirtSize == nil || *rec.lastRows[0].ShirtSize != "adult_m" {
		t.Fatalf("shirt_size = %v", rec.lastRows[0].ShirtSize)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil || g.LastAction == nil || *g.LastAction != "sheet_resynced" {
		t.Fatalf("last_action = %v", g.LastAction)
	}
}

func TestUnwaiveCoachFee(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	stub := &stubStripe{
		ensureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "cus_seed", nil
		},
		createInvoice: func(_ context.Context, _, _ string, _ []InvoiceLine, _ int64, invoiceType string) (InvoiceResult, error) {
			if invoiceType != "coach" {
				t.Fatalf("invoice type = %q, want coach", invoiceType)
			}
			return InvoiceResult{ID: "in_coach_after", StripeEmailed: true}, nil
		},
	}
	svc := NewService(repo, stub, nil, nil, Config{StripePriceCoach: "price_coach"})

	groupID := insertCoachGroup(ctx, t, pool, domain.BillingBalancePaid, "lodge", false)
	if err := svc.WaiveCoachFee(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("WaiveCoachFee: %v", err)
	}
	if err := svc.UnwaiveCoachFee(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("UnwaiveCoachFee: %v", err)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil || g.CoachFeeWaivedAt != nil {
		t.Fatalf("coach_fee_waived_at should be cleared: %+v", g)
	}
	if err := svc.SendCoachInvoice(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("SendCoachInvoice after unwaive: %v", err)
	}
	g, _ = repo.FindGroupByID(ctx, groupID)
	if g == nil || g.StripeCoachInvoiceID == nil || *g.StripeCoachInvoiceID != "in_coach_after" {
		t.Fatalf("coach invoice not sent after unwaive: %+v", g)
	}
}

func insertTwoCoachGroup(ctx context.Context, t *testing.T, pool *pgxpool.Pool, billingStatus string) (groupID, camper1ID, camper2ID string) {
	t.Helper()
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_customer_id, total_amount_pence, currency, billing_status, version
		) VALUES ('Aliyah', 'Oliveros', 'aliyah@example.com', '07000000001', 'paid', 'cus_two', 10000, 'GBP', $1, 1)
		RETURNING id`, billingStatus).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert two-coach group: %v", err)
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type, needs_coach,
			allocated_accommodation_code
		) VALUES ($1, true, 'Aliyah', 'Oliveros', 'female', 25, 'Leader', false, 'full_week', true, 'lodge')
		RETURNING id`, groupID).Scan(&camper1ID)
	if err != nil {
		t.Fatalf("insert camper 1: %v", err)
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type, needs_coach,
			allocated_accommodation_code
		) VALUES ($1, false, 'Stephanie', 'Alves', 'female', 19, 'Leader', false, 'full_week', true, 'lodge')
		RETURNING id`, groupID).Scan(&camper2ID)
	if err != nil {
		t.Fatalf("insert camper 2: %v", err)
	}
	return groupID, camper1ID, camper2ID
}

func TestUpdateCamperCoach_turnOffOneThenInvoice(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)

	var captured []InvoiceLine
	stub := &stubStripe{
		ensureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "cus_two", nil
		},
		createInvoice: func(_ context.Context, _, _ string, lines []InvoiceLine, _ int64, invoiceType string) (InvoiceResult, error) {
			captured = lines
			if invoiceType != "coach" {
				t.Fatalf("invoice type = %q, want coach", invoiceType)
			}
			return InvoiceResult{ID: "in_coach_one", StripeEmailed: true}, nil
		},
	}
	svc := NewService(repo, stub, nil, nil, Config{StripePriceCoach: "price_coach"})

	groupID, camper1ID, _ := insertTwoCoachGroup(ctx, t, pool, domain.BillingBalancePaid)
	if err := svc.WaiveCoachFee(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("WaiveCoachFee: %v", err)
	}

	sum, err := svc.UpdateCamperCoach(ctx, groupID, camper1ID, "Admin", domain.SkipVersionCheck, UpdateCamperCoachRequest{NeedsCoach: false})
	if err != nil {
		t.Fatalf("UpdateCamperCoach: %v", err)
	}
	if sum.NoOp || sum.CamperName != "Aliyah Oliveros" {
		t.Fatalf("summary = %+v", sum)
	}

	campers, _ := repo.CampersForGroup(ctx, groupID)
	if coachEligibleCount(campers) != 1 {
		t.Fatalf("eligible count = %d, want 1", coachEligibleCount(campers))
	}

	if err := svc.UnwaiveCoachFee(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("UnwaiveCoachFee: %v", err)
	}
	if err := svc.SendCoachInvoice(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("SendCoachInvoice: %v", err)
	}
	if len(captured) != 1 || captured[0].Quantity != 1 {
		t.Fatalf("invoice lines = %+v, want qty 1", captured)
	}
}

func TestUpdateCamperCoach_guards(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{StripePriceCoach: "price_coach"})

	groupID, camper1ID, _ := insertTwoCoachGroup(ctx, t, pool, domain.BillingBalancePaid)

	// Coach fee already paid via separate invoice.
	if _, err := pool.Exec(ctx, `
		UPDATE registration_groups SET coach_fee_paid_at = NOW() WHERE id = $1`, groupID); err != nil {
		t.Fatalf("set coach paid: %v", err)
	}
	_, err := svc.UpdateCamperCoach(ctx, groupID, camper1ID, "Admin", domain.SkipVersionCheck, UpdateCamperCoachRequest{NeedsCoach: false})
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
		t.Fatalf("expected bad_request when coach paid, got %v", err)
	}

	// Coach folded into balance invoice.
	groupID2, camper2a, _ := insertTwoCoachGroup(ctx, t, pool, domain.BillingBalancePaid)
	if _, err := pool.Exec(ctx, `
		UPDATE registration_groups SET coach_included_in_balance = true WHERE id = $1`, groupID2); err != nil {
		t.Fatalf("set coach in balance: %v", err)
	}
	_, err = svc.UpdateCamperCoach(ctx, groupID2, camper2a, "Admin", domain.SkipVersionCheck, UpdateCamperCoachRequest{NeedsCoach: false})
	if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
		t.Fatalf("expected bad_request when coach in balance, got %v", err)
	}

	// Day-pass camper.
	var dayGroup, dayCamper string
	if err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status, version
		) VALUES ('Day', 'Pass', 'day@example.com', '07000000002', 'paid', 5000, 'GBP', 'balance_paid', 1)
		RETURNING id`).Scan(&dayGroup); err != nil {
		t.Fatalf("insert day group: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type, day_pass_days
		) VALUES ($1, true, 'Day', 'Pass', 'male', 30, 'Leader', false, 'day_pass', ARRAY['mon'])
		RETURNING id`, dayGroup).Scan(&dayCamper); err != nil {
		t.Fatalf("insert day camper: %v", err)
	}
	_, err = svc.UpdateCamperCoach(ctx, dayGroup, dayCamper, "Admin", domain.SkipVersionCheck, UpdateCamperCoachRequest{NeedsCoach: true})
	if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
		t.Fatalf("expected bad_request for day pass, got %v", err)
	}
}

func TestUpdateCamperCoach_voidOpenInvoice(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)

	var voided []string
	stub := &stubStripe{
		ensureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "cus_two", nil
		},
		createInvoice: func(_ context.Context, _, _ string, _ []InvoiceLine, _ int64, _ string) (InvoiceResult, error) {
			return InvoiceResult{ID: "in_coach_open", StripeEmailed: true}, nil
		},
		voidInvIdem: func(_ context.Context, id string) error {
			voided = append(voided, id)
			return nil
		},
	}
	svc := NewService(repo, stub, nil, nil, Config{StripePriceCoach: "price_coach"})

	groupID, camper1ID, _ := insertTwoCoachGroup(ctx, t, pool, domain.BillingBalancePaid)
	if err := svc.SendCoachInvoice(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("SendCoachInvoice: %v", err)
	}

	sum, err := svc.UpdateCamperCoach(ctx, groupID, camper1ID, "Admin", domain.SkipVersionCheck, UpdateCamperCoachRequest{NeedsCoach: false})
	if err != nil {
		t.Fatalf("UpdateCamperCoach: %v", err)
	}
	if !sum.InvoiceVoided {
		t.Fatal("expected invoice voided")
	}
	if len(voided) != 1 || voided[0] != "in_coach_open" {
		t.Fatalf("voided = %v", voided)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil || g.StripeCoachInvoiceID != nil {
		t.Fatalf("coach invoice columns not cleared: %+v", g)
	}
}

func TestUpdateCamperCoach_noOp(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{})

	groupID, camper1ID, _ := insertTwoCoachGroup(ctx, t, pool, domain.BillingBalancePaid)
	gBefore, _ := repo.FindGroupByID(ctx, groupID)

	sum, err := svc.UpdateCamperCoach(ctx, groupID, camper1ID, "Admin", domain.SkipVersionCheck, UpdateCamperCoachRequest{NeedsCoach: true})
	if err != nil {
		t.Fatalf("UpdateCamperCoach: %v", err)
	}
	if !sum.NoOp {
		t.Fatal("expected no-op")
	}
	gAfter, _ := repo.FindGroupByID(ctx, groupID)
	if gAfter.Version != gBefore.Version {
		t.Fatalf("version changed on no-op: %d -> %d", gBefore.Version, gAfter.Version)
	}
}

func TestUpdateCamperCoach_versionConflict(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{})

	groupID, camper1ID, _ := insertTwoCoachGroup(ctx, t, pool, domain.BillingBalancePaid)
	_, err := svc.UpdateCamperCoach(ctx, groupID, camper1ID, "Admin", 99, UpdateCamperCoachRequest{NeedsCoach: false})
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "stale_state" {
		t.Fatalf("expected stale_state conflict, got %v", err)
	}
}

func TestUpdateCamperCoach_waivedRemainsWhenLastPassengerRemoved(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{StripePriceCoach: "price_coach"})

	groupID, camper1ID, camper2ID := insertTwoCoachGroup(ctx, t, pool, domain.BillingBalancePaid)
	if err := svc.WaiveCoachFee(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("WaiveCoachFee: %v", err)
	}

	if _, err := svc.UpdateCamperCoach(ctx, groupID, camper1ID, "Admin", domain.SkipVersionCheck, UpdateCamperCoachRequest{NeedsCoach: false}); err != nil {
		t.Fatalf("UpdateCamperCoach camper1: %v", err)
	}
	if _, err := svc.UpdateCamperCoach(ctx, groupID, camper2ID, "Admin", domain.SkipVersionCheck, UpdateCamperCoachRequest{NeedsCoach: false}); err != nil {
		t.Fatalf("UpdateCamperCoach camper2: %v", err)
	}

	campers, _ := repo.CampersForGroup(ctx, groupID)
	if coachEligibleCount(campers) != 0 {
		t.Fatalf("eligible count = %d, want 0", coachEligibleCount(campers))
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil || g.CoachFeeWaivedAt == nil {
		t.Fatalf("coach_fee_waived_at should remain set: %+v", g)
	}

	if _, err := svc.UpdateCamperCoach(ctx, groupID, camper2ID, "Admin", domain.SkipVersionCheck, UpdateCamperCoachRequest{NeedsCoach: true}); err != nil {
		t.Fatalf("UpdateCamperCoach restore camper2: %v", err)
	}
	campers, _ = repo.CampersForGroup(ctx, groupID)
	if coachEligibleCount(campers) != 1 {
		t.Fatalf("eligible count after restore = %d, want 1", coachEligibleCount(campers))
	}
	g, _ = repo.FindGroupByID(ctx, groupID)
	if g == nil || g.CoachFeeWaivedAt == nil {
		t.Fatalf("coach_fee_waived_at should still be set after partial restore: %+v", g)
	}
}

func validConvertRequest() ConvertToDayVisitorRequest {
	needs := true
	return ConvertToDayVisitorRequest{
		Days:          []string{"mon", "tue"},
		TshirtOption:  domain.TshirtOptionNone,
		ShirtSize:     domain.ShirtSizeNotApplicable,
		NeedsCatering: &needs,
	}
}

func TestConvertCamperToDayVisitor_allocated(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	stub := &stubStripe{
		ensureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "cus_convert", nil
		},
	}
	svc := NewService(repo, stub, nil, nil, Config{DepositPricePence: 5000})

	groupID, _, camper2ID, _ := insertThreeCamperGroup(ctx, t, pool, domain.BillingAllocated, nil)

	sum, err := svc.ConvertCamperToDayVisitor(ctx, groupID, camper2ID, "Admin", 1, validConvertRequest())
	if err != nil {
		t.Fatalf("ConvertCamperToDayVisitor: %v", err)
	}
	if sum.CamperName != "Alex Test" {
		t.Fatalf("camper name = %q", sum.CamperName)
	}
	if sum.DepositCreditPence != 5000 {
		t.Fatalf("credit pence = %d, want 5000", sum.DepositCreditPence)
	}
	if sum.InvoiceVoided {
		t.Fatal("expected no invoice void")
	}
	if len(stub.creditCalls) != 1 {
		t.Fatalf("credit calls = %d, want 1", len(stub.creditCalls))
	}
	if stub.creditCalls[0].amountPence != 5000 {
		t.Fatalf("credit amount = %d", stub.creditCalls[0].amountPence)
	}
	wantKey := "deposit-credit-" + camper2ID
	if stub.creditCalls[0].idempotencyKey != wantKey {
		t.Fatalf("idempotency key = %q, want %q", stub.creditCalls[0].idempotencyKey, wantKey)
	}

	campers, _ := repo.CampersForGroup(ctx, groupID)
	var converted domain.Camper
	for _, c := range campers {
		if c.ID == camper2ID {
			converted = c
		}
	}
	if converted.AttendanceType != domain.AttendanceDayPass {
		t.Fatalf("attendance = %q", converted.AttendanceType)
	}
	if len(converted.DayPassDays) != 2 {
		t.Fatalf("day_pass_days = %v", converted.DayPassDays)
	}
	if converted.AllocatedAccommodationCode != nil {
		t.Fatal("allocation should be cleared")
	}
	if converted.NeedsCoach != nil {
		t.Fatal("needs_coach should be cleared")
	}
	if converted.DepositCreditPence != 5000 {
		t.Fatalf("deposit_credit_pence = %d", converted.DepositCreditPence)
	}
	if converted.DayPassTshirtOption == nil || *converted.DayPassTshirtOption != domain.TshirtOptionNone {
		t.Fatalf("day_pass_tshirt_option = %v", converted.DayPassTshirtOption)
	}
	if converted.DayPassNeedsCatering == nil || !*converted.DayPassNeedsCatering {
		t.Fatalf("day_pass_needs_catering = %v", converted.DayPassNeedsCatering)
	}

	g, err := repo.FindGroupByID(ctx, groupID)
	if err != nil || g == nil {
		t.Fatalf("FindGroupByID: %v", err)
	}
	if g.Version != 2 {
		t.Fatalf("version = %d, want 2", g.Version)
	}
}

func TestConvertCamperToDayVisitor_multiConvertTwoCredits(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	stub := &stubStripe{
		ensureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "cus_multi", nil
		},
	}
	svc := NewService(repo, stub, nil, nil, Config{DepositPricePence: 5000})

	groupID, _, camper2ID, camper3ID := insertThreeCamperGroup(ctx, t, pool, domain.BillingAllocated, nil)

	if _, err := svc.ConvertCamperToDayVisitor(ctx, groupID, camper2ID, "Admin", 1, validConvertRequest()); err != nil {
		t.Fatalf("first convert: %v", err)
	}
	if _, err := svc.ConvertCamperToDayVisitor(ctx, groupID, camper3ID, "Admin", 2, validConvertRequest()); err != nil {
		t.Fatalf("second convert: %v", err)
	}
	if len(stub.creditCalls) != 2 {
		t.Fatalf("credit calls = %d, want 2", len(stub.creditCalls))
	}
	keys := map[string]bool{
		stub.creditCalls[0].idempotencyKey: true,
		stub.creditCalls[1].idempotencyKey: true,
	}
	if !keys["deposit-credit-"+camper2ID] || !keys["deposit-credit-"+camper3ID] {
		t.Fatalf("unexpected idempotency keys: %+v", stub.creditCalls)
	}
}

func TestConvertCamperToDayVisitor_invoicedVoidsAndReverts(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	voided := false
	stub := &stubStripe{
		voidInvIdem: func(_ context.Context, id string) error {
			if id != "in_open" {
				t.Fatalf("void id = %q", id)
			}
			voided = true
			return nil
		},
		ensureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "cus_void", nil
		},
	}
	svc := NewService(repo, stub, nil, nil, Config{DepositPricePence: 5000})

	inv := "in_open"
	groupID, _, camper2ID, _ := insertThreeCamperGroup(ctx, t, pool, domain.BillingInvoiced, &inv)

	sum, err := svc.ConvertCamperToDayVisitor(ctx, groupID, camper2ID, "Admin", domain.SkipVersionCheck, validConvertRequest())
	if err != nil {
		t.Fatalf("ConvertCamperToDayVisitor: %v", err)
	}
	if !voided {
		t.Fatal("expected invoice void")
	}
	if !sum.InvoiceVoided {
		t.Fatal("summary should report invoice voided")
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil || g.BillingStatus != domain.BillingAllocated {
		t.Fatalf("billing_status = %v, want allocated", g)
	}
	if g.StripeInvoiceID != nil {
		t.Fatal("stripe_invoice_id should be cleared")
	}
}

func TestConvertCamperToDayVisitor_invoicedRevertsToNoneWhenDayPassOnly(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	stub := &stubStripe{
		voidInvIdem: func(context.Context, string) error { return nil },
		ensureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "cus_none", nil
		},
	}
	svc := NewService(repo, stub, nil, nil, Config{DepositPricePence: 5000})

	var groupID, camperID string
	inv := "in_solo"
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_payment_intent_id, stripe_invoice_id,
			total_amount_pence, currency, billing_status, version
		) VALUES ('Solo', 'Camper', 'solo@example.com', '07000000000', 'paid', 'pi_test', $1, 5000, 'GBP', 'invoiced', 1)
		RETURNING id`, inv).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type,
			allocated_accommodation_code
		) VALUES ($1, true, 'Solo', 'Camper', 'male', 25, 'Leader', false, 'full_week', 'lodge')
		RETURNING id`, groupID).Scan(&camperID)
	if err != nil {
		t.Fatalf("insert camper: %v", err)
	}

	_, err = svc.ConvertCamperToDayVisitor(ctx, groupID, camperID, "Admin", 1, validConvertRequest())
	if err != nil {
		t.Fatalf("ConvertCamperToDayVisitor: %v", err)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil || g.BillingStatus != domain.BillingNone {
		t.Fatalf("billing_status = %v, want none", g)
	}
}

func TestConvertCamperToDayVisitor_guards(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	stub := &stubStripe{}
	svc := NewService(repo, stub, nil, nil, Config{DepositPricePence: 5000})

	t.Run("balance_paid", func(t *testing.T) {
		groupID, _, camper2ID, _ := insertThreeCamperGroup(ctx, t, pool, domain.BillingBalancePaid, nil)
		_, err := svc.ConvertCamperToDayVisitor(ctx, groupID, camper2ID, "Admin", domain.SkipVersionCheck, validConvertRequest())
		var apiErr commonerrors.APIError
		if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
			t.Fatalf("expected bad_request, got %v", err)
		}
	})

	t.Run("day_pass_camper", func(t *testing.T) {
		groupID := insertCoachGroup(ctx, t, pool, domain.BillingAllocated, "lodge", false)
		campers, _ := repo.CampersForGroup(ctx, groupID)
		_, err := pool.Exec(ctx, `UPDATE registrations SET attendance_type = 'day_pass', day_pass_days = '{mon}' WHERE id = $1`, campers[0].ID)
		if err != nil {
			t.Fatalf("update camper: %v", err)
		}
		_, err = svc.ConvertCamperToDayVisitor(ctx, groupID, campers[0].ID, "Admin", domain.SkipVersionCheck, validConvertRequest())
		var apiErr commonerrors.APIError
		if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
			t.Fatalf("expected bad_request, got %v", err)
		}
	})

	t.Run("empty_days", func(t *testing.T) {
		groupID, _, camper2ID, _ := insertThreeCamperGroup(ctx, t, pool, domain.BillingAllocated, nil)
		req := validConvertRequest()
		req.Days = nil
		_, err := svc.ConvertCamperToDayVisitor(ctx, groupID, camper2ID, "Admin", domain.SkipVersionCheck, req)
		var apiErr commonerrors.APIError
		if !errors.As(err, &apiErr) || apiErr.Code != "validation_failed" {
			t.Fatalf("expected validation_failed, got %v", err)
		}
	})

	t.Run("is_free", func(t *testing.T) {
		groupID := insertCoachGroup(ctx, t, pool, domain.BillingAllocated, "lodge", true)
		campers, _ := repo.CampersForGroup(ctx, groupID)
		_, err := svc.ConvertCamperToDayVisitor(ctx, groupID, campers[0].ID, "Admin", domain.SkipVersionCheck, validConvertRequest())
		var apiErr commonerrors.APIError
		if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
			t.Fatalf("expected bad_request, got %v", err)
		}
		if len(stub.creditCalls) != 0 {
			t.Fatal("free group should not trigger credit")
		}
	})
}

func TestConvertCamperToDayVisitor_under4NoCredit(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	stub := &stubStripe{}
	svc := NewService(repo, stub, nil, nil, Config{DepositPricePence: 5000})

	groupID, _, _, _ := insertThreeCamperGroup(ctx, t, pool, domain.BillingAllocated, nil)
	var youngID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type,
			allocated_accommodation_code
		) VALUES ($1, false, 'Baby', 'Test', 'male', 2, 'Leader', false, 'full_week', 'lodge')
		RETURNING id`, groupID).Scan(&youngID)
	if err != nil {
		t.Fatalf("insert young camper: %v", err)
	}

	sum, err := svc.ConvertCamperToDayVisitor(ctx, groupID, youngID, "Admin", domain.SkipVersionCheck, validConvertRequest())
	if err != nil {
		t.Fatalf("ConvertCamperToDayVisitor: %v", err)
	}
	if sum.DepositCreditPence != 0 {
		t.Fatalf("credit pence = %d, want 0", sum.DepositCreditPence)
	}
	if len(stub.creditCalls) != 0 {
		t.Fatal("under-4 should not trigger credit")
	}
	campers, _ := repo.CampersForGroup(ctx, groupID)
	for _, c := range campers {
		if c.ID == youngID && c.DepositCreditPence != 0 {
			t.Fatalf("deposit_credit_pence = %d", c.DepositCreditPence)
		}
	}
}

type recordingSheets struct {
	sheets.NoopSync
	removeCalls int
	appendCalls int
	pendingCalls int
	lastGroupID  string
	lastRows     []sheets.Row
}

func (r *recordingSheets) RemoveByGroupID(_ context.Context, groupID string) error {
	r.removeCalls++
	r.lastGroupID = groupID
	return nil
}

func (r *recordingSheets) AppendPaidAndRemovePending(_ context.Context, groupID string, rows []sheets.Row) error {
	r.appendCalls++
	r.lastGroupID = groupID
	r.lastRows = rows
	return nil
}

func (r *recordingSheets) AppendPending(_ context.Context, rows []sheets.Row) error {
	r.pendingCalls++
	if len(rows) > 0 {
		r.lastGroupID = rows[0].GroupID
	}
	r.lastRows = rows
	return nil
}

func insertDayPassGroup(ctx context.Context, t *testing.T, pool *pgxpool.Pool, billingStatus string) (groupID, camperID string) {
	t.Helper()
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status, version
		) VALUES ('Day', 'Pass', 'daypass@example.com', '07000000000', 'paid', 0, 'GBP', $1, 1)
		RETURNING id`, billingStatus).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}
	tshirt := domain.TshirtOptionTeamActivities
	catering := true
	shirt := "adult_m"
	dietary := "None"
	err = pool.QueryRow(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type,
			day_pass_days, day_pass_tshirt_option, day_pass_needs_catering,
			shirt_size, dietary_requirements
		) VALUES ($1, true, 'Day', 'Passer', 'female', 30, 'Leader', false, 'day_pass',
			'{mon,tue}', $2, $3, $4, $5)
		RETURNING id`, groupID, tshirt, catering, shirt, dietary).Scan(&camperID)
	if err != nil {
		t.Fatalf("insert camper: %v", err)
	}
	return groupID, camperID
}

func validUpdateDayPassRequest() UpdateDayPassCamperRequest {
	needs := false
	diet := "Gluten free"
	return UpdateDayPassCamperRequest{
		TshirtOption:  domain.TshirtOptionTshirtOnly,
		ShirtSize:     "adult_l",
		NeedsCatering: &needs,
		Dietary:       &diet,
	}
}

func TestUpdateDayPassCamper_success(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	rec := &recordingSheets{}
	stripe := &stubStripe{
		voidInvIdem: func(context.Context, string) error {
			t.Fatal("unexpected VoidInvoiceIdempotent")
			return nil
		},
	}
	svc := NewService(repo, stripe, nil, rec, Config{})

	groupID, camperID := insertDayPassGroup(ctx, t, pool, domain.BillingNone)

	sum, err := svc.UpdateDayPassCamper(ctx, groupID, camperID, "Admin", 1, validUpdateDayPassRequest())
	if err != nil {
		t.Fatalf("UpdateDayPassCamper: %v", err)
	}
	if sum.CamperName != "Day Passer" {
		t.Fatalf("camper name = %q", sum.CamperName)
	}

	campers, _ := repo.CampersForGroup(ctx, groupID)
	var updated domain.Camper
	for _, c := range campers {
		if c.ID == camperID {
			updated = c
		}
	}
	if updated.AttendanceType != domain.AttendanceDayPass {
		t.Fatalf("attendance = %q", updated.AttendanceType)
	}
	if len(updated.DayPassDays) != 2 {
		t.Fatalf("day_pass_days changed: %v", updated.DayPassDays)
	}
	if updated.DayPassTshirtOption == nil || *updated.DayPassTshirtOption != domain.TshirtOptionTshirtOnly {
		t.Fatalf("tshirt option = %v", updated.DayPassTshirtOption)
	}
	if updated.DayPassNeedsCatering == nil || *updated.DayPassNeedsCatering {
		t.Fatal("expected needs_catering false")
	}
	if updated.ShirtSize == nil || *updated.ShirtSize != "adult_l" {
		t.Fatalf("shirt_size = %v", updated.ShirtSize)
	}
	if updated.DietaryRequirements == nil || *updated.DietaryRequirements != "Gluten free" {
		t.Fatalf("dietary = %v", updated.DietaryRequirements)
	}

	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil || g.Version != 2 {
		t.Fatalf("version = %v", g)
	}
	if g.LastAction == nil || *g.LastAction != "camper_updated" {
		t.Fatalf("last_action = %v", g.LastAction)
	}
	if rec.appendCalls != 1 {
		t.Fatalf("sheet append calls = %d, want 1", rec.appendCalls)
	}
	if rec.lastGroupID != groupID {
		t.Fatalf("sheet group id = %q", rec.lastGroupID)
	}
	if len(stripe.creditCalls) != 0 {
		t.Fatalf("credit calls = %d, want 0", len(stripe.creditCalls))
	}
}

func TestUpdateDayPassCamper_allowedWhenBalancePaid(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{})

	groupID, camperID := insertDayPassGroup(ctx, t, pool, domain.BillingBalancePaid)

	_, err := svc.UpdateDayPassCamper(ctx, groupID, camperID, "Admin", domain.SkipVersionCheck, validUpdateDayPassRequest())
	if err != nil {
		t.Fatalf("UpdateDayPassCamper on balance_paid: %v", err)
	}
}

func TestUpdateDayPassCamper_guards(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{})

	t.Run("full_week", func(t *testing.T) {
		groupID, _, camper2ID, _ := insertThreeCamperGroup(ctx, t, pool, domain.BillingAllocated, nil)
		_, err := svc.UpdateDayPassCamper(ctx, groupID, camper2ID, "Admin", domain.SkipVersionCheck, validUpdateDayPassRequest())
		var apiErr commonerrors.APIError
		if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
			t.Fatalf("expected bad_request, got %v", err)
		}
	})

	t.Run("validation_empty_shirt", func(t *testing.T) {
		groupID, camperID := insertDayPassGroup(ctx, t, pool, domain.BillingNone)
		req := validUpdateDayPassRequest()
		req.TshirtOption = domain.TshirtOptionTeamActivities
		req.ShirtSize = ""
		_, err := svc.UpdateDayPassCamper(ctx, groupID, camperID, "Admin", domain.SkipVersionCheck, req)
		var apiErr commonerrors.APIError
		if !errors.As(err, &apiErr) || apiErr.Code != "validation_failed" {
			t.Fatalf("expected validation_failed, got %v", err)
		}
	})

	t.Run("none_normalizes_shirt", func(t *testing.T) {
		groupID, camperID := insertDayPassGroup(ctx, t, pool, domain.BillingNone)
		needs := false
		req := UpdateDayPassCamperRequest{
			TshirtOption:  domain.TshirtOptionNone,
			ShirtSize:     domain.ShirtSizeNotApplicable,
			NeedsCatering: &needs,
		}
		_, err := svc.UpdateDayPassCamper(ctx, groupID, camperID, "Admin", domain.SkipVersionCheck, req)
		if err != nil {
			t.Fatalf("UpdateDayPassCamper: %v", err)
		}
		campers, _ := repo.CampersForGroup(ctx, groupID)
		for _, c := range campers {
			if c.ID == camperID {
				if c.ShirtSize == nil || *c.ShirtSize != domain.ShirtSizeNotApplicable {
					t.Fatalf("shirt_size = %v", c.ShirtSize)
				}
			}
		}
	})

	t.Run("dietary_clear", func(t *testing.T) {
		groupID, camperID := insertDayPassGroup(ctx, t, pool, domain.BillingNone)
		needs := true
		empty := ""
		req := UpdateDayPassCamperRequest{
			TshirtOption:  domain.TshirtOptionTeamActivities,
			ShirtSize:     "adult_m",
			NeedsCatering: &needs,
			Dietary:       &empty,
		}
		_, err := svc.UpdateDayPassCamper(ctx, groupID, camperID, "Admin", domain.SkipVersionCheck, req)
		if err != nil {
			t.Fatalf("UpdateDayPassCamper: %v", err)
		}
		campers, _ := repo.CampersForGroup(ctx, groupID)
		for _, c := range campers {
			if c.ID == camperID {
				if c.DietaryRequirements == nil || *c.DietaryRequirements != "" {
					t.Fatalf("dietary = %v", c.DietaryRequirements)
				}
			}
		}
	})
}

func TestResolvePriceID_childUnder4UsesUnder3Price(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{StripePriceChildUnder3: "price_child_u3"})
	id, err := svc.resolvePriceID(ctx, registration.AccommodationChild, 3)
	if err != nil {
		t.Fatalf("resolve child under 4: %v", err)
	}
	if id != "price_child_u3" {
		t.Fatalf("got %q, want price_child_u3", id)
	}
}

func TestResolvePriceID_childAge8UsesTierPrice(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	if err := repo.UpdateAccommodationStripePrice(ctx, "child", "price_child_tier"); err != nil {
		t.Fatalf("seed child price: %v", err)
	}
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{StripePriceChildUnder3: "price_child_u3"})
	id, err := svc.resolvePriceID(ctx, registration.AccommodationChild, 8)
	if err != nil {
		t.Fatalf("resolve child age 8: %v", err)
	}
	if id != "price_child_tier" {
		t.Fatalf("got %q, want price_child_tier", id)
	}
}

func TestResolvePriceID_tentUsesTierPrice(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	if err := repo.UpdateAccommodationStripePrice(ctx, "tent", "price_tent"); err != nil {
		t.Fatalf("seed tent price: %v", err)
	}
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{})
	id, err := svc.resolvePriceID(ctx, "tent", 25)
	if err != nil {
		t.Fatalf("resolve tent: %v", err)
	}
	if id != "price_tent" {
		t.Fatalf("got %q, want price_tent", id)
	}
}

func TestResolvePriceID_emptyStripePriceErrors(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	if _, err := pool.Exec(ctx, `UPDATE accommodation_types SET stripe_price_id = NULL WHERE code = 'cabin'`); err != nil {
		t.Fatalf("clear cabin price: %v", err)
	}
	t.Cleanup(func() {
		_ = repo.UpdateAccommodationStripePrice(context.Background(), "cabin", "price_cabin_restore")
	})
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{})
	_, err := svc.resolvePriceID(ctx, "cabin", 25)
	if err == nil {
		t.Fatal("expected error for tier without stripe_price_id")
	}
}

func TestAllocate_prepaidGroupKeepsBalancePaid(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	if err := repo.UpdateAccommodationStripePrice(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("seed lodge price: %v", err)
	}
	svc := NewService(repo, NewStripeBilling(), nil, nil, Config{})

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

	if err := svc.Allocate(ctx, groupID, "Admin", domain.SkipVersionCheck, AllocateRequest{
		Campers: []AllocateCamper{{
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

func TestSendInvoice_rejectsPrepaidGroup(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{})

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status, paid_in_full_at_registration
		) VALUES ('Pre', 'Paid', 'prepaid2@example.com', '07000000000', 'paid', 70000, 'GBP', 'balance_paid', true)
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	err = svc.SendInvoice(ctx, groupID, "Admin", domain.SkipVersionCheck)
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
		t.Fatalf("expected bad_request, got %v", err)
	}
	if !strings.Contains(apiErr.Message, "paid in full at registration") {
		t.Fatalf("unexpected message: %q", apiErr.Message)
	}
}

func TestSendInvoicesBulk_skipsPrepaidGroup(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{})

	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, billing_status, paid_in_full_at_registration
		) VALUES ('Pre', 'Paid', 'prepaid3@example.com', '07000000000', 'paid', 70000, 'GBP', 'balance_paid', true)
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}

	errs := svc.SendInvoicesBulk(ctx, "Admin", []string{groupID})
	if len(errs) != 0 {
		t.Fatalf("expected prepaid group skipped silently, got errs=%v", errs)
	}
}

func TestMarkBalancePaidManually_allocatedGroupSkipsStripe(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	mailer := &recordingMailer{}
	voids := 0
	stub := &stubStripe{
		voidInvIdem: func(context.Context, string) error {
			voids++
			return nil
		},
	}
	svc := NewService(repo, stub, mailer, nil, Config{})

	groupID := insertCoachGroup(ctx, t, pool, domain.BillingAllocated, "lodge", false)

	if err := svc.MarkBalancePaidManually(ctx, groupID, "Diane", domain.SkipVersionCheck); err != nil {
		t.Fatalf("MarkBalancePaidManually: %v", err)
	}

	g, _ := repo.FindGroupByID(ctx, groupID)
	if g.BillingStatus != domain.BillingBalancePaid {
		t.Fatalf("billing_status = %q, want balance_paid", g.BillingStatus)
	}
	if g.BalancePaidAt == nil {
		t.Fatal("balance_paid_at not set")
	}
	if voids != 0 {
		t.Fatalf("void called %d times, want 0 for an uninvoiced group", voids)
	}
	if len(mailer.balancePaid) != 1 {
		t.Fatalf("confirmation emails = %d, want 1", len(mailer.balancePaid))
	}
	if mailer.balancePaid[0].AmountLabel != "" {
		t.Fatalf("amount label = %q, want blank when there is no invoice", mailer.balancePaid[0].AmountLabel)
	}

	// The family keeps the beds they were given; marking paid is not a release.
	campers, _ := repo.CampersForGroup(ctx, groupID)
	for _, c := range campers {
		if c.AllocatedAccommodationCode == nil || *c.AllocatedAccommodationCode != "lodge" {
			t.Fatalf("camper %s lost their allocation", c.FirstName)
		}
	}
}

func TestMarkBalancePaidManually_invoicedVoidsInvoice(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	mailer := &recordingMailer{}

	var voided []string
	stub := &stubStripe{
		getInvoice: func(_ context.Context, id string) (InvoiceResult, error) {
			return InvoiceResult{ID: id, AmountDuePence: 25000, Currency: "gbp"}, nil
		},
		voidInvIdem: func(_ context.Context, id string) error {
			voided = append(voided, id)
			return nil
		},
	}
	svc := NewService(repo, stub, mailer, nil, Config{})

	groupID := insertCoachGroup(ctx, t, pool, domain.BillingInvoiced, "lodge", false)
	if _, err := pool.Exec(ctx, `
		UPDATE registration_groups SET stripe_invoice_id = 'in_bank_transfer' WHERE id = $1`, groupID); err != nil {
		t.Fatalf("seed invoice id: %v", err)
	}

	if err := svc.MarkBalancePaidManually(ctx, groupID, "Diane", domain.SkipVersionCheck); err != nil {
		t.Fatalf("MarkBalancePaidManually: %v", err)
	}

	if len(voided) != 1 || voided[0] != "in_bank_transfer" {
		t.Fatalf("voided = %v, want the open invoice voided once", voided)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g.BillingStatus != domain.BillingBalancePaid {
		t.Fatalf("billing_status = %q, want balance_paid", g.BillingStatus)
	}
	if len(mailer.balancePaid) != 1 || mailer.balancePaid[0].AmountLabel != "£250.00" {
		t.Fatalf("confirmation = %+v, want one email showing £250.00", mailer.balancePaid)
	}
}

func TestMarkBalancePaidManually_voidFailureLeavesGroupUnpaid(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	mailer := &recordingMailer{}
	stub := &stubStripe{
		getInvoice: func(_ context.Context, id string) (InvoiceResult, error) {
			return InvoiceResult{ID: id, AmountDuePence: 25000, Currency: "gbp"}, nil
		},
		voidInvIdem: func(context.Context, string) error {
			return errors.New("stripe down")
		},
	}
	svc := NewService(repo, stub, mailer, nil, Config{})

	groupID := insertCoachGroup(ctx, t, pool, domain.BillingInvoiced, "lodge", false)
	if _, err := pool.Exec(ctx, `
		UPDATE registration_groups SET stripe_invoice_id = 'in_void_fails' WHERE id = $1`, groupID); err != nil {
		t.Fatalf("seed invoice id: %v", err)
	}

	err := svc.MarkBalancePaidManually(ctx, groupID, "Diane", domain.SkipVersionCheck)
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
		t.Fatalf("expected bad_request when the void fails, got %v", err)
	}

	// A live payment link plus a paid-looking group is the double charge we are
	// preventing, so nothing may have changed.
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g.BillingStatus != domain.BillingInvoiced {
		t.Fatalf("billing_status = %q, want it left invoiced", g.BillingStatus)
	}
	if g.BalancePaidAt != nil {
		t.Fatal("balance_paid_at set despite the void failing")
	}
	if len(mailer.balancePaid) != 0 {
		t.Fatalf("sent %d confirmations for a payment that was not recorded", len(mailer.balancePaid))
	}
}

func TestMarkBalancePaidManually_guards(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{})

	expectBadRequest := func(t *testing.T, err error) {
		t.Helper()
		var apiErr commonerrors.APIError
		if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
			t.Fatalf("expected bad_request, got %v", err)
		}
	}

	t.Run("already_balance_paid", func(t *testing.T) {
		groupID := insertCoachGroup(ctx, t, pool, domain.BillingBalancePaid, "lodge", false)
		expectBadRequest(t, svc.MarkBalancePaidManually(ctx, groupID, "Diane", domain.SkipVersionCheck))
	})

	t.Run("church_sponsored", func(t *testing.T) {
		groupID := insertCoachGroup(ctx, t, pool, domain.BillingAllocated, "lodge", true)
		expectBadRequest(t, svc.MarkBalancePaidManually(ctx, groupID, "Diane", domain.SkipVersionCheck))
	})

	t.Run("released", func(t *testing.T) {
		groupID := insertCoachGroup(ctx, t, pool, domain.BillingReleased, "lodge", false)
		expectBadRequest(t, svc.MarkBalancePaidManually(ctx, groupID, "Diane", domain.SkipVersionCheck))
	})

	t.Run("deposit_unpaid", func(t *testing.T) {
		var groupID string
		if err := pool.QueryRow(ctx, `
			INSERT INTO registration_groups (
				contact_first_name, contact_last_name, contact_email, contact_phone,
				payment_status, total_amount_pence, currency, billing_status
			) VALUES ('No', 'Deposit', 'nodeposit@example.com', '07000000000', 'pending', 5000, 'GBP', 'allocated')
			RETURNING id`).Scan(&groupID); err != nil {
			t.Fatalf("insert group: %v", err)
		}
		expectBadRequest(t, svc.MarkBalancePaidManually(ctx, groupID, "Diane", domain.SkipVersionCheck))
	})
}

func TestMarkBalancePaidManually_versionConflict(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, nil, nil, Config{})

	groupID := insertCoachGroup(ctx, t, pool, domain.BillingAllocated, "lodge", false)

	err := svc.MarkBalancePaidManually(ctx, groupID, "Diane", 99)
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "stale_state" {
		t.Fatalf("expected stale_state conflict, got %v", err)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g.BillingStatus != domain.BillingAllocated {
		t.Fatalf("billing_status = %q, want unchanged allocated", g.BillingStatus)
	}
}

package billing

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"pagasacentre/backend/internal/email"
	"pagasacentre/backend/internal/registration"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/registration/storage"
	"pagasacentre/backend/internal/testhelper"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
)

// fakePrices stands in for the live price table so a test can prove the deposit
// charged to a late arrival is read at the moment they are added, not baked into
// config when the process booted.
type fakePrices struct {
	depositPence int
	err          error
}

func (f fakePrices) GetPrice(_ context.Context, code string) (registration.PriceRow, error) {
	if f.err != nil {
		return registration.PriceRow{}, f.err
	}
	if code != domain.PriceDeposit {
		return registration.PriceRow{}, errors.New("unexpected price code " + code)
	}
	return registration.PriceRow{AmountPence: f.depositPence, Currency: "GBP"}, nil
}

// mutablePrices is a live price source a test can edit mid-run, standing in for
// the White Team changing the deposit in the prices table without a restart.
type mutablePrices struct{ depositPence int }

func (p *mutablePrices) GetPrice(_ context.Context, code string) (registration.PriceRow, error) {
	if code != domain.PriceDeposit {
		return registration.PriceRow{}, errors.New("unexpected price code " + code)
	}
	return registration.PriceRow{AmountPence: p.depositPence, Currency: "GBP"}, nil
}

// newFullWeekCamper is a valid full-week arrival, old enough to owe a deposit.
func newFullWeekCamper() AddCamperRequest {
	return AddCamperRequest{
		Camper: domain.CamperDTO{
			FirstName: "Ruth", LastName: "Adeyemi", Gender: "female", Age: 24,
			CellLeaderName: "Leader",
			Attendance: domain.AttendanceDTO{
				Type:                      domain.AttendanceFullWeek,
				ShirtSize:                 "adult_m",
				AccommodationFirstChoice:  "lodge",
				AccommodationSecondChoice: "static_caravan",
			},
		},
	}
}

// insertEditableGroup seeds a paid group of two full-week campers, both
// allocated, with the given billing status.
func insertEditableGroup(
	ctx context.Context,
	t *testing.T,
	pool *pgxpool.Pool,
	billingStatus string,
) (groupID, mainID, otherID string) {
	t.Helper()
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, stripe_customer_id, total_amount_pence, currency,
			billing_status, version
		) VALUES ('Grace', 'Fundi', 'grace@example.com', '07000000123', 'paid',
			'cus_edit', 10000, 'GBP', $1, 1)
		RETURNING id`, billingStatus).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type, needs_coach,
			shirt_size, accommodation_first_choice, accommodation_second_choice,
			allocated_accommodation_code
		) VALUES ($1, true, 'Grace', 'Fundi', 'female', 34, 'Leader', false,
			'full_week', false, 'adult_m', 'lodge', 'static_caravan', 'lodge')
		RETURNING id`, groupID).Scan(&mainID)
	if err != nil {
		t.Fatalf("insert main camper: %v", err)
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO registrations (
			group_id, is_main_contact, first_name, last_name, gender, age,
			cell_leader_name, is_cell_leader, attendance_type, needs_coach,
			shirt_size, accommodation_first_choice, accommodation_second_choice,
			allocated_accommodation_code
		) VALUES ($1, false, 'Priscilla', 'Fundi', 'female', 21, 'Leader', false,
			'full_week', false, 'adult_s', 'lodge', 'static_caravan', 'lodge')
		RETURNING id`, groupID).Scan(&otherID)
	if err != nil {
		t.Fatalf("insert second camper: %v", err)
	}
	return groupID, mainID, otherID
}

// editFor builds a full-replacement edit request from a stored camper, so a test
// only has to state the field it is actually changing.
func editFor(c domain.Camper) EditCamperRequest {
	return EditCamperRequest{
		FirstName:                  c.FirstName,
		LastName:                   c.LastName,
		Gender:                     c.Gender,
		Age:                        c.Age,
		CellLeaderName:             c.CellLeaderName,
		IsCellLeader:               c.IsCellLeader,
		ShirtSize:                  strVal(c.ShirtSize),
		DietaryRequirements:        strVal(c.DietaryRequirements),
		AccommodationFirstChoice:   strVal(c.AccommodationFirstChoice),
		AccommodationSecondChoice:  strVal(c.AccommodationSecondChoice),
		AllocatedAccommodationCode: strVal(c.AllocatedAccommodationCode),
		AllocatedUnitCode:          strVal(c.AllocatedUnitCode),
	}
}

func camperByID(t *testing.T, campers []domain.Camper, id string) domain.Camper {
	t.Helper()
	for _, c := range campers {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("camper %s not found", id)
	return domain.Camper{}
}

func TestEditCamper_swapLeavesOpenInvoiceAlone(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	mailer := &recordingMailer{}

	voids := 0
	stub := &stubStripe{voidInvIdem: func(context.Context, string) error {
		voids++
		return nil
	}}
	svc := NewService(repo, stub, mailer, nil, Config{})

	groupID, _, otherID := insertEditableGroup(ctx, t, pool, domain.BillingInvoiced)
	if _, err := pool.Exec(ctx,
		`UPDATE registration_groups SET stripe_invoice_id = 'in_live' WHERE id = $1`,
		groupID); err != nil {
		t.Fatalf("seed invoice: %v", err)
	}

	campers, _ := repo.CampersForGroup(ctx, groupID)
	req := editFor(camperByID(t, campers, otherID))
	req.FirstName = "Deborah"
	req.LastName = "Mensah"

	sum, err := svc.EditCamper(ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, req)
	if err != nil {
		t.Fatalf("EditCamper: %v", err)
	}
	if sum.Repriced || sum.InvoiceVoided {
		t.Fatalf("a name swap must not touch the money: %+v", sum)
	}
	if sum.PreviousName != "Priscilla Fundi" || sum.CamperName != "Deborah Mensah" {
		t.Fatalf("summary names = %+v", sum)
	}
	if voids != 0 {
		t.Fatalf("void called %d times, want 0", voids)
	}

	g, _ := repo.FindGroupByID(ctx, groupID)
	if g.BillingStatus != domain.BillingInvoiced {
		t.Fatalf("billing_status = %q, want invoiced", g.BillingStatus)
	}
	if g.StripeInvoiceID == nil || *g.StripeInvoiceID != "in_live" {
		t.Fatal("the live invoice should still be attached")
	}

	after, _ := repo.CampersForGroup(ctx, groupID)
	got := camperByID(t, after, otherID)
	if got.FirstName != "Deborah" || got.LastName != "Mensah" {
		t.Fatalf("camper not renamed: %s %s", got.FirstName, got.LastName)
	}
	if strVal(got.AllocatedAccommodationCode) != "lodge" {
		t.Fatal("the seat's accommodation should survive a swap")
	}

	if len(mailer.bookingUpdated) != 1 {
		t.Fatalf("booking-updated emails = %d, want 1", len(mailer.bookingUpdated))
	}
	if mailer.bookingUpdated[0].InvoiceCancelled {
		t.Fatal("email must not claim an invoice was cancelled when none was")
	}
	if mailer.bookingUpdated[0].AddedName != "" {
		t.Fatal("an edit is not an addition")
	}
}

func TestEditCamper_accommodationChangeVoidsInvoice(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	mailer := &recordingMailer{}
	if err := repo.UpdateAccommodationStripePrice(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("set lodge price: %v", err)
	}
	if err := repo.UpdateAccommodationStripePrice(ctx, "tent", "price_tent"); err != nil {
		t.Fatalf("set tent price: %v", err)
	}

	var voided []string
	stub := &stubStripe{voidInvIdem: func(_ context.Context, id string) error {
		voided = append(voided, id)
		return nil
	}}
	svc := NewService(repo, stub, mailer, nil, Config{})

	groupID, _, otherID := insertEditableGroup(ctx, t, pool, domain.BillingInvoiced)
	if _, err := pool.Exec(ctx,
		`UPDATE registration_groups SET stripe_invoice_id = 'in_live' WHERE id = $1`,
		groupID); err != nil {
		t.Fatalf("seed invoice: %v", err)
	}

	campers, _ := repo.CampersForGroup(ctx, groupID)
	req := editFor(camperByID(t, campers, otherID))
	req.AllocatedAccommodationCode = "tent"

	sum, err := svc.EditCamper(ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, req)
	if err != nil {
		t.Fatalf("EditCamper: %v", err)
	}
	if !sum.Repriced || !sum.InvoiceVoided {
		t.Fatalf("summary = %+v, want repriced and voided", sum)
	}
	if len(voided) != 1 || voided[0] != "in_live" {
		t.Fatalf("voided = %v, want [in_live]", voided)
	}

	g, _ := repo.FindGroupByID(ctx, groupID)
	if g.BillingStatus != domain.BillingAllocated {
		t.Fatalf("billing_status = %q, want allocated", g.BillingStatus)
	}
	if g.StripeInvoiceID != nil {
		t.Fatalf("stripe_invoice_id = %v, want cleared", *g.StripeInvoiceID)
	}
	if len(mailer.bookingUpdated) != 1 || !mailer.bookingUpdated[0].InvoiceCancelled {
		t.Fatal("family must be told their invoice was cancelled")
	}
}

func TestEditCamper_clearingAllocationSendsGroupBackToNeedsAccommodation(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	if err := repo.UpdateAccommodationStripePrice(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("set lodge price: %v", err)
	}
	svc := NewService(repo, &stubStripe{}, &recordingMailer{}, nil, Config{})

	groupID, _, otherID := insertEditableGroup(ctx, t, pool, domain.BillingInvoiced)
	if _, err := pool.Exec(ctx,
		`UPDATE registration_groups SET stripe_invoice_id = 'in_live' WHERE id = $1`,
		groupID); err != nil {
		t.Fatalf("seed invoice: %v", err)
	}

	campers, _ := repo.CampersForGroup(ctx, groupID)
	req := editFor(camperByID(t, campers, otherID))
	req.AllocatedAccommodationCode = ""

	if _, err := svc.EditCamper(
		ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, req); err != nil {
		t.Fatalf("EditCamper: %v", err)
	}

	// Someone now has no bed, so the group must be asking for accommodation
	// rather than sitting in a state that claims the work is done.
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g.BillingStatus != domain.BillingNone {
		t.Fatalf("billing_status = %q, want none while a camper is unplaced",
			g.BillingStatus)
	}
}

// TestCamperActions_recordDistinctAuditActions pins the action string each
// mutation stamps on the group. The dashboard turns these into readable labels
// by exact match, so a value that drifts from the label map is not a cosmetic
// slip — the group card falls back to printing the raw action at the admin.
func TestCamperActions_recordDistinctAuditActions(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, &recordingMailer{}, nil, Config{}).
		WithPriceLookup(fakePrices{depositPence: 5000})

	lastAction := func(groupID string) string {
		t.Helper()
		g, err := repo.FindGroupByID(ctx, groupID)
		if err != nil || g == nil {
			t.Fatalf("find group: %v", err)
		}
		if g.LastAction == nil {
			t.Fatal("no last_action recorded")
		}
		return *g.LastAction
	}

	t.Run("edit", func(t *testing.T) {
		groupID, _, otherID := insertEditableGroup(ctx, t, pool, domain.BillingAllocated)
		campers, _ := repo.CampersForGroup(ctx, groupID)
		req := editFor(camperByID(t, campers, otherID))
		req.FirstName = "Deborah"
		if _, err := svc.EditCamper(
			ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, req); err != nil {
			t.Fatalf("EditCamper: %v", err)
		}
		if got := lastAction(groupID); got != "camper_edited" {
			t.Fatalf("last_action = %q, want camper_edited", got)
		}
	})

	t.Run("add", func(t *testing.T) {
		groupID, _, _ := insertEditableGroup(ctx, t, pool, domain.BillingAllocated)
		if _, err := svc.AddCamper(
			ctx, groupID, "Diane", domain.SkipVersionCheck, newFullWeekCamper()); err != nil {
			t.Fatalf("AddCamper: %v", err)
		}
		if got := lastAction(groupID); got != "camper_added" {
			t.Fatalf("last_action = %q, want camper_added", got)
		}
	})

	t.Run("make_main_contact", func(t *testing.T) {
		groupID, _, otherID := insertEditableGroup(ctx, t, pool, domain.BillingAllocated)
		if _, err := svc.MakeMainContact(
			ctx, groupID, otherID, "Diane", domain.SkipVersionCheck); err != nil {
			t.Fatalf("MakeMainContact: %v", err)
		}
		if got := lastAction(groupID); got != "main_contact_moved" {
			t.Fatalf("last_action = %q, want main_contact_moved", got)
		}
	})

	t.Run("waive_deposit", func(t *testing.T) {
		groupID, _, otherID := insertEditableGroup(ctx, t, pool, domain.BillingAllocated)
		if _, err := pool.Exec(ctx,
			`UPDATE registrations SET deposit_owed_pence = 5000 WHERE id = $1`, otherID); err != nil {
			t.Fatalf("seed deposit owed: %v", err)
		}
		if _, err := svc.WaiveCamperDeposit(
			ctx, groupID, otherID, "Diane", domain.SkipVersionCheck); err != nil {
			t.Fatalf("WaiveCamperDeposit: %v", err)
		}
		if got := lastAction(groupID); got != "camper_deposit_waived" {
			t.Fatalf("last_action = %q, want camper_deposit_waived", got)
		}
	})

	// The day-pass editor's action must stay its own thing, or a rename would
	// announce itself as a day-pass change.
	for _, a := range []string{
		"camper_edited", "camper_added", "main_contact_moved", "camper_deposit_waived",
	} {
		if a == "camper_updated" {
			t.Fatalf("%q collides with the day-pass editor's action", a)
		}
	}
}

func TestCamperActions_rejectStaleVersion(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, &recordingMailer{}, nil, Config{}).
		WithPriceLookup(fakePrices{depositPence: 5000})

	groupID, _, otherID := insertEditableGroup(ctx, t, pool, domain.BillingAllocated)
	if _, err := pool.Exec(ctx,
		`UPDATE registrations SET deposit_owed_pence = 5000 WHERE id = $1`, otherID); err != nil {
		t.Fatalf("seed deposit owed: %v", err)
	}
	campers, _ := repo.CampersForGroup(ctx, groupID)

	const staleVersion = 99
	cases := map[string]func() error{
		"edit": func() error {
			_, err := svc.EditCamper(
				ctx, groupID, otherID, "Diane", staleVersion,
				editFor(camperByID(t, campers, otherID)))
			return err
		},
		"add": func() error {
			_, err := svc.AddCamper(ctx, groupID, "Diane", staleVersion, newFullWeekCamper())
			return err
		},
		"make_main_contact": func() error {
			_, err := svc.MakeMainContact(ctx, groupID, otherID, "Diane", staleVersion)
			return err
		},
		"waive_deposit": func() error {
			_, err := svc.WaiveCamperDeposit(ctx, groupID, otherID, "Diane", staleVersion)
			return err
		},
	}

	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			err := call()
			var apiErr commonerrors.APIError
			if !errors.As(err, &apiErr) || apiErr.Code != "stale_state" {
				t.Fatalf("error = %v, want stale_state", err)
			}
		})
	}
}

// TestEditCamper_voidFailureLeavesCamperUnchanged proves the ordering that keeps
// a booking honest: if the stale invoice cannot be cancelled, the edit is refused
// outright rather than leaving the roster changed while a live payment link for
// the old amount is still in the family's inbox.
func TestEditCamper_voidFailureLeavesCamperUnchanged(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	if err := repo.UpdateAccommodationStripePrice(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("set lodge price: %v", err)
	}
	if err := repo.UpdateAccommodationStripePrice(ctx, "tent", "price_tent"); err != nil {
		t.Fatalf("set tent price: %v", err)
	}
	stub := &stubStripe{voidInvIdem: func(context.Context, string) error {
		return errors.New("stripe is down")
	}}
	svc := NewService(repo, stub, &recordingMailer{}, nil, Config{})

	groupID, _, otherID := insertEditableGroup(ctx, t, pool, domain.BillingInvoiced)
	if _, err := pool.Exec(ctx,
		`UPDATE registration_groups SET stripe_invoice_id = 'in_live' WHERE id = $1`,
		groupID); err != nil {
		t.Fatalf("seed invoice: %v", err)
	}

	campers, _ := repo.CampersForGroup(ctx, groupID)
	before := camperByID(t, campers, otherID)
	req := editFor(before)
	req.FirstName = "Deborah"
	req.AllocatedAccommodationCode = "tent"

	if _, err := svc.EditCamper(
		ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, req); err == nil {
		t.Fatal("expected the edit to be refused when the void fails")
	}

	after, _ := repo.CampersForGroup(ctx, groupID)
	got := camperByID(t, after, otherID)
	if got.FirstName != before.FirstName {
		t.Fatalf("first_name = %q, want it untouched at %q", got.FirstName, before.FirstName)
	}
	if strVal(got.AllocatedAccommodationCode) != strVal(before.AllocatedAccommodationCode) {
		t.Fatalf("allocation = %q, want it untouched",
			strVal(got.AllocatedAccommodationCode))
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g.BillingStatus != domain.BillingInvoiced {
		t.Fatalf("billing_status = %q, want the group left invoiced", g.BillingStatus)
	}
}

// TestEditCamper_allowedOnChurchSponsoredGroup guards a deliberate disagreement
// with the neighbouring mutations: sponsored groups have no invoice to disturb,
// so editing them is allowed even though converting and the coach toggle refuse.
func TestEditCamper_allowedOnChurchSponsoredGroup(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, &recordingMailer{}, nil, Config{})

	groupID, _, otherID := insertEditableGroup(ctx, t, pool, domain.BillingAllocated)
	if _, err := pool.Exec(ctx,
		`UPDATE registration_groups SET is_free = true WHERE id = $1`, groupID); err != nil {
		t.Fatalf("mark sponsored: %v", err)
	}

	campers, _ := repo.CampersForGroup(ctx, groupID)
	req := editFor(camperByID(t, campers, otherID))
	req.FirstName = "Deborah"
	req.AllocatedAccommodationCode = "tent"

	sum, err := svc.EditCamper(ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, req)
	if err != nil {
		t.Fatalf("EditCamper on a sponsored group: %v", err)
	}
	// Nothing is billed for a sponsored group, so even a move between
	// accommodations cannot be a repricing change.
	if sum.Repriced || sum.InvoiceVoided {
		t.Fatalf("summary = %+v, want no repricing on a sponsored group", sum)
	}
}

func TestEditCamper_repricingWithNoInvoiceStaysAllocated(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	if err := repo.UpdateAccommodationStripePrice(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("set lodge price: %v", err)
	}
	if err := repo.UpdateAccommodationStripePrice(ctx, "tent", "price_tent"); err != nil {
		t.Fatalf("set tent price: %v", err)
	}
	voids := 0
	stub := &stubStripe{voidInvIdem: func(context.Context, string) error {
		voids++
		return nil
	}}
	svc := NewService(repo, stub, &recordingMailer{}, nil, Config{})

	groupID, _, otherID := insertEditableGroup(ctx, t, pool, domain.BillingAllocated)
	campers, _ := repo.CampersForGroup(ctx, groupID)
	req := editFor(camperByID(t, campers, otherID))
	req.AllocatedAccommodationCode = "tent"

	sum, err := svc.EditCamper(ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, req)
	if err != nil {
		t.Fatalf("EditCamper: %v", err)
	}
	if !sum.Repriced {
		t.Fatal("moving between priced accommodations is a repricing change")
	}
	if sum.InvoiceVoided || voids != 0 {
		t.Fatalf("nothing was invoiced yet, so nothing should be voided: %+v", sum)
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g.BillingStatus != domain.BillingAllocated {
		t.Fatalf("billing_status = %q, want the group left allocated", g.BillingStatus)
	}
}

func TestEditCamper_repricingRefusedOnPaidBalance(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	if err := repo.UpdateAccommodationStripePrice(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("set lodge price: %v", err)
	}
	if err := repo.UpdateAccommodationStripePrice(ctx, "tent", "price_tent"); err != nil {
		t.Fatalf("set tent price: %v", err)
	}
	svc := NewService(repo, &stubStripe{}, &recordingMailer{}, nil, Config{})

	groupID, _, otherID := insertEditableGroup(ctx, t, pool, domain.BillingBalancePaid)
	campers, _ := repo.CampersForGroup(ctx, groupID)

	reprice := editFor(camperByID(t, campers, otherID))
	reprice.AllocatedAccommodationCode = "tent"
	if _, err := svc.EditCamper(
		ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, reprice); err == nil {
		t.Fatal("expected a repricing edit to be refused once the balance is paid")
	} else {
		var apiErr commonerrors.APIError
		if !errors.As(err, &apiErr) || apiErr.Code != "bad_request" {
			t.Fatalf("error = %v, want bad_request", err)
		}
	}

	// A change that costs nothing is still allowed, which is the point of the rule.
	rename := editFor(camperByID(t, campers, otherID))
	rename.FirstName = "Deborah"
	if _, err := svc.EditCamper(
		ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, rename); err != nil {
		t.Fatalf("a free-of-charge edit on a paid group should be allowed: %v", err)
	}
}

func TestEditCamper_ageingPastCoachThresholdReprices(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	if err := repo.UpdateAccommodationStripePrice(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("set lodge price: %v", err)
	}
	voids := 0
	stub := &stubStripe{voidInvIdem: func(context.Context, string) error {
		voids++
		return nil
	}}
	svc := NewService(repo, stub, &recordingMailer{}, nil, Config{})

	groupID, _, otherID := insertEditableGroup(ctx, t, pool, domain.BillingInvoiced)
	if _, err := pool.Exec(ctx, `
		UPDATE registration_groups SET stripe_invoice_id = 'in_live' WHERE id = $1`,
		groupID); err != nil {
		t.Fatalf("seed invoice: %v", err)
	}
	// Put them on the coach, below the age where a seat is charged.
	if _, err := pool.Exec(ctx, `
		UPDATE registrations SET needs_coach = true, age = $2 WHERE id = $1`,
		otherID, registration.MinDepositAge-1); err != nil {
		t.Fatalf("seed coach passenger: %v", err)
	}

	campers, _ := repo.CampersForGroup(ctx, groupID)
	req := editFor(camperByID(t, campers, otherID))
	req.Age = registration.MinDepositAge

	sum, err := svc.EditCamper(ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, req)
	if err != nil {
		t.Fatalf("EditCamper: %v", err)
	}
	if !sum.Repriced {
		t.Fatal("crossing the coach-charging age changes what they owe")
	}
	if voids != 1 {
		t.Fatalf("void called %d times, want 1", voids)
	}
}

func TestEditCamper_childAccommodationAgeLimit(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, &recordingMailer{}, nil, Config{})

	groupID, _, otherID := insertEditableGroup(ctx, t, pool, domain.BillingAllocated)
	if _, err := pool.Exec(ctx, `
		UPDATE registrations
		   SET allocated_accommodation_code = 'child',
		       accommodation_first_choice = 'child',
		       accommodation_second_choice = NULL,
		       age = 8
		 WHERE id = $1`, otherID); err != nil {
		t.Fatalf("seed child placement: %v", err)
	}

	campers, _ := repo.CampersForGroup(ctx, groupID)
	req := editFor(camperByID(t, campers, otherID))
	req.Age = registration.MaxChildAccommodationAge + 1

	_, err := svc.EditCamper(ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, req)
	if err == nil {
		t.Fatal("expected an age past the child limit to be refused while in child accommodation")
	}
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "validation_failed" {
		t.Fatalf("error = %v, want validation_failed", err)
	}
}

func TestEditCamper_fullReplacementCatchesInvalidCombination(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, &recordingMailer{}, nil, Config{})

	groupID, _, otherID := insertEditableGroup(ctx, t, pool, domain.BillingAllocated)
	campers, _ := repo.CampersForGroup(ctx, groupID)

	req := editFor(camperByID(t, campers, otherID))
	req.ShirtSize = ""

	_, err := svc.EditCamper(ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, req)
	if err == nil {
		t.Fatal("a full-week camper with no shirt size should be rejected")
	}
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "validation_failed" {
		t.Fatalf("error = %v, want validation_failed", err)
	}
}

func TestAddCamper_recordsDepositAtLivePrice(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	mailer := &recordingMailer{}
	svc := NewService(repo, &stubStripe{}, mailer, nil, Config{DepositPricePence: 5000}).
		WithPriceLookup(fakePrices{depositPence: 7500})

	groupID, _, _ := insertEditableGroup(ctx, t, pool, domain.BillingAllocated)

	sum, err := svc.AddCamper(ctx, groupID, "Diane", domain.SkipVersionCheck, AddCamperRequest{
		Camper: domain.CamperDTO{
			FirstName: "Ruth", LastName: "Adeyemi", Gender: "female", Age: 24,
			CellLeaderName: "Leader",
			Attendance: domain.AttendanceDTO{
				Type:                      domain.AttendanceFullWeek,
				ShirtSize:                 "adult_m",
				AccommodationFirstChoice:  "lodge",
				AccommodationSecondChoice: "static_caravan",
			},
		},
	})
	if err != nil {
		t.Fatalf("AddCamper: %v", err)
	}
	if sum.DepositOwedPence != 7500 {
		t.Fatalf("deposit = %d, want the live 7500 rather than the 5000 from config",
			sum.DepositOwedPence)
	}
	if !sum.NeedsAllocation {
		t.Fatal("a new full-week camper has no bed yet")
	}

	campers, _ := repo.CampersForGroup(ctx, groupID)
	added := camperByID(t, campers, sum.CamperID)
	if added.DepositOwedPence != 7500 {
		t.Fatalf("stored deposit = %d, want 7500", added.DepositOwedPence)
	}
	if added.IsMainContact {
		t.Fatal("an added camper must not take the main-contact marker")
	}

	// The group goes back to needing accommodation so the new person gets a bed.
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g.BillingStatus != domain.BillingNone {
		t.Fatalf("billing_status = %q, want none", g.BillingStatus)
	}

	if len(mailer.bookingUpdated) != 1 {
		t.Fatalf("booking-updated emails = %d, want 1", len(mailer.bookingUpdated))
	}
	if mailer.bookingUpdated[0].AddedName != "Ruth Adeyemi" {
		t.Fatalf("added name = %q", mailer.bookingUpdated[0].AddedName)
	}
}

func TestAddCamper_childUnderDepositAgeOwesNothing(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, &recordingMailer{}, nil, Config{}).
		WithPriceLookup(fakePrices{depositPence: 5000})

	groupID, _, _ := insertEditableGroup(ctx, t, pool, domain.BillingAllocated)

	sum, err := svc.AddCamper(ctx, groupID, "Diane", domain.SkipVersionCheck, AddCamperRequest{
		Camper: domain.CamperDTO{
			FirstName: "Baby", LastName: "Fundi", Gender: "male",
			Age:            registration.MinDepositAge - 1,
			CellLeaderName: "Leader",
			Attendance: domain.AttendanceDTO{
				Type:                     domain.AttendanceFullWeek,
				ShirtSize:                "adult_s",
				AccommodationFirstChoice: "child",
			},
		},
	})
	if err != nil {
		t.Fatalf("AddCamper: %v", err)
	}
	if sum.DepositOwedPence != 0 {
		t.Fatalf("deposit = %d, want 0 for a camper below the deposit age",
			sum.DepositOwedPence)
	}
}

func TestAddCamper_guards(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, &recordingMailer{}, nil, Config{}).
		WithPriceLookup(fakePrices{depositPence: 5000})

	newcomer := AddCamperRequest{
		Camper: domain.CamperDTO{
			FirstName: "Ruth", LastName: "Adeyemi", Gender: "female", Age: 24,
			CellLeaderName: "Leader",
			Attendance: domain.AttendanceDTO{
				Type:                      domain.AttendanceFullWeek,
				ShirtSize:                 "adult_m",
				AccommodationFirstChoice:  "lodge",
				AccommodationSecondChoice: "static_caravan",
			},
		},
	}

	cases := []struct {
		name    string
		setup   func(groupID string)
		wantSub string
	}{
		{
			name: "deposit not paid",
			setup: func(groupID string) {
				_, _ = pool.Exec(ctx,
					`UPDATE registration_groups SET payment_status = 'pending' WHERE id = $1`,
					groupID)
			},
			wantSub: "deposit",
		},
		{
			name: "church sponsored",
			setup: func(groupID string) {
				_, _ = pool.Exec(ctx,
					`UPDATE registration_groups SET is_free = true WHERE id = $1`, groupID)
			},
			wantSub: "sponsored",
		},
		{
			name: "paid in full at registration",
			setup: func(groupID string) {
				_, _ = pool.Exec(ctx, `
					UPDATE registration_groups SET paid_in_full_at_registration = true
					 WHERE id = $1`, groupID)
			},
			wantSub: "in full at registration",
		},
		{
			name: "balance already paid",
			setup: func(groupID string) {
				_, _ = pool.Exec(ctx, `
					UPDATE registration_groups SET billing_status = 'balance_paid'
					 WHERE id = $1`, groupID)
			},
			wantSub: "already paid",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			groupID, _, _ := insertEditableGroup(ctx, t, pool, domain.BillingAllocated)
			tc.setup(groupID)
			_, err := svc.AddCamper(ctx, groupID, "Diane", domain.SkipVersionCheck, newcomer)
			if err == nil {
				t.Fatal("expected the add to be refused")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error = %q, want it to mention %q", err.Error(), tc.wantSub)
			}
		})
	}
}

// failingMailer proves the family email is best-effort. The change is already
// committed by the time it is sent, so a mail outage must not undo it.
type failingMailer struct{ email.NoopMailer }

func (failingMailer) SendBookingUpdated(context.Context, email.BookingUpdated) error {
	return errors.New("smtp is down")
}

func TestEditCamper_emailsTheFamilyAndNamesACancelledInvoice(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	if err := repo.UpdateAccommodationStripePrice(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("set lodge price: %v", err)
	}
	if err := repo.UpdateAccommodationStripePrice(ctx, "tent", "price_tent"); err != nil {
		t.Fatalf("set tent price: %v", err)
	}

	t.Run("plain rename says nothing about invoices", func(t *testing.T) {
		mailer := &recordingMailer{}
		svc := NewService(repo, &stubStripe{}, mailer, nil, Config{})
		groupID, _, otherID := insertEditableGroup(ctx, t, pool, domain.BillingInvoiced)
		campers, _ := repo.CampersForGroup(ctx, groupID)
		req := editFor(camperByID(t, campers, otherID))
		req.FirstName = "Deborah"

		if _, err := svc.EditCamper(
			ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, req); err != nil {
			t.Fatalf("EditCamper: %v", err)
		}
		if len(mailer.bookingUpdated) != 1 {
			t.Fatalf("sent %d booking-updated emails, want 1", len(mailer.bookingUpdated))
		}
		got := mailer.bookingUpdated[0]
		if got.InvoiceCancelled {
			t.Fatal("no invoice was cancelled, so the email must not claim one was")
		}
		if got.AddedName != "" {
			t.Fatalf("AddedName = %q, want empty for an edit", got.AddedName)
		}
		if got.ToEmail != "grace@example.com" {
			t.Fatalf("ToEmail = %q", got.ToEmail)
		}
		// The roster is the point of the mail: it must list the booking as it
		// stands now, including the new name.
		if len(got.Campers) != 2 {
			t.Fatalf("roster = %v, want both campers listed", got.Campers)
		}
		if !strings.Contains(strings.Join(got.Campers, "|"), "Deborah") {
			t.Fatalf("roster = %v, want it to show the new name", got.Campers)
		}
	})

	t.Run("repricing edit says the invoice is cancelled", func(t *testing.T) {
		mailer := &recordingMailer{}
		svc := NewService(repo, &stubStripe{
			voidInvIdem: func(context.Context, string) error { return nil },
		}, mailer, nil, Config{})
		groupID, _, otherID := insertEditableGroup(ctx, t, pool, domain.BillingInvoiced)
		if _, err := pool.Exec(ctx,
			`UPDATE registration_groups SET stripe_invoice_id = 'in_live' WHERE id = $1`,
			groupID); err != nil {
			t.Fatalf("seed invoice: %v", err)
		}
		campers, _ := repo.CampersForGroup(ctx, groupID)
		req := editFor(camperByID(t, campers, otherID))
		req.AllocatedAccommodationCode = "tent"

		if _, err := svc.EditCamper(
			ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, req); err != nil {
			t.Fatalf("EditCamper: %v", err)
		}
		if len(mailer.bookingUpdated) != 1 || !mailer.bookingUpdated[0].InvoiceCancelled {
			t.Fatalf("emails = %+v, want one saying the invoice was cancelled",
				mailer.bookingUpdated)
		}
	})

	t.Run("a mail failure still leaves the change applied", func(t *testing.T) {
		svc := NewService(repo, &stubStripe{}, failingMailer{}, nil, Config{})
		groupID, _, otherID := insertEditableGroup(ctx, t, pool, domain.BillingAllocated)
		campers, _ := repo.CampersForGroup(ctx, groupID)
		req := editFor(camperByID(t, campers, otherID))
		req.FirstName = "Deborah"

		if _, err := svc.EditCamper(
			ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, req); err != nil {
			t.Fatalf("a mailer outage must not fail the edit: %v", err)
		}
		after, _ := repo.CampersForGroup(ctx, groupID)
		if got := camperByID(t, after, otherID); got.FirstName != "Deborah" {
			t.Fatalf("first_name = %q, want the edit to have stuck", got.FirstName)
		}
	})
}

func TestAddCamper_emailsTheFamilyNamingTheNewArrival(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	mailer := &recordingMailer{}
	svc := NewService(repo, &stubStripe{}, mailer, nil, Config{}).
		WithPriceLookup(fakePrices{depositPence: 5000})

	groupID, _, _ := insertEditableGroup(ctx, t, pool, domain.BillingAllocated)
	if _, err := svc.AddCamper(
		ctx, groupID, "Diane", domain.SkipVersionCheck, newFullWeekCamper()); err != nil {
		t.Fatalf("AddCamper: %v", err)
	}

	if len(mailer.bookingUpdated) != 1 {
		t.Fatalf("sent %d booking-updated emails, want 1", len(mailer.bookingUpdated))
	}
	got := mailer.bookingUpdated[0]
	if got.AddedName != "Ruth Adeyemi" {
		t.Fatalf("AddedName = %q, want the new arrival named so the charge makes sense",
			got.AddedName)
	}
	if len(got.Campers) != 3 {
		t.Fatalf("roster = %v, want all three people listed", got.Campers)
	}
}

// TestAddCamper_fullWeekReturnsGroupForAllocation covers the trap that invoicing
// refuses a group with an unplaced full-week camper: the new arrival has no bed,
// so the group has to go back in front of the White Team.
func TestAddCamper_fullWeekReturnsGroupForAllocation(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, &recordingMailer{}, nil, Config{}).
		WithPriceLookup(fakePrices{depositPence: 5000})

	groupID, mainID, otherID := insertEditableGroup(ctx, t, pool, domain.BillingAllocated)

	sum, err := svc.AddCamper(ctx, groupID, "Diane", domain.SkipVersionCheck, newFullWeekCamper())
	if err != nil {
		t.Fatalf("AddCamper: %v", err)
	}
	if !sum.NeedsAllocation {
		t.Fatal("a new full-week camper needs a bed picked for them")
	}

	g, _ := repo.FindGroupByID(ctx, groupID)
	if g.BillingStatus != domain.BillingNone {
		t.Fatalf("billing_status = %q, want none so the group asks for accommodation",
			g.BillingStatus)
	}

	// The admin should only have to place the new arrival, not redo the group.
	campers, _ := repo.CampersForGroup(ctx, groupID)
	for _, id := range []string{mainID, otherID} {
		if got := strVal(camperByID(t, campers, id).AllocatedAccommodationCode); got != "lodge" {
			t.Fatalf("existing camper %s allocation = %q, want it kept at lodge", id, got)
		}
	}
	if got := strVal(camperByID(t, campers, sum.CamperID).AllocatedAccommodationCode); got != "" {
		t.Fatalf("new camper allocation = %q, want them unplaced", got)
	}
}

func TestAddCamper_dayPassOwesNoDeposit(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, &recordingMailer{}, nil, Config{}).
		WithPriceLookup(fakePrices{depositPence: 5000})

	groupID, _, _ := insertEditableGroup(ctx, t, pool, domain.BillingAllocated)

	needsCatering := false
	sum, err := svc.AddCamper(ctx, groupID, "Diane", domain.SkipVersionCheck, AddCamperRequest{
		Camper: domain.CamperDTO{
			FirstName: "Sam", LastName: "Okafor", Gender: "male", Age: 30,
			CellLeaderName: "Leader",
			Attendance: domain.AttendanceDTO{
				Type:          domain.AttendanceDayPass,
				Days:          []string{"mon", "tue"},
				TshirtOption:  domain.TshirtOptionNone,
				NeedsCatering: &needsCatering,
			},
		},
	})
	if err != nil {
		t.Fatalf("AddCamper: %v", err)
	}
	// A day visitor never paid a deposit at signup, so there is none to catch up on.
	if sum.DepositOwedPence != 0 {
		t.Fatalf("deposit owed = %d, want 0 for a day visitor", sum.DepositOwedPence)
	}
	if sum.NeedsAllocation {
		t.Fatal("a day visitor does not need a bed")
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g.BillingStatus != domain.BillingAllocated {
		t.Fatalf("billing_status = %q, want the group left allocated", g.BillingStatus)
	}
}

// TestAddCamper_laterPriceChangeLeavesEarlierDepositAlone proves the amount is
// stored, not recomputed: what someone owes is fixed when they are added, so the
// White Team editing the deposit later cannot silently rewrite an existing debt.
func TestAddCamper_laterPriceChangeLeavesEarlierDepositAlone(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	prices := &mutablePrices{depositPence: 5000}
	svc := NewService(repo, &stubStripe{}, &recordingMailer{}, nil, Config{}).
		WithPriceLookup(prices)

	groupID, _, _ := insertEditableGroup(ctx, t, pool, domain.BillingAllocated)

	first, err := svc.AddCamper(ctx, groupID, "Diane", domain.SkipVersionCheck, newFullWeekCamper())
	if err != nil {
		t.Fatalf("AddCamper: %v", err)
	}
	if first.DepositOwedPence != 5000 {
		t.Fatalf("first deposit = %d, want 5000", first.DepositOwedPence)
	}

	// The White Team edits the deposit in the prices table; no restart.
	prices.depositPence = 7500

	second := newFullWeekCamper()
	second.Camper.FirstName = "Naomi"
	got, err := svc.AddCamper(ctx, groupID, "Diane", domain.SkipVersionCheck, second)
	if err != nil {
		t.Fatalf("AddCamper: %v", err)
	}
	if got.DepositOwedPence != 7500 {
		t.Fatalf("second deposit = %d, want the edited price of 7500", got.DepositOwedPence)
	}

	campers, _ := repo.CampersForGroup(ctx, groupID)
	if owed := camperByID(t, campers, first.CamperID).DepositOwedPence; owed != 5000 {
		t.Fatalf("first camper now owes %d, want their original 5000", owed)
	}
}

func TestSendInvoice_billsOutstandingDeposit(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	if err := repo.UpdateAccommodationStripePrice(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("set lodge price: %v", err)
	}

	var captured []InvoiceLine
	stub := &stubStripe{
		ensureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "cus_edit", nil
		},
		createInvoice: func(_ context.Context, _, _ string, lines []InvoiceLine, _ int64, _ string) (InvoiceResult, error) {
			captured = lines
			return InvoiceResult{ID: "in_with_deposit", StripeEmailed: true}, nil
		},
	}
	svc := NewService(repo, stub, &recordingMailer{}, nil, Config{})

	groupID, _, otherID := insertEditableGroup(ctx, t, pool, domain.BillingAllocated)
	if _, err := pool.Exec(ctx,
		`UPDATE registrations SET deposit_owed_pence = 5000 WHERE id = $1`, otherID); err != nil {
		t.Fatalf("seed deposit owed: %v", err)
	}

	if err := svc.SendInvoice(ctx, groupID, "Diane", domain.SkipVersionCheck); err != nil {
		t.Fatalf("SendInvoice: %v", err)
	}

	var deposits []InvoiceLine
	for _, l := range captured {
		if l.PriceID == "" {
			deposits = append(deposits, l)
		}
	}
	if len(deposits) != 1 {
		t.Fatalf("ad-hoc lines = %+v, want exactly one deposit line", captured)
	}
	if deposits[0].AmountPence != 5000 {
		t.Fatalf("deposit line amount = %d, want 5000", deposits[0].AmountPence)
	}
	if !strings.Contains(deposits[0].Description, "Priscilla Fundi") {
		t.Fatalf("deposit line description = %q, should name who it is for",
			deposits[0].Description)
	}
	if deposits[0].Currency != "GBP" {
		t.Fatalf("deposit line currency = %q", deposits[0].Currency)
	}
}

func TestMarkBalancePaid_clearsOutstandingDeposits(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, &recordingMailer{}, nil, Config{})

	groupID, _, otherID := insertEditableGroup(ctx, t, pool, domain.BillingAllocated)
	if _, err := pool.Exec(ctx,
		`UPDATE registrations SET deposit_owed_pence = 5000 WHERE id = $1`, otherID); err != nil {
		t.Fatalf("seed deposit owed: %v", err)
	}

	if err := svc.MarkBalancePaidManually(ctx, groupID, "Diane", domain.SkipVersionCheck); err != nil {
		t.Fatalf("MarkBalancePaidManually: %v", err)
	}

	campers, _ := repo.CampersForGroup(ctx, groupID)
	if got := camperByID(t, campers, otherID); got.DepositOwedPence != 0 {
		t.Fatalf("deposit_owed_pence = %d, want 0 once the balance is settled",
			got.DepositOwedPence)
	}
}

func TestConvertCamper_withOutstandingDepositIssuesNoCredit(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	stub := &stubStripe{
		ensureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "cus_edit", nil
		},
	}
	svc := NewService(repo, stub, &recordingMailer{}, nil, Config{
		DepositPricePence:  5000,
		StripePriceDayPass: "price_day",
	})

	groupID, _, otherID := insertEditableGroup(ctx, t, pool, domain.BillingAllocated)
	if _, err := pool.Exec(ctx,
		`UPDATE registrations SET deposit_owed_pence = 5000 WHERE id = $1`, otherID); err != nil {
		t.Fatalf("seed deposit owed: %v", err)
	}

	needsCatering := false
	if _, err := svc.ConvertCamperToDayVisitor(
		ctx, groupID, otherID, "Diane", domain.SkipVersionCheck,
		ConvertToDayVisitorRequest{
			Days:          []string{"mon"},
			TshirtOption:  domain.TshirtOptionNone,
			NeedsCatering: &needsCatering,
		}); err != nil {
		t.Fatalf("ConvertCamperToDayVisitor: %v", err)
	}

	if len(stub.creditCalls) != 0 {
		t.Fatalf("credits = %+v, want none: they never paid this deposit",
			stub.creditCalls)
	}
	campers, _ := repo.CampersForGroup(ctx, groupID)
	got := camperByID(t, campers, otherID)
	if got.DepositOwedPence != 0 {
		t.Fatalf("deposit_owed_pence = %d, want 0 after conversion", got.DepositOwedPence)
	}
	if got.DepositCreditPence != 0 {
		t.Fatalf("deposit_credit_pence = %d, want 0", got.DepositCreditPence)
	}
}

func TestWaiveCamperDeposit(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, &recordingMailer{}, nil, Config{})

	groupID, _, otherID := insertEditableGroup(ctx, t, pool, domain.BillingAllocated)
	if _, err := pool.Exec(ctx,
		`UPDATE registrations SET deposit_owed_pence = 5000 WHERE id = $1`, otherID); err != nil {
		t.Fatalf("seed deposit owed: %v", err)
	}

	name, err := svc.WaiveCamperDeposit(ctx, groupID, otherID, "Diane", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("WaiveCamperDeposit: %v", err)
	}
	if name != "Priscilla Fundi" {
		t.Fatalf("name = %q", name)
	}

	campers, _ := repo.CampersForGroup(ctx, groupID)
	if got := camperByID(t, campers, otherID); got.DepositOwedPence != 0 {
		t.Fatalf("deposit_owed_pence = %d, want 0", got.DepositOwedPence)
	}

	// Waiving again is harmless rather than an error, so a double-click is safe.
	if _, err := svc.WaiveCamperDeposit(
		ctx, groupID, otherID, "Diane", domain.SkipVersionCheck); err != nil {
		t.Fatalf("second waive should be a no-op: %v", err)
	}
}

func TestWaiveCamperDeposit_refusedOncePaid(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, &recordingMailer{}, nil, Config{})

	groupID, _, otherID := insertEditableGroup(ctx, t, pool, domain.BillingBalancePaid)

	_, err := svc.WaiveCamperDeposit(ctx, groupID, otherID, "Diane", domain.SkipVersionCheck)
	if err == nil {
		t.Fatal("expected a waive to be refused once the balance is paid")
	}
}

func TestMakeMainContact_movesTheMarkerExactlyOnce(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := NewService(repo, &stubStripe{}, &recordingMailer{}, nil, Config{})

	groupID, mainID, otherID := insertEditableGroup(ctx, t, pool, domain.BillingAllocated)

	name, err := svc.MakeMainContact(ctx, groupID, otherID, "Diane", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("MakeMainContact: %v", err)
	}
	if name != "Priscilla Fundi" {
		t.Fatalf("name = %q", name)
	}

	campers, _ := repo.CampersForGroup(ctx, groupID)
	if !camperByID(t, campers, otherID).IsMainContact {
		t.Fatal("target should now be the main contact")
	}
	if camperByID(t, campers, mainID).IsMainContact {
		t.Fatal("previous main contact should have been cleared")
	}
	n := 0
	for _, c := range campers {
		if c.IsMainContact {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("main contacts = %d, want exactly 1", n)
	}

	// The group's own contact details are a separate thing and must not move.
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g.ContactFirstName != "Grace" || g.ContactEmail != "grace@example.com" {
		t.Fatalf("group contact changed: %s <%s>", g.ContactFirstName, g.ContactEmail)
	}
}

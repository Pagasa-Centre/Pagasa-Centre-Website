package camperedit_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/billing/billingtest"
	"pagasacentre/backend/internal/billing/camperedit"
	"pagasacentre/backend/internal/email"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/testhelper"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

func strPtr(s string) *string { return &s }

type failingMailer struct{ email.NoopMailer }

func (failingMailer) SendBookingUpdated(context.Context, email.BookingUpdated) error {
	return errors.New("smtp is down")
}

func countTierAllocations(ctx context.Context, pool *pgxpool.Pool, groupID, tier string) int {
	var n int
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM registrations
		WHERE group_id = $1 AND allocated_accommodation_code = $2`, groupID, tier).Scan(&n)
	return n
}

func validConvertRequest() camperedit.ConvertToDayVisitorRequest {
	needs := true
	return camperedit.ConvertToDayVisitorRequest{
		Days:          []string{"mon", "tue"},
		TshirtOption:  domain.TshirtOptionNone,
		ShirtSize:     domain.ShirtSizeNotApplicable,
		NeedsCatering: &needs,
	}
}

func validUpdateDayPassRequest() camperedit.UpdateDayPassCamperRequest {
	needs := false
	diet := "Gluten free"
	return camperedit.UpdateDayPassCamperRequest{
		TshirtOption:  domain.TshirtOptionTshirtOnly,
		ShirtSize:     "adult_l",
		NeedsCatering: &needs,
		Dietary:       &diet,
	}
}

type fakePrices struct {
	depositPence int
	err          error
}

func (f fakePrices) GetPrice(_ context.Context, code string) (domain.PriceRow, error) {
	if f.err != nil {
		return domain.PriceRow{}, f.err
	}
	if code != domain.PriceDeposit {
		return domain.PriceRow{}, errors.New("unexpected price code " + code)
	}
	return domain.PriceRow{AmountPence: f.depositPence, Currency: "GBP"}, nil
}

type mutablePrices struct{ depositPence int }

func (p *mutablePrices) GetPrice(_ context.Context, code string) (domain.PriceRow, error) {
	if code != domain.PriceDeposit {
		return domain.PriceRow{}, errors.New("unexpected price code " + code)
	}
	return domain.PriceRow{AmountPence: p.depositPence, Currency: "GBP"}, nil
}

func newFullWeekCamper() camperedit.AddCamperRequest {
	return camperedit.AddCamperRequest{
		Camper: domain.NewCamper{
			FirstName: "Ruth", LastName: "Adeyemi", Gender: "female", Age: 24,
			CellLeaderName: "Leader",
			Attendance: domain.Attendance{
				Type:                      domain.AttendanceFullWeek,
				ShirtSize:                 "adult_m",
				AccommodationFirstChoice:  "lodge",
				AccommodationSecondChoice: "static_caravan",
			},
		},
	}
}

func editFor(c domain.Camper) camperedit.EditCamperRequest {
	return camperedit.EditCamperRequest{
		FirstName:                  c.FirstName,
		LastName:                   c.LastName,
		Gender:                     c.Gender,
		Age:                        c.Age,
		CellLeaderName:             c.CellLeaderName,
		IsCellLeader:               c.IsCellLeader,
		ShirtSize:                  billing.StrVal(c.ShirtSize),
		DietaryRequirements:        billing.StrVal(c.DietaryRequirements),
		AccommodationFirstChoice:   billing.StrVal(c.AccommodationFirstChoice),
		AccommodationSecondChoice:  billing.StrVal(c.AccommodationSecondChoice),
		AllocatedAccommodationCode: billing.StrVal(c.AllocatedAccommodationCode),
		AllocatedUnitCode:          billing.StrVal(c.AllocatedUnitCode),
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

func TestRemoveCamper_success(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)
	repo := env.Repo

	groupID, _, camper2ID, camper3ID := billingtest.InsertThreeCamperGroup(ctx, t, pool, domain.BillingAllocated, nil)

	sum, err := env.Campers.RemoveCamper(ctx, groupID, camper2ID, "Admin", 1)
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
	if g.BillingStatus != domain.BillingAllocated {
		t.Fatalf("billing_status = %v, want allocated (remaining campers placed)", g.BillingStatus)
	}
}

func TestRemoveCamper_mainContactBlocked(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)
	repo := env.Repo

	groupID, mainID, _, _ := billingtest.InsertThreeCamperGroup(ctx, t, pool, domain.BillingAllocated, nil)

	_, err := env.Campers.RemoveCamper(ctx, groupID, mainID, "Admin", domain.SkipVersionCheck)
	var de *apperr.Error
	if !errors.As(err, &de) || de.Kind != apperr.KindInvalid {
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
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)

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

	_, err = env.Campers.RemoveCamper(ctx, groupID, camperID, "Admin", domain.SkipVersionCheck)
	var de *apperr.Error
	if !errors.As(err, &de) || de.Kind != apperr.KindInvalid {
		t.Fatalf("expected bad_request, got %v", err)
	}
}

func TestRemoveCamper_balancePaidBlocked(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)

	groupID, _, camper2ID, _ := billingtest.InsertThreeCamperGroup(ctx, t, pool, domain.BillingBalancePaid, nil)

	_, err := env.Campers.RemoveCamper(ctx, groupID, camper2ID, "Admin", domain.SkipVersionCheck)
	var de *apperr.Error
	if !errors.As(err, &de) || de.Kind != apperr.KindInvalid {
		t.Fatalf("expected bad_request, got %v", err)
	}
}

func TestRemoveCamper_invoicedVoidsAndReverts(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	inv := "in_remove_camper"
	var voided bool
	stub := &billingtest.StubStripe{
		VoidInvIdem: func(_ context.Context, id string) error {
			voided = true
			if id != inv {
				t.Fatalf("void invoice = %q, want %q", id, inv)
			}
			return nil
		},
	}
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)
	repo := env.Repo

	groupID, _, camper2ID, _ := billingtest.InsertThreeCamperGroup(ctx, t, pool, domain.BillingInvoiced, &inv)

	sum, err := env.Campers.RemoveCamper(ctx, groupID, camper2ID, "Admin", domain.SkipVersionCheck)
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
	if g.BalanceInvoiceRef != nil {
		t.Fatalf("stripe_invoice_id should be cleared, got %v", g.BalanceInvoiceRef)
	}
	if g.InvoiceDueAt != nil {
		t.Fatal("invoice_due_at should be cleared")
	}
	campers, _ := repo.CampersForGroup(ctx, groupID)
	if len(campers) != 2 {
		t.Fatalf("expected 2 campers, got %d", len(campers))
	}
}

func TestRemoveCamper_invoicedRevertsToNoneWhenFullWeekUnplaced(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	stub := &billingtest.StubStripe{VoidInvIdem: func(context.Context, string) error { return nil }}
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)
	repo := env.Repo

	inv := "in_remove_unplaced"
	groupID, _, camper2ID, camper3ID := billingtest.InsertThreeCamperGroup(ctx, t, pool, domain.BillingInvoiced, &inv)
	if _, err := pool.Exec(ctx,
		`UPDATE registrations SET allocated_accommodation_code = NULL WHERE id = $1`, camper3ID); err != nil {
		t.Fatalf("clear allocation: %v", err)
	}

	sum, err := env.Campers.RemoveCamper(ctx, groupID, camper2ID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("RemoveCamper: %v", err)
	}
	if !sum.InvoiceVoided {
		t.Fatal("expected invoice voided")
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil || g.BillingStatus != domain.BillingNone {
		t.Fatalf("billing_status = %v, want none", g)
	}
	if g.BalanceInvoiceRef != nil {
		t.Fatal("stripe_invoice_id should be cleared")
	}
}

func TestRemoveCamper_freesAllocation(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)

	groupID, _, camper2ID, _ := billingtest.InsertThreeCamperGroup(ctx, t, pool, domain.BillingAllocated, nil)
	before := countTierAllocations(ctx, pool, groupID, "lodge")
	if before != 2 {
		t.Fatalf("lodge count before = %d, want 2", before)
	}

	_, err := env.Campers.RemoveCamper(ctx, groupID, camper2ID, "Admin", domain.SkipVersionCheck)
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
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)

	groupID, _, camper2ID, _ := billingtest.InsertThreeCamperGroup(ctx, t, pool, domain.BillingAllocated, nil)

	_, err := env.Campers.RemoveCamper(ctx, groupID, camper2ID, "Admin", 99)
	var de *apperr.Error
	if !errors.As(err, &de) || de.Kind != apperr.KindConflict || de.Code != "stale_state" {
		t.Fatalf("expected stale_state, got %v", err)
	}
}

func TestRemoveCamper_sheetFailureNonFatal(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, billingtest.FailingSheets{}, cfg)
	repo := env.Repo

	groupID, _, camper2ID, _ := billingtest.InsertThreeCamperGroup(ctx, t, pool, domain.BillingAllocated, nil)

	_, err := env.Campers.RemoveCamper(ctx, groupID, camper2ID, "Admin", domain.SkipVersionCheck)
	if err != nil {
		t.Fatalf("RemoveCamper should succeed despite sheet error: %v", err)
	}
	campers, _ := repo.CampersForGroup(ctx, groupID)
	if len(campers) != 2 {
		t.Fatalf("expected 2 campers, got %d", len(campers))
	}
}

func TestUpdateCamperCoach_turnOffOneThenInvoice(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	var captured []billing.InvoiceLine
	stub := &billingtest.StubStripe{
		EnsureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "cus_two", nil
		},
		CreateInv: func(_ context.Context, _, _ string, lines []billing.InvoiceLine, _ int64, invoiceType string) (billing.InvoiceResult, error) {
			captured = lines
			if invoiceType != "coach" {
				t.Fatalf("invoice type = %q, want coach", invoiceType)
			}
			return billing.InvoiceResult{ID: "in_coach_one", ProviderEmailed: true}, nil
		},
	}
	cfg := billing.Config{CoachPriceRef: "price_coach"}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)
	repo := env.Repo

	groupID, camper1ID, _ := billingtest.InsertTwoCoachGroup(ctx, t, pool, domain.BillingBalancePaid)
	if err := env.Invoicing.WaiveCoachFee(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("WaiveCoachFee: %v", err)
	}

	sum, err := env.Campers.UpdateCamperCoach(ctx, groupID, camper1ID, "Admin", domain.SkipVersionCheck, camperedit.UpdateCamperCoachRequest{NeedsCoach: false})
	if err != nil {
		t.Fatalf("UpdateCamperCoach: %v", err)
	}
	if sum.NoOp || sum.CamperName != "Aliyah Oliveros" {
		t.Fatalf("summary = %+v", sum)
	}

	campers, _ := repo.CampersForGroup(ctx, groupID)
	if billing.CoachEligibleCount(campers) != 1 {
		t.Fatalf("eligible count = %d, want 1", billing.CoachEligibleCount(campers))
	}

	if err := env.Invoicing.UnwaiveCoachFee(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("UnwaiveCoachFee: %v", err)
	}
	if err := env.Invoicing.SendCoachInvoice(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("SendCoachInvoice: %v", err)
	}
	if len(captured) != 1 || captured[0].Quantity != 1 {
		t.Fatalf("invoice lines = %+v, want qty 1", captured)
	}
}

func TestUpdateCamperCoach_guards(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{CoachPriceRef: "price_coach"}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)

	groupID, camper1ID, _ := billingtest.InsertTwoCoachGroup(ctx, t, pool, domain.BillingBalancePaid)

	// Coach fee already paid via separate invoice.
	if _, err := pool.Exec(ctx, `
		UPDATE registration_groups SET coach_fee_paid_at = NOW() WHERE id = $1`, groupID); err != nil {
		t.Fatalf("set coach paid: %v", err)
	}
	_, err := env.Campers.UpdateCamperCoach(ctx, groupID, camper1ID, "Admin", domain.SkipVersionCheck, camperedit.UpdateCamperCoachRequest{NeedsCoach: false})
	var de *apperr.Error
	if !errors.As(err, &de) || de.Kind != apperr.KindInvalid {
		t.Fatalf("expected bad_request when coach paid, got %v", err)
	}

	// Coach folded into balance invoice.
	groupID2, camper2a, _ := billingtest.InsertTwoCoachGroup(ctx, t, pool, domain.BillingBalancePaid)
	if _, err := pool.Exec(ctx, `
		UPDATE registration_groups SET coach_included_in_balance = true WHERE id = $1`, groupID2); err != nil {
		t.Fatalf("set coach in balance: %v", err)
	}
	_, err = env.Campers.UpdateCamperCoach(ctx, groupID2, camper2a, "Admin", domain.SkipVersionCheck, camperedit.UpdateCamperCoachRequest{NeedsCoach: false})
	if !errors.As(err, &de) || de.Kind != apperr.KindInvalid {
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
	_, err = env.Campers.UpdateCamperCoach(ctx, dayGroup, dayCamper, "Admin", domain.SkipVersionCheck, camperedit.UpdateCamperCoachRequest{NeedsCoach: true})
	if !errors.As(err, &de) || de.Kind != apperr.KindInvalid {
		t.Fatalf("expected bad_request for day pass, got %v", err)
	}
}

func TestUpdateCamperCoach_voidOpenInvoice(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	var voided []string
	stub := &billingtest.StubStripe{
		EnsureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "cus_two", nil
		},
		CreateInv: func(_ context.Context, _, _ string, _ []billing.InvoiceLine, _ int64, _ string) (billing.InvoiceResult, error) {
			return billing.InvoiceResult{ID: "in_coach_open", ProviderEmailed: true}, nil
		},
		VoidInvIdem: func(_ context.Context, id string) error {
			voided = append(voided, id)
			return nil
		},
	}
	cfg := billing.Config{CoachPriceRef: "price_coach"}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)
	repo := env.Repo

	groupID, camper1ID, _ := billingtest.InsertTwoCoachGroup(ctx, t, pool, domain.BillingBalancePaid)
	if err := env.Invoicing.SendCoachInvoice(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("SendCoachInvoice: %v", err)
	}

	sum, err := env.Campers.UpdateCamperCoach(ctx, groupID, camper1ID, "Admin", domain.SkipVersionCheck, camperedit.UpdateCamperCoachRequest{NeedsCoach: false})
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
	if g == nil || g.CoachInvoiceRef != nil {
		t.Fatalf("coach invoice columns not cleared: %+v", g)
	}
}

func TestUpdateCamperCoach_noOp(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)
	repo := env.Repo

	groupID, camper1ID, _ := billingtest.InsertTwoCoachGroup(ctx, t, pool, domain.BillingBalancePaid)
	gBefore, _ := repo.FindGroupByID(ctx, groupID)

	sum, err := env.Campers.UpdateCamperCoach(ctx, groupID, camper1ID, "Admin", domain.SkipVersionCheck, camperedit.UpdateCamperCoachRequest{NeedsCoach: true})
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
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)

	groupID, camper1ID, _ := billingtest.InsertTwoCoachGroup(ctx, t, pool, domain.BillingBalancePaid)
	_, err := env.Campers.UpdateCamperCoach(ctx, groupID, camper1ID, "Admin", 99, camperedit.UpdateCamperCoachRequest{NeedsCoach: false})
	var de *apperr.Error
	if !errors.As(err, &de) || de.Kind != apperr.KindConflict || de.Code != "stale_state" {
		t.Fatalf("expected stale_state conflict, got %v", err)
	}
}

func TestUpdateCamperCoach_waivedRemainsWhenLastPassengerRemoved(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{CoachPriceRef: "price_coach"}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)
	repo := env.Repo

	groupID, camper1ID, camper2ID := billingtest.InsertTwoCoachGroup(ctx, t, pool, domain.BillingBalancePaid)
	if err := env.Invoicing.WaiveCoachFee(ctx, groupID, "Admin", domain.SkipVersionCheck); err != nil {
		t.Fatalf("WaiveCoachFee: %v", err)
	}

	if _, err := env.Campers.UpdateCamperCoach(ctx, groupID, camper1ID, "Admin", domain.SkipVersionCheck, camperedit.UpdateCamperCoachRequest{NeedsCoach: false}); err != nil {
		t.Fatalf("UpdateCamperCoach camper1: %v", err)
	}
	if _, err := env.Campers.UpdateCamperCoach(ctx, groupID, camper2ID, "Admin", domain.SkipVersionCheck, camperedit.UpdateCamperCoachRequest{NeedsCoach: false}); err != nil {
		t.Fatalf("UpdateCamperCoach camper2: %v", err)
	}

	campers, _ := repo.CampersForGroup(ctx, groupID)
	if billing.CoachEligibleCount(campers) != 0 {
		t.Fatalf("eligible count = %d, want 0", billing.CoachEligibleCount(campers))
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil || g.CoachFeeWaivedAt == nil {
		t.Fatalf("coach_fee_waived_at should remain set: %+v", g)
	}

	if _, err := env.Campers.UpdateCamperCoach(ctx, groupID, camper2ID, "Admin", domain.SkipVersionCheck, camperedit.UpdateCamperCoachRequest{NeedsCoach: true}); err != nil {
		t.Fatalf("UpdateCamperCoach restore camper2: %v", err)
	}
	campers, _ = repo.CampersForGroup(ctx, groupID)
	if billing.CoachEligibleCount(campers) != 1 {
		t.Fatalf("eligible count after restore = %d, want 1", billing.CoachEligibleCount(campers))
	}
	g, _ = repo.FindGroupByID(ctx, groupID)
	if g == nil || g.CoachFeeWaivedAt == nil {
		t.Fatalf("coach_fee_waived_at should still be set after partial restore: %+v", g)
	}
}

func TestConvertCamperToDayVisitor_allocated(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	stub := &billingtest.StubStripe{
		EnsureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "cus_convert", nil
		},
	}
	cfg := billing.Config{DepositPricePence: 5000}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)
	repo := env.Repo

	groupID, _, camper2ID, _ := billingtest.InsertThreeCamperGroup(ctx, t, pool, domain.BillingAllocated, nil)

	sum, err := env.Campers.ConvertCamperToDayVisitor(ctx, groupID, camper2ID, "Admin", 1, validConvertRequest())
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
	if len(stub.CreditCalls) != 1 {
		t.Fatalf("credit calls = %d, want 1", len(stub.CreditCalls))
	}
	if stub.CreditCalls[0].AmountPence != 5000 {
		t.Fatalf("credit amount = %d", stub.CreditCalls[0].AmountPence)
	}
	wantKey := "deposit-credit-" + camper2ID
	if stub.CreditCalls[0].IdempotencyKey != wantKey {
		t.Fatalf("idempotency key = %q, want %q", stub.CreditCalls[0].IdempotencyKey, wantKey)
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
	if g.BillingStatus != domain.BillingAllocated {
		t.Fatalf("billing_status = %v, want allocated (remaining campers placed)", g.BillingStatus)
	}
	if g.Version != 2 {
		t.Fatalf("version = %d, want 2", g.Version)
	}
}

func TestConvertCamperToDayVisitor_multiConvertTwoCredits(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	stub := &billingtest.StubStripe{
		EnsureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "cus_multi", nil
		},
	}
	cfg := billing.Config{DepositPricePence: 5000}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)

	groupID, _, camper2ID, camper3ID := billingtest.InsertThreeCamperGroup(ctx, t, pool, domain.BillingAllocated, nil)

	if _, err := env.Campers.ConvertCamperToDayVisitor(ctx, groupID, camper2ID, "Admin", 1, validConvertRequest()); err != nil {
		t.Fatalf("first convert: %v", err)
	}
	if _, err := env.Campers.ConvertCamperToDayVisitor(ctx, groupID, camper3ID, "Admin", 2, validConvertRequest()); err != nil {
		t.Fatalf("second convert: %v", err)
	}
	if len(stub.CreditCalls) != 2 {
		t.Fatalf("credit calls = %d, want 2", len(stub.CreditCalls))
	}
	keys := map[string]bool{
		stub.CreditCalls[0].IdempotencyKey: true,
		stub.CreditCalls[1].IdempotencyKey: true,
	}
	if !keys["deposit-credit-"+camper2ID] || !keys["deposit-credit-"+camper3ID] {
		t.Fatalf("unexpected idempotency keys: %+v", stub.CreditCalls)
	}
}

func TestConvertCamperToDayVisitor_invoicedVoidsAndReverts(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	voided := false
	stub := &billingtest.StubStripe{
		VoidInvIdem: func(_ context.Context, id string) error {
			if id != "in_open" {
				t.Fatalf("void id = %q", id)
			}
			voided = true
			return nil
		},
		EnsureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "cus_void", nil
		},
	}
	cfg := billing.Config{DepositPricePence: 5000}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)
	repo := env.Repo

	inv := "in_open"
	groupID, _, camper2ID, _ := billingtest.InsertThreeCamperGroup(ctx, t, pool, domain.BillingInvoiced, &inv)

	sum, err := env.Campers.ConvertCamperToDayVisitor(ctx, groupID, camper2ID, "Admin", domain.SkipVersionCheck, validConvertRequest())
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
	if g.BalanceInvoiceRef != nil {
		t.Fatal("stripe_invoice_id should be cleared")
	}
}

func TestConvertCamperToDayVisitor_invoicedRevertsToNoneWhenFullWeekUnplaced(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	stub := &billingtest.StubStripe{
		VoidInvIdem: func(context.Context, string) error { return nil },
		EnsureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "cus_unplaced", nil
		},
	}
	cfg := billing.Config{DepositPricePence: 5000}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)
	repo := env.Repo

	inv := "in_convert_unplaced"
	groupID, _, camper2ID, camper3ID := billingtest.InsertThreeCamperGroup(ctx, t, pool, domain.BillingInvoiced, &inv)
	if _, err := pool.Exec(ctx,
		`UPDATE registrations SET allocated_accommodation_code = NULL WHERE id = $1`, camper3ID); err != nil {
		t.Fatalf("clear allocation: %v", err)
	}

	sum, err := env.Campers.ConvertCamperToDayVisitor(ctx, groupID, camper2ID, "Admin", domain.SkipVersionCheck, validConvertRequest())
	if err != nil {
		t.Fatalf("ConvertCamperToDayVisitor: %v", err)
	}
	if !sum.InvoiceVoided {
		t.Fatal("expected invoice voided")
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g == nil || g.BillingStatus != domain.BillingNone {
		t.Fatalf("billing_status = %v, want none", g)
	}
}

func TestConvertCamperToDayVisitor_invoicedRevertsToNoneWhenDayPassOnly(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	stub := &billingtest.StubStripe{
		VoidInvIdem: func(context.Context, string) error { return nil },
		EnsureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "cus_none", nil
		},
	}
	cfg := billing.Config{DepositPricePence: 5000}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)
	repo := env.Repo

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

	_, err = env.Campers.ConvertCamperToDayVisitor(ctx, groupID, camperID, "Admin", 1, validConvertRequest())
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
	stub := &billingtest.StubStripe{}
	cfg := billing.Config{DepositPricePence: 5000}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)
	repo := env.Repo

	t.Run("balance_paid", func(t *testing.T) {
		groupID, _, camper2ID, _ := billingtest.InsertThreeCamperGroup(ctx, t, pool, domain.BillingBalancePaid, nil)
		_, err := env.Campers.ConvertCamperToDayVisitor(ctx, groupID, camper2ID, "Admin", domain.SkipVersionCheck, validConvertRequest())
		var de *apperr.Error
		if !errors.As(err, &de) || de.Kind != apperr.KindInvalid {
			t.Fatalf("expected bad_request, got %v", err)
		}
	})

	t.Run("day_pass_camper", func(t *testing.T) {
		groupID := billingtest.InsertCoachGroup(ctx, t, pool, domain.BillingAllocated, "lodge", false)
		campers, _ := repo.CampersForGroup(ctx, groupID)
		_, err := pool.Exec(ctx, `UPDATE registrations SET attendance_type = 'day_pass', day_pass_days = '{mon}' WHERE id = $1`, campers[0].ID)
		if err != nil {
			t.Fatalf("update camper: %v", err)
		}
		_, err = env.Campers.ConvertCamperToDayVisitor(ctx, groupID, campers[0].ID, "Admin", domain.SkipVersionCheck, validConvertRequest())
		var de *apperr.Error
		if !errors.As(err, &de) || de.Kind != apperr.KindInvalid {
			t.Fatalf("expected bad_request, got %v", err)
		}
	})

	t.Run("empty_days", func(t *testing.T) {
		groupID, _, camper2ID, _ := billingtest.InsertThreeCamperGroup(ctx, t, pool, domain.BillingAllocated, nil)
		req := validConvertRequest()
		req.Days = nil
		_, err := env.Campers.ConvertCamperToDayVisitor(ctx, groupID, camper2ID, "Admin", domain.SkipVersionCheck, req)
		var de *apperr.Error
		if !errors.As(err, &de) || de.Kind != apperr.KindValidation {
			t.Fatalf("expected validation_failed, got %v", err)
		}
	})

	t.Run("is_free", func(t *testing.T) {
		groupID := billingtest.InsertCoachGroup(ctx, t, pool, domain.BillingAllocated, "lodge", true)
		campers, _ := repo.CampersForGroup(ctx, groupID)
		_, err := env.Campers.ConvertCamperToDayVisitor(ctx, groupID, campers[0].ID, "Admin", domain.SkipVersionCheck, validConvertRequest())
		var de *apperr.Error
		if !errors.As(err, &de) || de.Kind != apperr.KindInvalid {
			t.Fatalf("expected bad_request, got %v", err)
		}
		if len(stub.CreditCalls) != 0 {
			t.Fatal("free group should not trigger credit")
		}
	})
}

func TestConvertCamperToDayVisitor_under4NoCredit(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	stub := &billingtest.StubStripe{}
	cfg := billing.Config{DepositPricePence: 5000}
	env := billingtest.NewEnv(t, pool, stub, nil, nil, cfg)
	repo := env.Repo

	groupID, _, _, _ := billingtest.InsertThreeCamperGroup(ctx, t, pool, domain.BillingAllocated, nil)
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

	sum, err := env.Campers.ConvertCamperToDayVisitor(ctx, groupID, youngID, "Admin", domain.SkipVersionCheck, validConvertRequest())
	if err != nil {
		t.Fatalf("ConvertCamperToDayVisitor: %v", err)
	}
	if sum.DepositCreditPence != 0 {
		t.Fatalf("credit pence = %d, want 0", sum.DepositCreditPence)
	}
	if len(stub.CreditCalls) != 0 {
		t.Fatal("under-4 should not trigger credit")
	}
	campers, _ := repo.CampersForGroup(ctx, groupID)
	for _, c := range campers {
		if c.ID == youngID && c.DepositCreditPence != 0 {
			t.Fatalf("deposit_credit_pence = %d", c.DepositCreditPence)
		}
	}
}

func TestUpdateDayPassCamper_success(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	rec := &billingtest.RecordingSheets{}
	stripe := &billingtest.StubStripe{
		VoidInvIdem: func(context.Context, string) error {
			t.Fatal("unexpected VoidInvoiceIdempotent")
			return nil
		},
	}
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, stripe, nil, rec, cfg)
	repo := env.Repo

	groupID, camperID := billingtest.InsertDayPassGroup(ctx, t, pool, domain.BillingNone)

	sum, err := env.Campers.UpdateDayPassCamper(ctx, groupID, camperID, "Admin", 1, validUpdateDayPassRequest())
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
	if rec.AppendCalls != 1 {
		t.Fatalf("sheet append calls = %d, want 1", rec.AppendCalls)
	}
	if rec.LastGroupID != groupID {
		t.Fatalf("sheet group id = %q", rec.LastGroupID)
	}
	if len(stripe.CreditCalls) != 0 {
		t.Fatalf("credit calls = %d, want 0", len(stripe.CreditCalls))
	}
}

func TestUpdateDayPassCamper_allowedWhenBalancePaid(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)

	groupID, camperID := billingtest.InsertDayPassGroup(ctx, t, pool, domain.BillingBalancePaid)

	_, err := env.Campers.UpdateDayPassCamper(ctx, groupID, camperID, "Admin", domain.SkipVersionCheck, validUpdateDayPassRequest())
	if err != nil {
		t.Fatalf("UpdateDayPassCamper on balance_paid: %v", err)
	}
}

func TestUpdateDayPassCamper_guards(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)
	repo := env.Repo

	t.Run("full_week", func(t *testing.T) {
		groupID, _, camper2ID, _ := billingtest.InsertThreeCamperGroup(ctx, t, pool, domain.BillingAllocated, nil)
		_, err := env.Campers.UpdateDayPassCamper(ctx, groupID, camper2ID, "Admin", domain.SkipVersionCheck, validUpdateDayPassRequest())
		var de *apperr.Error
		if !errors.As(err, &de) || de.Kind != apperr.KindInvalid {
			t.Fatalf("expected bad_request, got %v", err)
		}
	})

	t.Run("validation_empty_shirt", func(t *testing.T) {
		groupID, camperID := billingtest.InsertDayPassGroup(ctx, t, pool, domain.BillingNone)
		req := validUpdateDayPassRequest()
		req.TshirtOption = domain.TshirtOptionTeamActivities
		req.ShirtSize = ""
		_, err := env.Campers.UpdateDayPassCamper(ctx, groupID, camperID, "Admin", domain.SkipVersionCheck, req)
		var de *apperr.Error
		if !errors.As(err, &de) || de.Kind != apperr.KindValidation {
			t.Fatalf("expected validation_failed, got %v", err)
		}
	})

	t.Run("none_normalizes_shirt", func(t *testing.T) {
		groupID, camperID := billingtest.InsertDayPassGroup(ctx, t, pool, domain.BillingNone)
		needs := false
		req := camperedit.UpdateDayPassCamperRequest{
			TshirtOption:  domain.TshirtOptionNone,
			ShirtSize:     domain.ShirtSizeNotApplicable,
			NeedsCatering: &needs,
		}
		_, err := env.Campers.UpdateDayPassCamper(ctx, groupID, camperID, "Admin", domain.SkipVersionCheck, req)
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
		groupID, camperID := billingtest.InsertDayPassGroup(ctx, t, pool, domain.BillingNone)
		needs := true
		empty := ""
		req := camperedit.UpdateDayPassCamperRequest{
			TshirtOption:  domain.TshirtOptionTeamActivities,
			ShirtSize:     "adult_m",
			NeedsCatering: &needs,
			Dietary:       &empty,
		}
		_, err := env.Campers.UpdateDayPassCamper(ctx, groupID, camperID, "Admin", domain.SkipVersionCheck, req)
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

func TestEditCamper_swapLeavesOpenInvoiceAlone(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	mailer := &billingtest.RecordingMailer{}

	voids := 0
	stub := &billingtest.StubStripe{VoidInvIdem: func(context.Context, string) error {
		voids++
		return nil
	}}
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, stub, mailer, nil, cfg)
	repo := env.Repo

	groupID, _, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingInvoiced)
	if _, err := pool.Exec(ctx,
		`UPDATE registration_groups SET stripe_invoice_id = 'in_live' WHERE id = $1`,
		groupID); err != nil {
		t.Fatalf("seed invoice: %v", err)
	}

	campers, _ := repo.CampersForGroup(ctx, groupID)
	req := editFor(camperByID(t, campers, otherID))
	req.FirstName = "Deborah"
	req.LastName = "Mensah"

	sum, err := env.Campers.EditCamper(ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, req)
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
	if g.BalanceInvoiceRef == nil || *g.BalanceInvoiceRef != "in_live" {
		t.Fatal("the live invoice should still be attached")
	}

	after, _ := repo.CampersForGroup(ctx, groupID)
	got := camperByID(t, after, otherID)
	if got.FirstName != "Deborah" || got.LastName != "Mensah" {
		t.Fatalf("camper not renamed: %s %s", got.FirstName, got.LastName)
	}
	if billing.StrVal(got.AllocatedAccommodationCode) != "lodge" {
		t.Fatal("the seat's accommodation should survive a swap")
	}

	if len(mailer.BookingUpdated) != 1 {
		t.Fatalf("booking-updated emails = %d, want 1", len(mailer.BookingUpdated))
	}
	if mailer.BookingUpdated[0].InvoiceCancelled {
		t.Fatal("email must not claim an invoice was cancelled when none was")
	}
	if mailer.BookingUpdated[0].AddedName != "" {
		t.Fatal("an edit is not an addition")
	}
}

func TestEditCamper_accommodationChangeVoidsInvoice(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	mailer := &billingtest.RecordingMailer{}
	var voided []string
	stub := &billingtest.StubStripe{VoidInvIdem: func(_ context.Context, id string) error {
		voided = append(voided, id)
		return nil
	}}
	env := billingtest.NewEnv(t, pool, stub, mailer, nil, cfg)
	repo := env.Repo
	if err := repo.UpdateAccommodationPriceRef(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("set lodge price: %v", err)
	}
	if err := repo.UpdateAccommodationPriceRef(ctx, "tent", "price_tent"); err != nil {
		t.Fatalf("set tent price: %v", err)
	}

	groupID, _, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingInvoiced)
	if _, err := pool.Exec(ctx,
		`UPDATE registration_groups SET stripe_invoice_id = 'in_live' WHERE id = $1`,
		groupID); err != nil {
		t.Fatalf("seed invoice: %v", err)
	}

	campers, _ := repo.CampersForGroup(ctx, groupID)
	req := editFor(camperByID(t, campers, otherID))
	req.AllocatedAccommodationCode = "tent"

	sum, err := env.Campers.EditCamper(ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, req)
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
	if g.BalanceInvoiceRef != nil {
		t.Fatalf("stripe_invoice_id = %v, want cleared", *g.BalanceInvoiceRef)
	}
	if len(mailer.BookingUpdated) != 1 || !mailer.BookingUpdated[0].InvoiceCancelled {
		t.Fatal("family must be told their invoice was cancelled")
	}
}

func TestEditCamper_clearingAllocationSendsGroupBackToNeedsAccommodation(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, &billingtest.RecordingMailer{}, nil, cfg)
	repo := env.Repo
	if err := repo.UpdateAccommodationPriceRef(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("set lodge price: %v", err)
	}

	groupID, _, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingInvoiced)
	if _, err := pool.Exec(ctx,
		`UPDATE registration_groups SET stripe_invoice_id = 'in_live' WHERE id = $1`,
		groupID); err != nil {
		t.Fatalf("seed invoice: %v", err)
	}

	campers, _ := repo.CampersForGroup(ctx, groupID)
	req := editFor(camperByID(t, campers, otherID))
	req.AllocatedAccommodationCode = ""

	if _, err := env.Campers.EditCamper(
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

func TestCamperActions_recordDistinctAuditActions(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnvWithPrices(t, pool, &billingtest.StubStripe{}, &billingtest.RecordingMailer{}, nil, cfg, fakePrices{depositPence: 5000})
	repo := env.Repo

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
		groupID, _, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingAllocated)
		campers, _ := repo.CampersForGroup(ctx, groupID)
		req := editFor(camperByID(t, campers, otherID))
		req.FirstName = "Deborah"
		if _, err := env.Campers.EditCamper(
			ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, req); err != nil {
			t.Fatalf("EditCamper: %v", err)
		}
		if got := lastAction(groupID); got != "camper_edited" {
			t.Fatalf("last_action = %q, want camper_edited", got)
		}
	})

	t.Run("add", func(t *testing.T) {
		groupID, _, _ := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingAllocated)
		if _, err := env.Campers.AddCamper(
			ctx, groupID, "Diane", domain.SkipVersionCheck, newFullWeekCamper()); err != nil {
			t.Fatalf("AddCamper: %v", err)
		}
		if got := lastAction(groupID); got != "camper_added" {
			t.Fatalf("last_action = %q, want camper_added", got)
		}
	})

	t.Run("make_main_contact", func(t *testing.T) {
		groupID, _, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingAllocated)
		if _, err := env.Campers.MakeMainContact(
			ctx, groupID, otherID, "Diane", domain.SkipVersionCheck); err != nil {
			t.Fatalf("MakeMainContact: %v", err)
		}
		if got := lastAction(groupID); got != "main_contact_moved" {
			t.Fatalf("last_action = %q, want main_contact_moved", got)
		}
	})

	t.Run("waive_deposit", func(t *testing.T) {
		groupID, _, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingAllocated)
		if _, err := pool.Exec(ctx,
			`UPDATE registrations SET deposit_owed_pence = 5000 WHERE id = $1`, otherID); err != nil {
			t.Fatalf("seed deposit owed: %v", err)
		}
		if _, err := env.Campers.WaiveCamperDeposit(
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
	cfg := billing.Config{}
	env := billingtest.NewEnvWithPrices(t, pool, &billingtest.StubStripe{}, &billingtest.RecordingMailer{}, nil, cfg, fakePrices{depositPence: 5000})
	repo := env.Repo

	groupID, _, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingAllocated)
	if _, err := pool.Exec(ctx,
		`UPDATE registrations SET deposit_owed_pence = 5000 WHERE id = $1`, otherID); err != nil {
		t.Fatalf("seed deposit owed: %v", err)
	}
	campers, _ := repo.CampersForGroup(ctx, groupID)

	const staleVersion = 99
	cases := map[string]func() error{
		"edit": func() error {
			_, err := env.Campers.EditCamper(
				ctx, groupID, otherID, "Diane", staleVersion,
				editFor(camperByID(t, campers, otherID)))
			return err
		},
		"add": func() error {
			_, err := env.Campers.AddCamper(ctx, groupID, "Diane", staleVersion, newFullWeekCamper())
			return err
		},
		"make_main_contact": func() error {
			_, err := env.Campers.MakeMainContact(ctx, groupID, otherID, "Diane", staleVersion)
			return err
		},
		"waive_deposit": func() error {
			_, err := env.Campers.WaiveCamperDeposit(ctx, groupID, otherID, "Diane", staleVersion)
			return err
		},
	}

	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			err := call()
			var de *apperr.Error
			if !errors.As(err, &de) || de.Kind != apperr.KindConflict || de.Code != "stale_state" {
				t.Fatalf("error = %v, want stale_state", err)
			}
		})
	}
}

func TestEditCamper_voidFailureLeavesCamperUnchanged(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	stub := &billingtest.StubStripe{VoidInvIdem: func(context.Context, string) error {
		return errors.New("stripe is down")
	}}
	env := billingtest.NewEnv(t, pool, stub, &billingtest.RecordingMailer{}, nil, cfg)
	repo := env.Repo
	if err := repo.UpdateAccommodationPriceRef(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("set lodge price: %v", err)
	}
	if err := repo.UpdateAccommodationPriceRef(ctx, "tent", "price_tent"); err != nil {
		t.Fatalf("set tent price: %v", err)
	}

	groupID, _, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingInvoiced)
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

	if _, err := env.Campers.EditCamper(
		ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, req); err == nil {
		t.Fatal("expected the edit to be refused when the void fails")
	}

	after, _ := repo.CampersForGroup(ctx, groupID)
	got := camperByID(t, after, otherID)
	if got.FirstName != before.FirstName {
		t.Fatalf("first_name = %q, want it untouched at %q", got.FirstName, before.FirstName)
	}
	if billing.StrVal(got.AllocatedAccommodationCode) != billing.StrVal(before.AllocatedAccommodationCode) {
		t.Fatalf("allocation = %q, want it untouched",
			billing.StrVal(got.AllocatedAccommodationCode))
	}
	g, _ := repo.FindGroupByID(ctx, groupID)
	if g.BillingStatus != domain.BillingInvoiced {
		t.Fatalf("billing_status = %q, want the group left invoiced", g.BillingStatus)
	}
}

func TestEditCamper_allowedOnChurchSponsoredGroup(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, &billingtest.RecordingMailer{}, nil, cfg)
	repo := env.Repo

	groupID, _, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingAllocated)
	if _, err := pool.Exec(ctx,
		`UPDATE registration_groups SET is_free = true WHERE id = $1`, groupID); err != nil {
		t.Fatalf("mark sponsored: %v", err)
	}

	campers, _ := repo.CampersForGroup(ctx, groupID)
	req := editFor(camperByID(t, campers, otherID))
	req.FirstName = "Deborah"
	req.AllocatedAccommodationCode = "tent"

	sum, err := env.Campers.EditCamper(ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, req)
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
	cfg := billing.Config{}
	voids := 0
	stub := &billingtest.StubStripe{VoidInvIdem: func(context.Context, string) error {
		voids++
		return nil
	}}
	env := billingtest.NewEnv(t, pool, stub, &billingtest.RecordingMailer{}, nil, cfg)
	repo := env.Repo
	if err := repo.UpdateAccommodationPriceRef(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("set lodge price: %v", err)
	}
	if err := repo.UpdateAccommodationPriceRef(ctx, "tent", "price_tent"); err != nil {
		t.Fatalf("set tent price: %v", err)
	}

	groupID, _, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingAllocated)
	campers, _ := repo.CampersForGroup(ctx, groupID)
	req := editFor(camperByID(t, campers, otherID))
	req.AllocatedAccommodationCode = "tent"

	sum, err := env.Campers.EditCamper(ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, req)
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
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, &billingtest.RecordingMailer{}, nil, cfg)
	repo := env.Repo
	if err := repo.UpdateAccommodationPriceRef(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("set lodge price: %v", err)
	}
	if err := repo.UpdateAccommodationPriceRef(ctx, "tent", "price_tent"); err != nil {
		t.Fatalf("set tent price: %v", err)
	}

	groupID, _, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingBalancePaid)
	campers, _ := repo.CampersForGroup(ctx, groupID)

	reprice := editFor(camperByID(t, campers, otherID))
	reprice.AllocatedAccommodationCode = "tent"
	if _, err := env.Campers.EditCamper(
		ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, reprice); err == nil {
		t.Fatal("expected a repricing edit to be refused once the balance is paid")
	} else {
		var de *apperr.Error
		if !errors.As(err, &de) || de.Kind != apperr.KindInvalid {
			t.Fatalf("error = %v, want bad_request", err)
		}
	}

	// A change that costs nothing is still allowed, which is the point of the rule.
	rename := editFor(camperByID(t, campers, otherID))
	rename.FirstName = "Deborah"
	if _, err := env.Campers.EditCamper(
		ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, rename); err != nil {
		t.Fatalf("a free-of-charge edit on a paid group should be allowed: %v", err)
	}
}

func TestEditCamper_ageingPastCoachThresholdReprices(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	voids := 0
	stub := &billingtest.StubStripe{VoidInvIdem: func(context.Context, string) error {
		voids++
		return nil
	}}
	env := billingtest.NewEnv(t, pool, stub, &billingtest.RecordingMailer{}, nil, cfg)
	repo := env.Repo
	if err := repo.UpdateAccommodationPriceRef(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("set lodge price: %v", err)
	}

	groupID, _, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingInvoiced)
	if _, err := pool.Exec(ctx, `
		UPDATE registration_groups SET stripe_invoice_id = 'in_live' WHERE id = $1`,
		groupID); err != nil {
		t.Fatalf("seed invoice: %v", err)
	}
	// Put them on the coach, below the age where a seat is charged.
	if _, err := pool.Exec(ctx, `
		UPDATE registrations SET needs_coach = true, age = $2 WHERE id = $1`,
		otherID, domain.MinDepositAge-1); err != nil {
		t.Fatalf("seed coach passenger: %v", err)
	}

	campers, _ := repo.CampersForGroup(ctx, groupID)
	req := editFor(camperByID(t, campers, otherID))
	req.Age = domain.MinDepositAge

	sum, err := env.Campers.EditCamper(ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, req)
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
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, &billingtest.RecordingMailer{}, nil, cfg)
	repo := env.Repo

	groupID, _, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingAllocated)
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
	req.Age = domain.MaxChildAccommodationAge + 1

	_, err := env.Campers.EditCamper(ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, req)
	if err == nil {
		t.Fatal("expected an age past the child limit to be refused while in child accommodation")
	}
	var de *apperr.Error
	if !errors.As(err, &de) || de.Kind != apperr.KindValidation {
		t.Fatalf("error = %v, want validation_failed", err)
	}
}

func TestEditCamper_fullReplacementCatchesInvalidCombination(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, &billingtest.RecordingMailer{}, nil, cfg)
	repo := env.Repo

	groupID, _, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingAllocated)
	campers, _ := repo.CampersForGroup(ctx, groupID)

	req := editFor(camperByID(t, campers, otherID))
	req.ShirtSize = ""

	_, err := env.Campers.EditCamper(ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, req)
	if err == nil {
		t.Fatal("a full-week camper with no shirt size should be rejected")
	}
	var de *apperr.Error
	if !errors.As(err, &de) || de.Kind != apperr.KindValidation {
		t.Fatalf("error = %v, want validation_failed", err)
	}
}

func TestAddCamper_recordsDepositAtLivePrice(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	mailer := &billingtest.RecordingMailer{}
	cfg := billing.Config{DepositPricePence: 5000}
	env := billingtest.NewEnvWithPrices(t, pool, &billingtest.StubStripe{}, mailer, nil, cfg, fakePrices{depositPence: 7500})
	repo := env.Repo

	groupID, _, _ := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingAllocated)

	sum, err := env.Campers.AddCamper(ctx, groupID, "Diane", domain.SkipVersionCheck, camperedit.AddCamperRequest{
		Camper: domain.NewCamper{
			FirstName: "Ruth", LastName: "Adeyemi", Gender: "female", Age: 24,
			CellLeaderName: "Leader",
			Attendance: domain.Attendance{
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

	if len(mailer.BookingUpdated) != 1 {
		t.Fatalf("booking-updated emails = %d, want 1", len(mailer.BookingUpdated))
	}
	if mailer.BookingUpdated[0].AddedName != "Ruth Adeyemi" {
		t.Fatalf("added name = %q", mailer.BookingUpdated[0].AddedName)
	}
}

func TestAddCamper_childUnderDepositAgeOwesNothing(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnvWithPrices(t, pool, &billingtest.StubStripe{}, &billingtest.RecordingMailer{}, nil, cfg, fakePrices{depositPence: 5000})

	groupID, _, _ := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingAllocated)

	sum, err := env.Campers.AddCamper(ctx, groupID, "Diane", domain.SkipVersionCheck, camperedit.AddCamperRequest{
		Camper: domain.NewCamper{
			FirstName: "Baby", LastName: "Fundi", Gender: "male",
			Age:            domain.MinDepositAge - 1,
			CellLeaderName: "Leader",
			Attendance: domain.Attendance{
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
	cfg := billing.Config{}
	env := billingtest.NewEnvWithPrices(t, pool, &billingtest.StubStripe{}, &billingtest.RecordingMailer{}, nil, cfg, fakePrices{depositPence: 5000})

	newcomer := camperedit.AddCamperRequest{
		Camper: domain.NewCamper{
			FirstName: "Ruth", LastName: "Adeyemi", Gender: "female", Age: 24,
			CellLeaderName: "Leader",
			Attendance: domain.Attendance{
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
			groupID, _, _ := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingAllocated)
			tc.setup(groupID)
			_, err := env.Campers.AddCamper(ctx, groupID, "Diane", domain.SkipVersionCheck, newcomer)
			if err == nil {
				t.Fatal("expected the add to be refused")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error = %q, want it to mention %q", err.Error(), tc.wantSub)
			}
		})
	}
}

func TestEditCamper_emailsTheFamilyAndNamesACancelledInvoice(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, nil, nil, cfg)
	repo := env.Repo
	if err := repo.UpdateAccommodationPriceRef(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("set lodge price: %v", err)
	}
	if err := repo.UpdateAccommodationPriceRef(ctx, "tent", "price_tent"); err != nil {
		t.Fatalf("set tent price: %v", err)
	}

	t.Run("plain rename says nothing about invoices", func(t *testing.T) {
		mailer := &billingtest.RecordingMailer{}
		sub := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, mailer, nil, cfg)
		groupID, _, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingInvoiced)
		campers, _ := repo.CampersForGroup(ctx, groupID)
		req := editFor(camperByID(t, campers, otherID))
		req.FirstName = "Deborah"

		if _, err := sub.Campers.EditCamper(
			ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, req); err != nil {
			t.Fatalf("EditCamper: %v", err)
		}
		if len(mailer.BookingUpdated) != 1 {
			t.Fatalf("sent %d booking-updated emails, want 1", len(mailer.BookingUpdated))
		}
		got := mailer.BookingUpdated[0]
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
		mailer := &billingtest.RecordingMailer{}
		sub := billingtest.NewEnv(t, pool, &billingtest.StubStripe{
			VoidInvIdem: func(context.Context, string) error { return nil },
		}, mailer, nil, cfg)
		groupID, _, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingInvoiced)
		if _, err := pool.Exec(ctx,
			`UPDATE registration_groups SET stripe_invoice_id = 'in_live' WHERE id = $1`,
			groupID); err != nil {
			t.Fatalf("seed invoice: %v", err)
		}
		campers, _ := repo.CampersForGroup(ctx, groupID)
		req := editFor(camperByID(t, campers, otherID))
		req.AllocatedAccommodationCode = "tent"

		if _, err := sub.Campers.EditCamper(
			ctx, groupID, otherID, "Diane", domain.SkipVersionCheck, req); err != nil {
			t.Fatalf("EditCamper: %v", err)
		}
		if len(mailer.BookingUpdated) != 1 || !mailer.BookingUpdated[0].InvoiceCancelled {
			t.Fatalf("emails = %+v, want one saying the invoice was cancelled",
				mailer.BookingUpdated)
		}
	})

	t.Run("a mail failure still leaves the change applied", func(t *testing.T) {
		sub := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, failingMailer{}, nil, cfg)
		groupID, _, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingAllocated)
		campers, _ := repo.CampersForGroup(ctx, groupID)
		req := editFor(camperByID(t, campers, otherID))
		req.FirstName = "Deborah"

		if _, err := sub.Campers.EditCamper(
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
	mailer := &billingtest.RecordingMailer{}
	cfg := billing.Config{}
	env := billingtest.NewEnvWithPrices(t, pool, &billingtest.StubStripe{}, mailer, nil, cfg, fakePrices{depositPence: 5000})

	groupID, _, _ := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingAllocated)
	if _, err := env.Campers.AddCamper(
		ctx, groupID, "Diane", domain.SkipVersionCheck, newFullWeekCamper()); err != nil {
		t.Fatalf("AddCamper: %v", err)
	}

	if len(mailer.BookingUpdated) != 1 {
		t.Fatalf("sent %d booking-updated emails, want 1", len(mailer.BookingUpdated))
	}
	got := mailer.BookingUpdated[0]
	if got.AddedName != "Ruth Adeyemi" {
		t.Fatalf("AddedName = %q, want the new arrival named so the charge makes sense",
			got.AddedName)
	}
	if len(got.Campers) != 3 {
		t.Fatalf("roster = %v, want all three people listed", got.Campers)
	}
}

func TestAddCamper_fullWeekReturnsGroupForAllocation(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnvWithPrices(t, pool, &billingtest.StubStripe{}, &billingtest.RecordingMailer{}, nil, cfg, fakePrices{depositPence: 5000})
	repo := env.Repo

	groupID, mainID, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingAllocated)

	sum, err := env.Campers.AddCamper(ctx, groupID, "Diane", domain.SkipVersionCheck, newFullWeekCamper())
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
		if got := billing.StrVal(camperByID(t, campers, id).AllocatedAccommodationCode); got != "lodge" {
			t.Fatalf("existing camper %s allocation = %q, want it kept at lodge", id, got)
		}
	}
	if got := billing.StrVal(camperByID(t, campers, sum.CamperID).AllocatedAccommodationCode); got != "" {
		t.Fatalf("new camper allocation = %q, want them unplaced", got)
	}
}

func TestAddCamper_dayPassOwesNoDeposit(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnvWithPrices(t, pool, &billingtest.StubStripe{}, &billingtest.RecordingMailer{}, nil, cfg, fakePrices{depositPence: 5000})
	repo := env.Repo

	groupID, _, _ := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingAllocated)

	needsCatering := false
	sum, err := env.Campers.AddCamper(ctx, groupID, "Diane", domain.SkipVersionCheck, camperedit.AddCamperRequest{
		Camper: domain.NewCamper{
			FirstName: "Sam", LastName: "Okafor", Gender: "male", Age: 30,
			CellLeaderName: "Leader",
			Attendance: domain.Attendance{
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

func TestAddCamper_laterPriceChangeLeavesEarlierDepositAlone(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	prices := &mutablePrices{depositPence: 5000}
	cfg := billing.Config{}
	env := billingtest.NewEnvWithPrices(t, pool, &billingtest.StubStripe{}, &billingtest.RecordingMailer{}, nil, cfg, prices)
	repo := env.Repo

	groupID, _, _ := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingAllocated)

	first, err := env.Campers.AddCamper(ctx, groupID, "Diane", domain.SkipVersionCheck, newFullWeekCamper())
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
	got, err := env.Campers.AddCamper(ctx, groupID, "Diane", domain.SkipVersionCheck, second)
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

func TestConvertCamper_withOutstandingDepositIssuesNoCredit(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	stub := &billingtest.StubStripe{
		EnsureCust: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "cus_edit", nil
		},
	}
	cfg := billing.Config{
		DepositPricePence: 5000,
		DayPassPriceRef:   "price_day",
	}
	env := billingtest.NewEnv(t, pool, stub, &billingtest.RecordingMailer{}, nil, cfg)
	repo := env.Repo

	groupID, _, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingAllocated)
	if _, err := pool.Exec(ctx,
		`UPDATE registrations SET deposit_owed_pence = 5000 WHERE id = $1`, otherID); err != nil {
		t.Fatalf("seed deposit owed: %v", err)
	}

	needsCatering := false
	if _, err := env.Campers.ConvertCamperToDayVisitor(
		ctx, groupID, otherID, "Diane", domain.SkipVersionCheck,
		camperedit.ConvertToDayVisitorRequest{
			Days:          []string{"mon"},
			TshirtOption:  domain.TshirtOptionNone,
			NeedsCatering: &needsCatering,
		}); err != nil {
		t.Fatalf("ConvertCamperToDayVisitor: %v", err)
	}

	if len(stub.CreditCalls) != 0 {
		t.Fatalf("credits = %+v, want none: they never paid this deposit",
			stub.CreditCalls)
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
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, &billingtest.RecordingMailer{}, nil, cfg)
	repo := env.Repo

	groupID, _, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingAllocated)
	if _, err := pool.Exec(ctx,
		`UPDATE registrations SET deposit_owed_pence = 5000 WHERE id = $1`, otherID); err != nil {
		t.Fatalf("seed deposit owed: %v", err)
	}

	name, err := env.Campers.WaiveCamperDeposit(ctx, groupID, otherID, "Diane", domain.SkipVersionCheck)
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
	if _, err := env.Campers.WaiveCamperDeposit(
		ctx, groupID, otherID, "Diane", domain.SkipVersionCheck); err != nil {
		t.Fatalf("second waive should be a no-op: %v", err)
	}
}

func TestWaiveCamperDeposit_refusedOncePaid(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, &billingtest.RecordingMailer{}, nil, cfg)

	groupID, _, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingBalancePaid)

	_, err := env.Campers.WaiveCamperDeposit(ctx, groupID, otherID, "Diane", domain.SkipVersionCheck)
	if err == nil {
		t.Fatal("expected a waive to be refused once the balance is paid")
	}
}

func TestMakeMainContact_movesTheMarkerExactlyOnce(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	cfg := billing.Config{}
	env := billingtest.NewEnv(t, pool, &billingtest.StubStripe{}, &billingtest.RecordingMailer{}, nil, cfg)
	repo := env.Repo

	groupID, mainID, otherID := billingtest.InsertEditableGroup(ctx, t, pool, domain.BillingAllocated)

	name, err := env.Campers.MakeMainContact(ctx, groupID, otherID, "Diane", domain.SkipVersionCheck)
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

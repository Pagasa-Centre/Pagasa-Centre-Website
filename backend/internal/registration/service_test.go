package registration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/registration/storage"
	"pagasacentre/backend/internal/testhelper"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
)

type fakePriceLookup struct{ amount int }

func (f fakePriceLookup) GetPrice(_ context.Context, code string) (PriceRow, error) {
	if code != domain.PriceDeposit {
		return PriceRow{}, nil
	}
	return PriceRow{AmountPence: f.amount, Currency: "GBP"}, nil
}

func TestDepositPayingCount(t *testing.T) {
	req := domain.SubmitRequest{Campers: []domain.CamperDTO{
		{Age: 30, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek}}, // pays
		{Age: 25, Attendance: domain.AttendanceDTO{Type: domain.AttendanceDayPass}},  // free (day pass)
		{Age: 10, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek}}, // pays
		{Age: 3, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek}},  // free (under 4)
		{Age: 4, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek}},  // pays (4 == threshold)
	}}
	if got := depositPayingCount(req); got != 3 {
		t.Errorf("depositPayingCount = %d, want 3", got)
	}
}

func TestComputeTotal_DepositPerPayingCamper(t *testing.T) {
	svc := &Service{prices: fakePriceLookup{amount: 5000}}
	req := domain.SubmitRequest{Campers: []domain.CamperDTO{
		{Age: 30, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek}}, // £50
		{Age: 30, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek}}, // £50
		{Age: 25, Attendance: domain.AttendanceDTO{Type: domain.AttendanceDayPass}},  // free
	}}
	total, currency, err := svc.computeTotal(context.Background(), req)
	if err != nil {
		t.Fatalf("computeTotal: %v", err)
	}
	if total != 10000 {
		t.Errorf("total = %d, want 10000", total)
	}
	if currency != "GBP" {
		t.Errorf("currency = %q, want GBP", currency)
	}
}

func TestComputeTotal_UnderThreesAreFree(t *testing.T) {
	svc := &Service{prices: fakePriceLookup{amount: 5000}}
	req := domain.SubmitRequest{Campers: []domain.CamperDTO{
		{Age: 30, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek}}, // £50
		{Age: 3, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek}},  // free (under 4)
		{Age: 1, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek}},  // free
	}}
	total, _, err := svc.computeTotal(context.Background(), req)
	if err != nil {
		t.Fatalf("computeTotal: %v", err)
	}
	if total != 5000 {
		t.Errorf("total = %d, want 5000 (only the adult pays)", total)
	}
}

func TestComputeTotal_DayPassOnlyIsZero(t *testing.T) {
	svc := &Service{prices: fakePriceLookup{amount: 5000}}
	req := domain.SubmitRequest{Campers: []domain.CamperDTO{
		{Age: 25, Attendance: domain.AttendanceDTO{Type: domain.AttendanceDayPass}},
		{Age: 25, Attendance: domain.AttendanceDTO{Type: domain.AttendanceDayPass}},
	}}
	total, _, err := svc.computeTotal(context.Background(), req)
	if err != nil {
		t.Fatalf("computeTotal: %v", err)
	}
	if total != 0 {
		t.Errorf("day-pass-only total = %d, want 0", total)
	}
}

func TestComputeTotal_AllUnderThreesIsZero(t *testing.T) {
	// Pathological but worth covering: a family registers only their two
	// toddlers full-week. Total should be £0 and the Submit path should
	// skip Stripe.
	svc := &Service{prices: fakePriceLookup{amount: 5000}}
	req := domain.SubmitRequest{Campers: []domain.CamperDTO{
		{Age: 1, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek}},
		{Age: 2, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek}},
	}}
	total, _, err := svc.computeTotal(context.Background(), req)
	if err != nil {
		t.Fatalf("computeTotal: %v", err)
	}
	if total != 0 {
		t.Errorf("all-toddlers total = %d, want 0", total)
	}
}

type fakeCampOpen struct {
	mode string
}

func (f fakeCampOpen) RegistrationsOpen(context.Context) (bool, error) { return true, nil }

func (f fakeCampOpen) RegistrationPaymentMode(context.Context) (string, error) {
	if f.mode == "" {
		return domain.PaymentModeDeposit, nil
	}
	return f.mode, nil
}

func newIntegrationService(pool *pgxpool.Pool) *Service {
	return NewService(storage.NewRepository(pool), fakePriceLookup{amount: 5000}, nil, nil, fakeCampOpen{}, nil, nil, "", Config{})
}

func seedUnusedFreeCode(t *testing.T, ctx context.Context, pool *pgxpool.Pool, code string) {
	t.Helper()
	_, err := pool.Exec(ctx,
		`INSERT INTO free_codes (code, created_by) VALUES ($1, 'Test')`, code)
	if err != nil {
		t.Fatalf("insert free code: %v", err)
	}
}

func seedUsedFreeCode(t *testing.T, ctx context.Context, pool *pgxpool.Pool, code string) {
	t.Helper()
	var groupID string
	err := pool.QueryRow(ctx, `
		INSERT INTO registration_groups (
			contact_first_name, contact_last_name, contact_email, contact_phone,
			payment_status, total_amount_pence, currency, is_free
		) VALUES ('Prior', 'User', 'prior@example.com', '07000000000', 'paid', 0, 'GBP', true)
		RETURNING id`).Scan(&groupID)
	if err != nil {
		t.Fatalf("insert group: %v", err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO free_codes (code, created_by, used_at, used_by_group_id)
		VALUES ($1, 'Test', now(), $2)`, code, groupID)
	if err != nil {
		t.Fatalf("insert used free code: %v", err)
	}
}

func seedRevokedFreeCode(t *testing.T, ctx context.Context, pool *pgxpool.Pool, code string) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO free_codes (code, created_by, revoked_at)
		VALUES ($1, 'Test', now())`, code)
	if err != nil {
		t.Fatalf("insert revoked free code: %v", err)
	}
}

func sponsoredRequest(code, email string) domain.SubmitRequest {
	req := validRequest()
	req.FreeCode = code
	req.Contact.Email = email
	return req
}

func countGroupsForEmail(t *testing.T, ctx context.Context, pool *pgxpool.Pool, email string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM registration_groups WHERE contact_email = $1`, email).Scan(&n); err != nil {
		t.Fatalf("count groups for %s: %v", email, err)
	}
	return n
}

func assertAPIErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != code {
		t.Fatalf("expected APIError code %q, got %v", code, err)
	}
}

func TestMaskFreeCode(t *testing.T) {
	cases := []struct{ in, want string }{
		{"SPON-Z9FEHB9Y", "*********HB9Y"},
		{"ABCD", "****"},
		{"XYZ", "***"},
		{"", ""},
	}
	for _, c := range cases {
		if got := MaskFreeCode(c.in); got != c.want {
			t.Errorf("MaskFreeCode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSubmit_invalidFreeCode(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	svc := newIntegrationService(pool)

	_, err := svc.Submit(ctx, sponsoredRequest("SPON-NOSUCH01", "bad@example.com"))
	assertAPIErrorCode(t, err, "invalid_free_code")
	if countGroupsForEmail(t, ctx, pool, "bad@example.com") != 0 {
		t.Fatalf("expected no group row for submitter after invalid code")
	}
}

func TestSubmit_usedFreeCodeRejected(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	svc := newIntegrationService(pool)
	seedUsedFreeCode(t, ctx, pool, "SPON-USED0001")

	_, err := svc.Submit(ctx, sponsoredRequest("SPON-USED0001", "used@example.com"))
	assertAPIErrorCode(t, err, "invalid_free_code")
	if countGroupsForEmail(t, ctx, pool, "used@example.com") != 0 {
		t.Fatalf("expected no group for rejected submitter")
	}
	if countGroupsForEmail(t, ctx, pool, "prior@example.com") != 1 {
		t.Fatalf("expected only the pre-seeded group")
	}
}

func TestSubmit_revokedFreeCodeRejected(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	svc := newIntegrationService(pool)
	seedRevokedFreeCode(t, ctx, pool, "SPON-REVOK001")

	_, err := svc.Submit(ctx, sponsoredRequest("SPON-REVOK001", "revoked@example.com"))
	assertAPIErrorCode(t, err, "invalid_free_code")
	if countGroupsForEmail(t, ctx, pool, "revoked@example.com") != 0 {
		t.Fatalf("expected no group row for submitter after revoked code")
	}
}

func TestSubmit_validFreeCode(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := newIntegrationService(pool)
	seedUnusedFreeCode(t, ctx, pool, "SPON-VALID001")

	resp, err := svc.Submit(ctx, sponsoredRequest("spon-valid001", "free@example.com"))
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if resp.TotalAmountPence != 0 {
		t.Fatalf("total = %d, want 0", resp.TotalAmountPence)
	}
	if resp.CheckoutURL != "" {
		t.Fatalf("expected no checkout URL for sponsored registration")
	}

	g, err := repo.FindGroupByID(ctx, resp.GroupID)
	if err != nil || g == nil {
		t.Fatalf("find group: %v", err)
	}
	if !g.IsFree {
		t.Fatal("expected is_free=true")
	}
	if g.PaymentStatus != domain.PaymentPaid {
		t.Fatalf("payment_status = %q, want paid", g.PaymentStatus)
	}
	if g.TotalAmountPence != 0 {
		t.Fatalf("stored total = %d, want 0", g.TotalAmountPence)
	}

	var usedBy *string
	err = pool.QueryRow(ctx,
		`SELECT used_by_group_id FROM free_codes WHERE code = $1`, "SPON-VALID001").Scan(&usedBy)
	if err != nil {
		t.Fatalf("lookup code: %v", err)
	}
	if usedBy == nil || *usedBy != resp.GroupID {
		t.Fatalf("code not marked used by group %s", resp.GroupID)
	}
}

func TestSubmit_doubleRedeemFreeCode(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	svc := newIntegrationService(pool)
	seedUnusedFreeCode(t, ctx, pool, "SPON-DOUBLE01")

	if _, err := svc.Submit(ctx, sponsoredRequest("SPON-DOUBLE01", "first@example.com")); err != nil {
		t.Fatalf("first Submit: %v", err)
	}
	_, err := svc.Submit(ctx, sponsoredRequest("SPON-DOUBLE01", "second@example.com"))
	assertAPIErrorCode(t, err, "invalid_free_code")
	if countGroupsForEmail(t, ctx, pool, "second@example.com") != 0 {
		t.Fatalf("expected no group for second redeem attempt")
	}
	if countGroupsForEmail(t, ctx, pool, "first@example.com") != 1 {
		t.Fatalf("expected exactly one group for first redeem")
	}
}

func setAccommodationAvailability(t *testing.T, ctx context.Context, pool *pgxpool.Pool, code string, available bool) {
	t.Helper()
	if _, err := pool.Exec(ctx,
		`UPDATE accommodation_types SET available_for_registration = $1 WHERE code = $2`,
		available, code); err != nil {
		t.Fatalf("set availability %s: %v", code, err)
	}
}

func TestAccommodationAvailabilityRepo_roundTrip(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	t.Cleanup(func() { setAccommodationAvailability(t, context.Background(), pool, "lodge", true) })

	if err := repo.SetAccommodationAvailableForRegistration(ctx, "lodge", false); err != nil {
		t.Fatalf("set availability: %v", err)
	}

	avail, err := repo.AccommodationAvailability(ctx)
	if err != nil {
		t.Fatalf("availability map: %v", err)
	}
	if avail["lodge"] {
		t.Fatalf("expected lodge disabled in availability map, got %v", avail)
	}

	types, err := repo.ListAccommodationTypes(ctx)
	if err != nil {
		t.Fatalf("list types: %v", err)
	}
	var found bool
	for _, ty := range types {
		if ty.Code == "lodge" {
			found = true
			if ty.AvailableForRegistration {
				t.Fatalf("expected lodge.AvailableForRegistration=false")
			}
		}
	}
	if !found {
		t.Fatal("lodge not present in ListAccommodationTypes")
	}

	if err := repo.SetAccommodationAvailableForRegistration(ctx, "does_not_exist", false); err == nil {
		t.Fatal("expected not-found error for unknown code")
	}
}

func TestSubmit_disabledAccommodationRejected(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	svc := newIntegrationService(pool)

	// validRequest picks lodge (1st) / cabin (2nd); disable the 1st choice.
	// accommodation_types isn't truncated between tests, so restore afterwards.
	setAccommodationAvailability(t, ctx, pool, "lodge", false)
	t.Cleanup(func() { setAccommodationAvailability(t, context.Background(), pool, "lodge", true) })

	_, err := svc.Submit(ctx, validRequest())
	assertAPIErrorCode(t, err, "accommodation_unavailable")
	if countGroupsForEmail(t, ctx, pool, "jane@example.com") != 0 {
		t.Fatalf("expected no group row after disabled-accommodation submit")
	}
}

func TestSubmit_availableAccommodationSucceeds(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	svc := newIntegrationService(pool)
	seedUnusedFreeCode(t, ctx, pool, "SPON-AVAIL001")

	// lodge/cabin are available by default; the sponsored path skips Stripe so
	// this proves the guard doesn't over-reject enabled tiers.
	if _, err := svc.Submit(ctx, sponsoredRequest("SPON-AVAIL001", "avail@example.com")); err != nil {
		t.Fatalf("Submit with available accommodation: %v", err)
	}
}

func TestSubmit_dayPassBlankAccommodationSkipsGuard(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	svc := newIntegrationService(pool)
	seedUnusedFreeCode(t, ctx, pool, "SPON-DAYPASS1")

	// Even with a tier disabled, a day-pass camper (blank accommodation) is
	// unaffected by the guard.
	setAccommodationAvailability(t, ctx, pool, "lodge", false)
	t.Cleanup(func() { setAccommodationAvailability(t, context.Background(), pool, "lodge", true) })

	req := sponsoredRequest("SPON-DAYPASS1", "daypass@example.com")
	req.Campers[0].Attendance = domain.AttendanceDTO{
		Type:          domain.AttendanceDayPass,
		Days:          []string{"mon"},
		TshirtOption:  domain.TshirtOptionNone,
		ShirtSize:     domain.ShirtSizeNotApplicable,
		NeedsCatering: boolPtr(false),
	}
	if _, err := svc.Submit(ctx, req); err != nil {
		t.Fatalf("day-pass Submit should skip accommodation guard: %v", err)
	}
}

func TestRevokeFreeCode_rejectsUsedCode(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := newIntegrationService(pool)
	seedUsedFreeCode(t, ctx, pool, "SPON-NOREVOK1")

	codes, err := repo.ListFreeCodes(ctx)
	if err != nil {
		t.Fatalf("ListFreeCodes: %v", err)
	}
	var id string
	for _, c := range codes {
		if c.Code == "SPON-NOREVOK1" {
			id = c.ID
			break
		}
	}
	if id == "" {
		t.Fatal("used code not found")
	}

	err = svc.RevokeFreeCode(ctx, id)
	assertAPIErrorCode(t, err, "bad_request")
}

func TestRowsFromRequest_dayPassIncludesShirtSize(t *testing.T) {
	req := domain.SubmitRequest{
		Contact: domain.ContactDTO{
			FirstName: "Day",
			LastName:  "Passer",
			Email:     "day@example.com",
			Phone:     "07000000000",
		},
		Campers: []domain.CamperDTO{{
			FirstName: "Day",
			LastName:  "Passer",
			Attendance: domain.AttendanceDTO{
				Type:         domain.AttendanceDayPass,
				Days:         []string{"mon", "tue"},
				TshirtOption: domain.TshirtOptionTshirtOnly,
				ShirtSize:    "adult_l",
			},
		}},
	}
	rows := rowsFromRequest("grp-1", req, domain.PaymentPaid, 0, "GBP", mustParseTime(t, "2026-07-01T12:00:00Z"), nil)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].ShirtSize == nil || *rows[0].ShirtSize != "adult_l" {
		t.Fatalf("shirt_size = %v, want adult_l", rows[0].ShirtSize)
	}
	if rows[0].DayPassTshirtOption == nil || *rows[0].DayPassTshirtOption != domain.TshirtOptionTshirtOnly {
		t.Fatalf("day_pass_tshirt_option = %v", rows[0].DayPassTshirtOption)
	}
	vals := rows[0].Values()
	// shirt_size is column index 18 (0-based) per sheets.Headers
	if len(vals) < 19 {
		t.Fatalf("values too short: %d", len(vals))
	}
	if vals[18] != "adult_l" {
		t.Fatalf("sheet shirt_size column = %v, want adult_l", vals[18])
	}
}

func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	return ts
}

type fakeStripeAmounts struct {
	amounts map[string]struct {
		pence    int64
		currency string
	}
}

func (f fakeStripeAmounts) Amount(_ context.Context, priceID string) (int64, string, error) {
	if a, ok := f.amounts[priceID]; ok {
		return a.pence, a.currency, nil
	}
	return 0, "GBP", fmt.Errorf("unknown price %q", priceID)
}

func TestComputeCharge_depositModeSingleLine(t *testing.T) {
	svc := &Service{prices: fakePriceLookup{amount: 5000}}
	req := domain.SubmitRequest{Campers: []domain.CamperDTO{
		{Age: 30, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek}},
		{Age: 30, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek}},
	}}
	ch, err := svc.computeCharge(context.Background(), req, domain.PaymentModeDeposit)
	if err != nil {
		t.Fatalf("computeCharge: %v", err)
	}
	if ch.totalPence != 10000 {
		t.Errorf("total = %d, want 10000", ch.totalPence)
	}
	if len(ch.lines) != 1 {
		t.Fatalf("lines = %d, want 1 deposit line", len(ch.lines))
	}
	if ch.lines[0].Quantity != 1 {
		t.Errorf("deposit line quantity = %d, want 1 (aggregated)", ch.lines[0].Quantity)
	}
}

func TestComputeCharge_fullModeMixedGroup(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	if err := repo.UpdateAccommodationStripePrice(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("seed lodge price: %v", err)
	}
	if err := repo.UpdateAccommodationStripePrice(ctx, "child", "price_child"); err != nil {
		t.Fatalf("seed child price: %v", err)
	}

	amounts := fakeStripeAmounts{amounts: map[string]struct {
		pence    int64
		currency string
	}{
		"price_lodge":       {25000, "GBP"},
		"price_child_under3": {0, "GBP"},
		"price_daypass":     {1500, "GBP"},
		"price_coach":       {2000, "GBP"},
	}}
	svc := &Service{
		repo:          repo,
		prices:        fakePriceLookup{amount: 5000},
		stripeAmounts: amounts,
		cfg: Config{
			StripePriceChildUnder3: "price_child_under3",
			StripePriceDayPass:     "price_daypass",
			StripePriceCoach:       "price_coach",
		},
	}

	yes := true
	req := domain.SubmitRequest{Campers: []domain.CamperDTO{
		{Age: 30, Attendance: domain.AttendanceDTO{
			Type: domain.AttendanceFullWeek, AccommodationFirstChoice: "lodge",
		}},
		{Age: 2, Attendance: domain.AttendanceDTO{
			Type: domain.AttendanceFullWeek, AccommodationFirstChoice: "child",
		}},
		{Age: 25, Attendance: domain.AttendanceDTO{
			Type: domain.AttendanceDayPass, Days: []string{"mon", "tue", "wed"},
		}},
		{Age: 30, Attendance: domain.AttendanceDTO{
			Type: domain.AttendanceFullWeek, AccommodationFirstChoice: "lodge",
			NeedsCoach: &yes,
		}},
	}}
	ch, err := svc.computeCharge(ctx, req, domain.PaymentModeFull)
	if err != nil {
		t.Fatalf("computeCharge: %v", err)
	}
	// deposit: 2 paying full-week adults (under-4 free) = 2 × £50 = 10000
	// lodge ×2 = 50000, day pass 3 days = 4500, coach ×1 = 2000
	want := 10000 + 50000 + 4500 + 2000
	if ch.totalPence != want {
		t.Errorf("total = %d, want %d", ch.totalPence, want)
	}
}

func TestComputeCharge_tierCollapse(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	if err := repo.UpdateAccommodationStripePrice(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("seed lodge price: %v", err)
	}
	amounts := fakeStripeAmounts{amounts: map[string]struct {
		pence    int64
		currency string
	}{"price_lodge": {25000, "GBP"}}}
	svc := &Service{
		repo:          repo,
		prices:        fakePriceLookup{amount: 5000},
		stripeAmounts: amounts,
		cfg:           Config{},
	}
	req := domain.SubmitRequest{Campers: []domain.CamperDTO{
		{Age: 30, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek, AccommodationFirstChoice: "lodge"}},
		{Age: 28, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek, AccommodationFirstChoice: "lodge"}},
	}}
	ch, err := svc.computeCharge(ctx, req, domain.PaymentModeFull)
	if err != nil {
		t.Fatalf("computeCharge: %v", err)
	}
	var lodgeLines int
	for _, ln := range ch.lines {
		if ln.Quantity == 2 && ln.AmountPence == 25000 {
			lodgeLines++
		}
	}
	if lodgeLines != 1 {
		t.Fatalf("expected one lodge line with qty 2, got lines %+v", ch.lines)
	}
}

func TestComputeCharge_zeroUnitOmitted(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	if err := repo.UpdateAccommodationStripePrice(ctx, "child", "price_child"); err != nil {
		t.Fatalf("seed child price: %v", err)
	}
	amounts := fakeStripeAmounts{amounts: map[string]struct {
		pence    int64
		currency string
	}{
		"price_child":        {10000, "GBP"},
		"price_child_under3": {0, "GBP"},
	}}
	svc := &Service{
		repo:          repo,
		prices:        fakePriceLookup{amount: 5000},
		stripeAmounts: amounts,
		cfg:           Config{StripePriceChildUnder3: "price_child_under3"},
	}
	req := domain.SubmitRequest{Campers: []domain.CamperDTO{
		{Age: 2, Attendance: domain.AttendanceDTO{Type: domain.AttendanceFullWeek, AccommodationFirstChoice: "child"}},
	}}
	ch, err := svc.computeCharge(ctx, req, domain.PaymentModeFull)
	if err != nil {
		t.Fatalf("computeCharge: %v", err)
	}
	for _, ln := range ch.lines {
		if ln.AmountPence == 0 {
			t.Fatalf("£0 line should be omitted, got %+v", ln)
		}
	}
}

func TestComputeCharge_dayPassOnlyNonZeroInFullMode(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	amounts := fakeStripeAmounts{amounts: map[string]struct {
		pence    int64
		currency string
	}{"price_daypass": {1500, "GBP"}}}
	svc := &Service{
		repo:          repo,
		prices:        fakePriceLookup{amount: 5000},
		stripeAmounts: amounts,
		cfg:           Config{StripePriceDayPass: "price_daypass"},
	}
	req := domain.SubmitRequest{Campers: []domain.CamperDTO{
		{Age: 25, Attendance: domain.AttendanceDTO{Type: domain.AttendanceDayPass, Days: []string{"mon", "tue"}}},
	}}
	ch, err := svc.computeCharge(ctx, req, domain.PaymentModeFull)
	if err != nil {
		t.Fatalf("computeCharge: %v", err)
	}
	if ch.totalPence != 3000 {
		t.Errorf("total = %d, want 3000", ch.totalPence)
	}
}

func TestComputeCharge_missingDayPassPrice(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	svc := &Service{
		repo:          repo,
		prices:        fakePriceLookup{amount: 5000},
		stripeAmounts: fakeStripeAmounts{amounts: map[string]struct {
			pence    int64
			currency string
		}{}},
		cfg: Config{},
	}
	req := domain.SubmitRequest{Campers: []domain.CamperDTO{
		{Age: 25, Attendance: domain.AttendanceDTO{Type: domain.AttendanceDayPass, Days: []string{"mon"}}},
	}}
	_, err := svc.computeCharge(ctx, req, domain.PaymentModeFull)
	if err == nil || !strings.Contains(err.Error(), "STRIPE_PRICE_DAY_PASS") {
		t.Fatalf("expected day pass config error, got %v", err)
	}
}

func TestValidateFullPaymentPricing_missingCoach(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	if err := repo.UpdateAccommodationStripePrice(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("seed lodge price: %v", err)
	}
	amounts := fakeStripeAmounts{amounts: map[string]struct {
		pence    int64
		currency string
	}{
		"price_lodge":         {25000, "GBP"},
		"price_child_under3":  {5000, "GBP"},
		"price_daypass":       {1500, "GBP"},
	}}
	svc := NewService(repo, fakePriceLookup{amount: 5000}, nil, amounts, fakeCampOpen{}, nil, nil, "", Config{
		StripePriceChildUnder3: "price_child_under3",
		StripePriceDayPass:     "price_daypass",
	})
	err := svc.ValidateFullPaymentPricing(ctx)
	if err == nil {
		t.Fatal("expected preflight failure for missing coach price")
	}
	if !strings.Contains(err.Error(), "Coach") {
		t.Fatalf("expected coach named in error, got %v", err)
	}
}

func TestValidateFullPaymentPricing_currencyMismatch(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)
	if err := repo.UpdateAccommodationStripePrice(ctx, "lodge", "price_lodge"); err != nil {
		t.Fatalf("seed lodge price: %v", err)
	}
	amounts := fakeStripeAmounts{amounts: map[string]struct {
		pence    int64
		currency string
	}{
		"price_lodge":        {25000, "USD"},
		"price_child_under3": {5000, "GBP"},
		"price_daypass":      {1500, "GBP"},
		"price_coach":        {2000, "GBP"},
	}}
	svc := NewService(repo, fakePriceLookup{amount: 5000}, nil, amounts, fakeCampOpen{}, nil, nil, "", Config{
		StripePriceChildUnder3: "price_child_under3",
		StripePriceDayPass:     "price_daypass",
		StripePriceCoach:       "price_coach",
	})
	err := svc.ValidateFullPaymentPricing(ctx)
	if err == nil {
		t.Fatal("expected currency mismatch failure")
	}
	if !strings.Contains(err.Error(), "currency") {
		t.Fatalf("expected currency in error, got %v", err)
	}
}

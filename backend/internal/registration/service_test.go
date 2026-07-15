package registration

import (
	"context"
	"errors"
	"testing"

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

type fakeCampOpen struct{}

func (fakeCampOpen) RegistrationsOpen(context.Context) (bool, error) { return true, nil }

func newIntegrationService(pool *pgxpool.Pool) *Service {
	return NewService(storage.NewRepository(pool), fakePriceLookup{amount: 5000}, nil, fakeCampOpen{}, nil, nil, "")
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

func countGroups(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM registration_groups`).Scan(&n); err != nil {
		t.Fatalf("count groups: %v", err)
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
	if countGroups(t, ctx, pool) != 0 {
		t.Fatalf("expected no group row after invalid code")
	}
}

func TestSubmit_usedFreeCodeRejected(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	svc := newIntegrationService(pool)
	seedUsedFreeCode(t, ctx, pool, "SPON-USED0001")

	_, err := svc.Submit(ctx, sponsoredRequest("SPON-USED0001", "used@example.com"))
	assertAPIErrorCode(t, err, "invalid_free_code")
	if countGroups(t, ctx, pool) != 1 {
		t.Fatalf("expected only the pre-seeded group, got %d", countGroups(t, ctx, pool))
	}
}

func TestSubmit_revokedFreeCodeRejected(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	svc := newIntegrationService(pool)
	seedRevokedFreeCode(t, ctx, pool, "SPON-REVOK001")

	_, err := svc.Submit(ctx, sponsoredRequest("SPON-REVOK001", "revoked@example.com"))
	assertAPIErrorCode(t, err, "invalid_free_code")
	if countGroups(t, ctx, pool) != 0 {
		t.Fatalf("expected no group row after revoked code")
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
	if countGroups(t, ctx, pool) != 1 {
		t.Fatalf("expected exactly one group after double-redeem attempt, got %d", countGroups(t, ctx, pool))
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
	if countGroups(t, ctx, pool) != 0 {
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

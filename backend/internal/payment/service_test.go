package payment_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"pagasacentre/backend/internal/email"
	"pagasacentre/backend/internal/payment"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/registration/storage"
	"pagasacentre/backend/internal/testhelper"
)

type recordingMailer struct {
	mu    sync.Mutex
	calls []email.DepositConfirmation
}

func (m *recordingMailer) SendAllocationReleased(context.Context, email.AllocationReleased) error {
	return nil
}

func (m *recordingMailer) SendWhiteTeamNotification(context.Context, email.WhiteTeamNotification) error {
	return nil
}

func (m *recordingMailer) SendBalanceInvoice(context.Context, email.BalanceInvoice) error {
	return nil
}

func (m *recordingMailer) SendBalancePaid(context.Context, email.BalancePaid) error {
	return nil
}

func (m *recordingMailer) SendBalancePaidConfirmation(context.Context, email.BalancePaidConfirmation) error {
	return nil
}

func (m *recordingMailer) SendSponsorshipConfirmed(context.Context, email.SponsorshipConfirmed) error {
	return nil
}

func (m *recordingMailer) SendCoachInvoice(context.Context, email.CoachInvoice) error {
	return nil
}

func (m *recordingMailer) SendAccommodationChanged(context.Context, email.AccommodationChangedNotice) error {
	return nil
}

func (m *recordingMailer) SendDepositConfirmation(_ context.Context, p email.DepositConfirmation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, p)
	return nil
}

func (m *recordingMailer) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

// seedPendingGroup commits a pending registration_group + camper with the given
// session id. Returns the group id.
func seedPendingGroup(t *testing.T, ctx context.Context, repo *storage.Repository, sessID, emailAddr string) string {
	t.Helper()
	tx, err := repo.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	req := domain.SubmitRequest{
		Contact: domain.ContactDTO{
			FirstName: "Web", LastName: "Hook", Email: emailAddr, Phone: "+44 0",
		},
		Campers: []domain.CamperDTO{{
			FirstName: "Web", LastName: "Hook", Gender: "female", Age: 25,
			CellLeaderName: "Pastor", IsMainContact: true,
			Attendance: domain.AttendanceDTO{
				Type:                      domain.AttendanceFullWeek,
				ShirtSize:                 "adult_m",
				AccommodationFirstChoice:  "lodge",
				AccommodationSecondChoice: "cabin",
			},
		}},
	}
	gid, err := repo.InsertGroup(ctx, tx, req, 5000, "GBP", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.InsertCamper(ctx, tx, gid, req.Campers[0]); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetStripeSession(ctx, tx, gid, sessID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return gid
}

func seedPendingPrepaidGroup(t *testing.T, ctx context.Context, repo *storage.Repository, sessID, emailAddr string, withCoach bool) string {
	t.Helper()
	tx, err := repo.Pool().BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	needsCoach := withCoach
	req := domain.SubmitRequest{
		Contact: domain.ContactDTO{
			FirstName: "Pre", LastName: "Paid", Email: emailAddr, Phone: "+44 0",
		},
		Campers: []domain.CamperDTO{{
			FirstName: "Pre", LastName: "Paid", Gender: "female", Age: 25,
			CellLeaderName: "Pastor", IsMainContact: true,
			Attendance: domain.AttendanceDTO{
				Type:                      domain.AttendanceFullWeek,
				ShirtSize:                 "adult_m",
				AccommodationFirstChoice:  "lodge",
				AccommodationSecondChoice: "cabin",
				NeedsCoach:                &needsCoach,
			},
		}},
	}
	gid, err := repo.InsertGroup(ctx, tx, req, 70000, "GBP", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.InsertCamper(ctx, tx, gid, req.Campers[0]); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetStripeSession(ctx, tx, gid, sessID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return gid
}

func TestHandleCheckoutCompleted_MarksPaidAndSendsEmail(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)

	_ = seedPendingGroup(t, ctx, repo, "sess_happy", "happy@example.com")
	mailer := &recordingMailer{}
	svc := payment.NewService(pool, repo, mailer, nil, "http://localhost:8080")

	if err := svc.HandleCheckoutCompleted(ctx, payment.CheckoutCompleted{
		SessionID:       "sess_happy",
		PaymentIntentID: "pi_happy",
	}); err != nil {
		t.Fatalf("HandleCheckoutCompleted: %v", err)
	}

	var status string
	if err := pool.QueryRow(ctx,
		`SELECT payment_status FROM registration_groups WHERE stripe_session_id = $1`,
		"sess_happy").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "paid" {
		t.Errorf("expected paid, got %q", status)
	}
	if mailer.callCount() != 1 {
		t.Errorf("expected 1 email, got %d", mailer.callCount())
	}
}

// TestHandleCheckoutCompleted_Idempotent ensures replaying the same event is a
// no-op (no duplicate email).
func TestHandleCheckoutCompleted_Idempotent(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)

	_ = seedPendingGroup(t, ctx, repo, "sess_idem", "idem@example.com")
	mailer := &recordingMailer{}
	svc := payment.NewService(pool, repo, mailer, nil, "http://localhost:8080")

	for i := 0; i < 3; i++ {
		if err := svc.HandleCheckoutCompleted(ctx, payment.CheckoutCompleted{
			SessionID:       "sess_idem",
			PaymentIntentID: "pi_idem",
		}); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if mailer.callCount() != 1 {
		t.Errorf("expected exactly 1 email across 3 replays, got %d", mailer.callCount())
	}
}

func TestHandleCheckoutCompleted_prepaidSetsBalancePaid(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)

	_ = seedPendingPrepaidGroup(t, ctx, repo, "sess_prepaid", "prepaid@example.com", true)
	mailer := &recordingMailer{}
	svc := payment.NewService(pool, repo, mailer, nil, "http://localhost:8080")

	if err := svc.HandleCheckoutCompleted(ctx, payment.CheckoutCompleted{
		SessionID:       "sess_prepaid",
		PaymentIntentID: "pi_prepaid",
	}); err != nil {
		t.Fatalf("HandleCheckoutCompleted: %v", err)
	}

	var paymentStatus, billingStatus string
	var coachIncluded bool
	var balancePaidAt *time.Time
	err := pool.QueryRow(ctx, `
		SELECT payment_status, billing_status, coach_included_in_balance, balance_paid_at
		FROM registration_groups WHERE stripe_session_id = $1`, "sess_prepaid").
		Scan(&paymentStatus, &billingStatus, &coachIncluded, &balancePaidAt)
	if err != nil {
		t.Fatal(err)
	}
	if paymentStatus != "paid" {
		t.Errorf("payment_status = %q, want paid", paymentStatus)
	}
	if billingStatus != "balance_paid" {
		t.Errorf("billing_status = %q, want balance_paid", billingStatus)
	}
	if !coachIncluded {
		t.Error("expected coach_included_in_balance=true for coach-eligible prepaid group")
	}
	if balancePaidAt == nil {
		t.Error("expected balance_paid_at set")
	}
	if mailer.callCount() != 1 {
		t.Errorf("expected 1 email, got %d", mailer.callCount())
	}
}

func TestHandleCheckoutCompleted_prepaidIdempotent(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()
	repo := storage.NewRepository(pool)

	_ = seedPendingPrepaidGroup(t, ctx, repo, "sess_prepaid_idem", "prepaid-idem@example.com", false)
	mailer := &recordingMailer{}
	svc := payment.NewService(pool, repo, mailer, nil, "http://localhost:8080")

	for i := 0; i < 3; i++ {
		if err := svc.HandleCheckoutCompleted(ctx, payment.CheckoutCompleted{
			SessionID:       "sess_prepaid_idem",
			PaymentIntentID: "pi_prepaid_idem",
		}); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if mailer.callCount() != 1 {
		t.Errorf("expected exactly 1 email across 3 replays, got %d", mailer.callCount())
	}
}

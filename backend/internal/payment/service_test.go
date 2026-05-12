package payment_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"

	"pagasacentre/backend/internal/accommodation"
	"pagasacentre/backend/internal/payment"
	"pagasacentre/backend/internal/registration"
	"pagasacentre/backend/internal/testhelper"
)

type fakeRefunder struct {
	calls atomic.Int32
	last  string
}

func (f *fakeRefunder) Refund(_ context.Context, pi string) error {
	f.calls.Add(1)
	f.last = pi
	return nil
}

// TestWebhookCapacityRace simulates two concurrent webhook deliveries for the
// last remaining slot in an accommodation type. Exactly one group must end up
// 'paid'; the other must be 'failed_capacity' and a refund must have been
// requested.
func TestWebhookCapacityRace(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()

	// Constrain the lodge to a single remaining seat for this test.
	if _, err := pool.Exec(ctx,
		`UPDATE accommodation_types SET capacity = 1 WHERE code = 'lodge'`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(),
			`UPDATE accommodation_types SET capacity = 24 WHERE code = 'lodge'`)
	})

	regRepo := registration.NewRepository(pool)
	accRepo := accommodation.NewRepository(pool)

	// Build two pending groups, each requesting the lodge.
	mkGroup := func(email string) string {
		tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		req := registration.SubmitRequest{
			Contact: registration.ContactDTO{
				FirstName: "Race", LastName: "Test", Email: email, Phone: "+44 0",
			},
			Campers: []registration.CamperDTO{{
				FirstName: "Race", LastName: "Test", Gender: "female", Age: 25,
				CellLeaderName: "Pastor", IsMainContact: true,
				Attendance: registration.AttendanceDTO{
					Type:              registration.AttendanceFullWeek,
					ShirtSize:         "adult_m",
					AccommodationCode: "lodge",
				},
			}},
		}
		groupID, err := regRepo.InsertGroup(ctx, tx, req, 0, "GBP")
		if err != nil {
			t.Fatal(err)
		}
		if err := regRepo.InsertCamper(ctx, tx, groupID, req.Campers[0]); err != nil {
			t.Fatal(err)
		}
		if err := regRepo.SetStripeSession(ctx, tx, groupID, "sess_"+email); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		return groupID
	}

	a := mkGroup("a@example.com")
	b := mkGroup("b@example.com")
	_ = a
	_ = b

	refund := &fakeRefunder{}
	svc := payment.NewService(pool, regRepo, payment.NewAccommodationLocker(accRepo), refund)

	var wg sync.WaitGroup
	wg.Add(2)
	errs := make(chan error, 2)
	run := func(sessID, pi string) {
		defer wg.Done()
		if err := svc.HandleCheckoutCompleted(ctx, payment.CheckoutCompleted{
			SessionID:       sessID,
			PaymentIntentID: pi,
		}); err != nil {
			errs <- err
		}
	}
	go run("sess_a@example.com", "pi_a")
	go run("sess_b@example.com", "pi_b")
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("handler error: %v", err)
	}

	statuses := map[string]int{}
	rows, err := pool.Query(ctx,
		`SELECT payment_status FROM registration_groups
		   WHERE stripe_session_id IN ('sess_a@example.com','sess_b@example.com')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		statuses[s]++
	}
	if statuses["paid"] != 1 || statuses["failed_capacity"] != 1 {
		t.Fatalf("expected one paid + one failed_capacity, got %v", statuses)
	}
	if refund.calls.Load() != 1 {
		t.Fatalf("expected exactly one refund, got %d", refund.calls.Load())
	}
}

// TestWebhookIdempotent ensures replaying the same checkout.session.completed
// is a no-op.
func TestWebhookIdempotent(t *testing.T) {
	pool := testhelper.MaybePool(t)
	ctx := context.Background()

	regRepo := registration.NewRepository(pool)
	accRepo := accommodation.NewRepository(pool)

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	req := registration.SubmitRequest{
		Contact: registration.ContactDTO{
			FirstName: "Idem", LastName: "Test", Email: "i@example.com", Phone: "+44 0",
		},
		Campers: []registration.CamperDTO{{
			FirstName: "Idem", LastName: "Test", Gender: "male", Age: 25,
			CellLeaderName: "Pastor", IsMainContact: true,
			Attendance: registration.AttendanceDTO{
				Type: registration.AttendanceFullWeek, ShirtSize: "adult_m",
				AccommodationCode: "tent",
			},
		}},
	}
	gid, err := regRepo.InsertGroup(ctx, tx, req, 0, "GBP")
	if err != nil {
		t.Fatal(err)
	}
	if err := regRepo.InsertCamper(ctx, tx, gid, req.Campers[0]); err != nil {
		t.Fatal(err)
	}
	if err := regRepo.SetStripeSession(ctx, tx, gid, "sess_idem"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	refund := &fakeRefunder{}
	svc := payment.NewService(pool, regRepo, payment.NewAccommodationLocker(accRepo), refund)

	for i := 0; i < 3; i++ {
		if err := svc.HandleCheckoutCompleted(ctx, payment.CheckoutCompleted{
			SessionID: "sess_idem", PaymentIntentID: "pi_idem",
		}); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if refund.calls.Load() != 0 {
		t.Fatalf("did not expect refunds, got %d", refund.calls.Load())
	}
}

package registration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"

regapi "pagasacentre/backend/internal/api/registration"
	"pagasacentre/backend/internal/email"
	"pagasacentre/backend/internal/registration"
	"pagasacentre/backend/internal/registration/domain"
	regstorage "pagasacentre/backend/internal/registration/storage"
	"pagasacentre/backend/internal/testhelper"
)

// Handler tests need real DB writes so they're gated on TEST_DATABASE_URL via
// testhelper.MaybePool.

type fakePrices struct{}

func (fakePrices) GetPrice(_ context.Context, code string) (registration.PriceRow, error) {
	if code == domain.PriceDeposit {
		return registration.PriceRow{AmountPence: 5000, Currency: "GBP"}, nil
	}
	return registration.PriceRow{}, nil
}

type fakeCheckout struct {
	id, url     string
	calls       int
	lastDescrip string
}

func (f *fakeCheckout) CreateCheckoutSession(_ context.Context, p registration.CheckoutParams) (registration.CheckoutSession, error) {
	f.calls++
	f.lastDescrip = p.Description
	return registration.CheckoutSession{ID: f.id, URL: f.url}, nil
}

type fakeCamp struct{ open bool }

func (f fakeCamp) RegistrationsOpen(_ context.Context) (bool, error) { return f.open, nil }

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

func (m *recordingMailer) SendDepositConfirmation(_ context.Context, p email.DepositConfirmation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, p)
	return nil
}

type harness struct {
	router *chi.Mux
	stripe *fakeCheckout
	mailer *recordingMailer
}

func newHarness(t *testing.T) *harness {
	pool := testhelper.MaybePool(t)
	repo := regstorage.NewRepository(pool)
	stripe := &fakeCheckout{id: "sess_test", url: "https://checkout.stripe.com/test"}
	mailer := &recordingMailer{}
	svc := registration.NewService(repo, fakePrices{}, stripe, fakeCamp{open: true}, mailer, nil, "http://localhost:8080")
	r := chi.NewRouter()
	h := regapi.NewHandler(svc)
	r.Post("/registrations", h.Submit())
	return &harness{router: r, stripe: stripe, mailer: mailer}
}

func fullWeekBody() []byte {
	body, _ := json.Marshal(map[string]any{
		"contact": map[string]any{
			"first_name": "Jane", "last_name": "Doe",
			"email": "jane@example.com", "phone": "+44 0",
		},
		"campers": []map[string]any{{
			"first_name": "Jane", "last_name": "Doe",
			"gender": "female", "age": 30,
			"cell_leader_name": "Pastor", "is_cell_leader": false,
			"is_main_contact": true,
			"attendance": map[string]any{
				"type":                         "full_week",
				"shirt_size":                   "adult_m",
				"accommodation_first_choice":   "lodge",
				"accommodation_second_choice":  "cabin",
				"roommate_requests":            "Sharing with my friend Mary",
			},
		}},
	})
	return body
}

func dayPassOnlyBody() []byte {
	body, _ := json.Marshal(map[string]any{
		"contact": map[string]any{
			"first_name": "Sam", "last_name": "Visitor",
			"email": "sam@example.com", "phone": "+44 0",
		},
		"campers": []map[string]any{{
			"first_name": "Sam", "last_name": "Visitor",
			"gender": "male", "age": 25,
			"cell_leader_name": "Pastor", "is_cell_leader": false,
			"is_main_contact": true,
			"attendance": map[string]any{
				"type":           "day_pass",
				"days":           []string{"wed", "thu"},
				"tshirt_option":  "none",
				"needs_catering": true,
			},
		}},
	})
	return body
}

func TestPostRegistrations_BadBody(t *testing.T) {
	h := newHarness(t)
	req := httptest.NewRequest(http.MethodPost, "/registrations", bytes.NewBufferString("not json"))
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPostRegistrations_FullWeek_CreatesStripeCheckout(t *testing.T) {
	h := newHarness(t)
	req := httptest.NewRequest(http.MethodPost, "/registrations", bytes.NewReader(fullWeekBody()))
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp domain.SubmitResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.CheckoutURL != h.stripe.url {
		t.Fatalf("expected checkout url %q, got %q", h.stripe.url, resp.CheckoutURL)
	}
	if resp.TotalAmountPence != 5000 {
		t.Fatalf("expected total 5000, got %d", resp.TotalAmountPence)
	}
	if h.stripe.calls != 1 {
		t.Fatalf("expected 1 stripe call, got %d", h.stripe.calls)
	}
	// Stripe path should NOT send email at submit time (webhook does that).
	if len(h.mailer.calls) != 0 {
		t.Fatalf("expected mailer not called at submit, got %d calls", len(h.mailer.calls))
	}
}

func TestPostRegistrations_DayPassOnly_SkipsStripeAndEmailsImmediately(t *testing.T) {
	h := newHarness(t)
	req := httptest.NewRequest(http.MethodPost, "/registrations", bytes.NewReader(dayPassOnlyBody()))
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp domain.SubmitResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.CheckoutURL != "" {
		t.Fatalf("expected no checkout URL for £0 registration, got %q", resp.CheckoutURL)
	}
	if resp.TotalAmountPence != 0 {
		t.Fatalf("expected total 0, got %d", resp.TotalAmountPence)
	}
	if h.stripe.calls != 0 {
		t.Fatalf("expected 0 stripe calls for day-pass-only, got %d", h.stripe.calls)
	}
	if len(h.mailer.calls) != 1 {
		t.Fatalf("expected mailer called once at submit, got %d calls", len(h.mailer.calls))
	}
	if h.mailer.calls[0].AmountPence != 0 {
		t.Fatalf("expected £0 confirmation, got %d", h.mailer.calls[0].AmountPence)
	}
}

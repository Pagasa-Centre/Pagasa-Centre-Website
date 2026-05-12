package registration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"pagasacentre/backend/internal/accommodation"
	"pagasacentre/backend/internal/registration"
	"pagasacentre/backend/internal/testhelper"
)

// All handler tests need real DB writes so they're gated on TEST_DATABASE_URL.

type fakeAccommodations struct {
	list []accommodation.Availability
}

func (f *fakeAccommodations) ListAvailability(_ context.Context) ([]accommodation.Availability, error) {
	return f.list, nil
}

type fakePrices struct{}

func (fakePrices) GetPrice(_ context.Context, code string) (registration.PriceRow, error) {
	return registration.PriceRow{AmountPence: 5000, Currency: "GBP"}, nil
}

type fakeCheckout struct{ id, url string }

func (f *fakeCheckout) CreateCheckoutSession(_ context.Context, _ registration.CheckoutParams) (registration.CheckoutSession, error) {
	return registration.CheckoutSession{ID: f.id, URL: f.url}, nil
}

type fakeCamp struct{ open bool }

func (f fakeCamp) RegistrationsOpen(_ context.Context) (bool, error) { return f.open, nil }

func newRouter(t *testing.T, accSvc registration.AccommodationLister) (*chi.Mux, *fakeCheckout) {
	pool := testhelper.MaybePool(t)
	repo := registration.NewRepository(pool)
	stripe := &fakeCheckout{id: "sess_test", url: "https://checkout.stripe.com/test"}
	svc := registration.NewService(repo, accSvc, fakePrices{}, stripe, fakeCamp{open: true}, "http://localhost:8080")
	r := chi.NewRouter()
	registration.Mount(r, svc)
	return r, stripe
}

func validBody() []byte {
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
				"type":               "full_week",
				"shirt_size":         "adult_m",
				"accommodation_code": "lodge",
			},
		}},
	})
	return body
}

func TestPostRegistrations_BadBody(t *testing.T) {
	cap20 := 20
	r, _ := newRouter(t, &fakeAccommodations{list: []accommodation.Availability{{Code: "lodge", Capacity: &cap20}}})
	req := httptest.NewRequest(http.MethodPost, "/registrations", bytes.NewBufferString("not json"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPostRegistrations_SoldOut(t *testing.T) {
	cap1 := 1
	r, _ := newRouter(t, &fakeAccommodations{
		list: []accommodation.Availability{{Code: "lodge", Capacity: &cap1, Taken: 1}},
	})
	req := httptest.NewRequest(http.MethodPost, "/registrations", bytes.NewReader(validBody()))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", w.Code, w.Body.String())
	}
	var apiErr struct{ Code string }
	_ = json.Unmarshal(w.Body.Bytes(), &apiErr)
	if apiErr.Code != "accommodation_sold_out" {
		t.Fatalf("expected accommodation_sold_out, got %s", apiErr.Code)
	}
}

func TestPostRegistrations_HappyPath(t *testing.T) {
	cap20 := 20
	r, stripe := newRouter(t, &fakeAccommodations{
		list: []accommodation.Availability{{Code: "lodge", Capacity: &cap20, Taken: 0}},
	})
	req := httptest.NewRequest(http.MethodPost, "/registrations", bytes.NewReader(validBody()))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp registration.SubmitResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.CheckoutURL != stripe.url {
		t.Fatalf("expected checkout url %q, got %q", stripe.url, resp.CheckoutURL)
	}
	if resp.HasMinor {
		t.Fatalf("expected no minor for age 30")
	}
}

func TestPostRegistrations_HasMinor(t *testing.T) {
	cap20 := 20
	r, _ := newRouter(t, &fakeAccommodations{
		list: []accommodation.Availability{{Code: "lodge", Capacity: &cap20, Taken: 0}},
	})
	body, _ := json.Marshal(map[string]any{
		"contact": map[string]any{
			"first_name": "Anne", "last_name": "Doe",
			"email": "anne@example.com", "phone": "+44 0",
		},
		"campers": []map[string]any{{
			"first_name": "Tim", "last_name": "Doe",
			"gender": "male", "age": 12,
			"cell_leader_name": "Pastor", "is_cell_leader": false, "is_main_contact": true,
			"attendance": map[string]any{
				"type":               "full_week",
				"shirt_size":         "child_9_11y",
				"accommodation_code": "lodge",
			},
		}},
	})
	req := httptest.NewRequest(http.MethodPost, "/registrations", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp registration.SubmitResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.HasMinor || resp.ConsentFormURL == "" {
		t.Fatalf("expected has_minor + consent URL, got %+v", resp)
	}
}

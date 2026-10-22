package campadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"pagasacentre/backend/internal/registration"
	regstorage "pagasacentre/backend/internal/registration/storage"
	"pagasacentre/backend/internal/testhelper"
	commondb "pagasacentre/backend/pkg/commonlibrary/db"
)

type noopAvailability struct{}

func (noopAvailability) SetAccommodationAvailability(context.Context, string, bool) error {
	return nil
}

func availabilityRouter(setter availabilitySetter) *chi.Mux {
	r := chi.NewRouter()
	// nil recorder is fine: admin.Audit is a no-op when rec == nil.
	r.Put("/camp-admin/accommodations/{code}/availability", putAccommodationAvailability(setter, nil))
	return r
}

// TestPutAccommodationAvailability_MissingAvailable checks the nil-body guard.
// The nil-check runs before the setter is touched, so a noop fake is fine.
func TestPutAccommodationAvailability_MissingAvailable(t *testing.T) {
	r := availabilityRouter(noopAvailability{})
	req := httptest.NewRequest(http.MethodPut,
		"/camp-admin/accommodations/lodge/availability", bytes.NewBufferString("{}"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestPutAccommodationAvailability_Valid checks the happy path end-to-end
// against a real DB: 200 with echoed state, and the change is persisted.
func TestPutAccommodationAvailability_Valid(t *testing.T) {
	pool := testhelper.MaybePool(t)
	repo := regstorage.NewRepository(pool)
	regSvc := registration.NewService(repo, commondb.NewTxRunner(pool), nil, nil, nil, nil, nil, nil, "", registration.Config{})
	// accommodation_types isn't truncated between tests; restore afterwards.
	t.Cleanup(func() {
		_ = repo.SetAccommodationAvailableForRegistration(context.Background(), "lodge", true)
	})

	r := availabilityRouter(regSvc)
	req := httptest.NewRequest(http.MethodPut,
		"/camp-admin/accommodations/lodge/availability",
		bytes.NewBufferString(`{"available":false}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Code                     string `json:"code"`
		AvailableForRegistration bool   `json:"available_for_registration"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Code != "lodge" || resp.AvailableForRegistration {
		t.Fatalf("unexpected echo: %+v", resp)
	}

	avail, err := repo.AccommodationAvailability(context.Background())
	if err != nil {
		t.Fatalf("availability: %v", err)
	}
	if avail["lodge"] {
		t.Fatalf("expected lodge disabled after PUT, got %v", avail)
	}
}

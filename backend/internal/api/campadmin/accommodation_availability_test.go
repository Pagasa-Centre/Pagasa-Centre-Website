package campadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	usecase "pagasacentre/backend/internal/campadmin"
	"pagasacentre/backend/internal/registration"
	regstorage "pagasacentre/backend/internal/registration/storage"
	"pagasacentre/backend/internal/testhelper"
	commondb "pagasacentre/backend/pkg/commonlibrary/db"
)

type noopAvailability struct{}

func (noopAvailability) SetAccommodationAvailability(_ context.Context, _ string, _ string, _ bool) error {
	return nil
}

func availabilityRouter(setter availabilitySetter) *chi.Mux {
	r := chi.NewRouter()
	r.Put("/camp-admin/accommodations/{code}/availability", putAccommodationAvailability(setter))
	return r
}

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

func TestPutAccommodationAvailability_Valid(t *testing.T) {
	pool := testhelper.MaybePool(t)
	repo := regstorage.NewRepository(pool)
	regSvc := registration.NewService(repo, commondb.NewTxRunner(pool), nil, nil, nil, nil, nil, nil, "", registration.Config{})
	t.Cleanup(func() {
		_ = repo.SetAccommodationAvailableForRegistration(context.Background(), "lodge", true)
	})

	adminSvc := usecase.NewService(nil, regSvc, nil, nil, nil)
	r := availabilityRouter(adminSvc)
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

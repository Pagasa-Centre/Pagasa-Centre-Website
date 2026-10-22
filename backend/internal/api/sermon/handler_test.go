package sermon_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sermonapi "pagasacentre/backend/internal/api/sermon"
	"pagasacentre/backend/internal/sermon"
	"pagasacentre/backend/internal/sermon/domain"
)

type stubLatest struct {
	sermon *domain.Sermon
	err    error
}

func (s *stubLatest) Latest(context.Context) (*domain.Sermon, error) {
	return s.sermon, s.err
}

func TestGetLatest_NotFound(t *testing.T) {
	h := sermonapi.NewHandler(&stubLatest{err: sermon.ErrNotFound})
	req := httptest.NewRequest(http.MethodGet, "/api/sermons/latest", nil)
	w := httptest.NewRecorder()
	h.GetLatest()(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status %d", w.Code)
	}
}

func TestGetLatest_OK(t *testing.T) {
	h := sermonapi.NewHandler(&stubLatest{sermon: &domain.Sermon{VideoID: "vid1", Title: "Sunday Cell"}})
	req := httptest.NewRequest(http.MethodGet, "/api/sermons/latest", nil)
	w := httptest.NewRecorder()
	h.GetLatest()(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var body struct {
		VideoID string `json:"videoId"`
		Title   string `json:"title"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.VideoID != "vid1" || body.Title != "Sunday Cell" {
		t.Fatalf("got %+v", body)
	}
}

func TestGetLatest_Internal(t *testing.T) {
	h := sermonapi.NewHandler(&stubLatest{err: errors.New("db down")})
	req := httptest.NewRequest(http.MethodGet, "/api/sermons/latest", nil)
	w := httptest.NewRecorder()
	h.GetLatest()(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", w.Code)
	}
	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "internal_error" {
		t.Fatalf("code %q", body.Code)
	}
	if strings.Contains(body.Message, "db down") {
		t.Fatalf("message leaked internal text: %q", body.Message)
	}
}

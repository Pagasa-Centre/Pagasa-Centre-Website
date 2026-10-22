package errors_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

func TestWriteError_sanitisesInternalAPIError(t *testing.T) {
	w := httptest.NewRecorder()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(log.Writer()) })

	commonerrors.WriteError(w, commonerrors.Internal("pq: relation secret"))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, "pq") {
		t.Fatalf("body leaked internal text: %s", body)
	}
	var parsed commonerrors.APIError
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Code != "internal_error" {
		t.Fatalf("code %q", parsed.Code)
	}
	if !regexp.MustCompile(`^Something went wrong\. Please try again\. \(ref [0-9a-f]{6}\)$`).MatchString(parsed.Message) {
		t.Fatalf("message %q", parsed.Message)
	}
	if !strings.Contains(buf.String(), "ref=") || !strings.Contains(buf.String(), "pq") {
		t.Fatalf("log missing ref or cause: %q", buf.String())
	}
}

func TestWriteError_sanitisesNonAPIError(t *testing.T) {
	w := httptest.NewRecorder()
	commonerrors.WriteError(w, errors.New("db down"))

	var parsed commonerrors.APIError
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(parsed.Message, "db down") {
		t.Fatalf("message leaked: %q", parsed.Message)
	}
}

func TestWriteError_passesBadRequest(t *testing.T) {
	fields := map[string]string{"email": "invalid"}
	w := httptest.NewRecorder()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(log.Writer()) })

	commonerrors.WriteError(w, commonerrors.BadRequest("x", fields))

	var parsed commonerrors.APIError
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Message != "x" || parsed.Fields["email"] != "invalid" {
		t.Fatalf("got %+v", parsed)
	}
	if buf.Len() != 0 {
		t.Fatalf("unexpected log: %q", buf.String())
	}
}

func TestPublic_passesStaleStateAndInvalidFreeCode(t *testing.T) {
	msg := "Camp registrations are currently closed"
	pub, ref := commonerrors.Public(commonerrors.APIError{Code: "invalid_free_code", Message: msg})
	if ref != "" || pub.Message != msg {
		t.Fatalf("invalid_free_code: ref=%q msg=%q", ref, pub.Message)
	}

	staleMsg := "stale"
	pub, ref = commonerrors.Public(commonerrors.Conflict("stale_state", staleMsg, nil))
	if ref != "" || pub.Message != staleMsg {
		t.Fatalf("stale_state: ref=%q msg=%q", ref, pub.Message)
	}
}

func TestWriteError_domainKinds(t *testing.T) {
	stale := apperr.Conflict("stale_state", "updated by other")
	w := httptest.NewRecorder()
	commonerrors.WriteError(w, stale)
	if w.Code != http.StatusConflict {
		t.Fatalf("stale_state status %d", w.Code)
	}
	var parsed commonerrors.APIError
	_ = json.Unmarshal(w.Body.Bytes(), &parsed)
	if parsed.Code != "stale_state" || parsed.Message != "updated by other" {
		t.Fatalf("stale: %+v", parsed)
	}

	free := apperr.InvalidCode("invalid_free_code", "bad code")
	w = httptest.NewRecorder()
	commonerrors.WriteError(w, free)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("invalid_free_code status %d", w.Code)
	}

	val := apperr.Validation(map[string]string{"email": "x"})
	w = httptest.NewRecorder()
	commonerrors.WriteError(w, val)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("validation status %d", w.Code)
	}
	_ = json.Unmarshal(w.Body.Bytes(), &parsed)
	if parsed.Fields["email"] != "x" {
		t.Fatalf("fields %+v", parsed.Fields)
	}

	wrapped := fmt.Errorf("outer: %w", apperr.NotFound("gone"))
	w = httptest.NewRecorder()
	commonerrors.WriteError(w, wrapped)
	if w.Code != http.StatusNotFound {
		t.Fatalf("wrapped not found status %d", w.Code)
	}
}

func TestPublic_idempotentOnSanitised(t *testing.T) {
	first, ref1 := commonerrors.Public(commonerrors.Internal("secret"))
	if ref1 == "" {
		t.Fatal("expected ref on first sanitise")
	}
	second, ref2 := commonerrors.Public(first)
	if ref2 != "" {
		t.Fatalf("second pass should not re-sanitise, ref=%q", ref2)
	}
	if second.Message != first.Message {
		t.Fatalf("message changed")
	}
}

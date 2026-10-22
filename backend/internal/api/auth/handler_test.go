package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authsvc "pagasacentre/backend/internal/auth"
	"pagasacentre/backend/internal/testhelper"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
)

func fixedAuthSvc(t *testing.T) *authsvc.Service {
	t.Helper()
	return authsvc.NewService(authsvc.Config{
		Password:      "secret",
		SessionSecret: []byte("handler-test-secret"),
	}, nil, func() time.Time {
		return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	})
}

func TestHandler_login_successGolden(t *testing.T) {
	h := NewHandler(fixedAuthSvc(t), false)
	body := []byte(`{"password":"secret","first_name":"Pat","last_name":"Lee"}`)
	req := httptest.NewRequest(http.MethodPost, "/camp-admin/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	h.Login()(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	var sessCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == authsvc.SessionCookieName {
			sessCookie = c
			break
		}
	}
	if sessCookie == nil || sessCookie.Value == "" {
		t.Fatal("expected session cookie")
	}
	if sessCookie.Path != "/camp-admin" || !sessCookie.HttpOnly {
		t.Fatalf("cookie attrs: %+v", sessCookie)
	}
	testhelper.AssertGoldenJSON(t, "login_response.json", json.RawMessage(rec.Body.Bytes()))
}

func TestHandler_session_successGolden(t *testing.T) {
	svc := fixedAuthSvc(t)
	sess, err := svc.Login(t.Context(), authsvc.LoginInput{
		Password: "secret", FirstName: "Pat", LastName: "Lee", ClientKey: "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(svc, false)
	req := httptest.NewRequest(http.MethodGet, "/camp-admin/session", nil)
	req.AddCookie(&http.Cookie{Name: authsvc.SessionCookieName, Value: sess.Token})
	rec := httptest.NewRecorder()
	h.Session()(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	testhelper.AssertGoldenJSON(t, "session_response.json", json.RawMessage(rec.Body.Bytes()))
}

func TestHandler_session_unauthorized(t *testing.T) {
	h := NewHandler(fixedAuthSvc(t), false)
	req := httptest.NewRequest(http.MethodGet, "/camp-admin/session", nil)
	rec := httptest.NewRecorder()
	h.Session()(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
	var parsed commonerrors.APIError
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Code != "unauthorized" || parsed.Message != "unauthorized" {
		t.Fatalf("got %+v", parsed)
	}
}

func TestHandler_login_badRequestMissingNames(t *testing.T) {
	h := NewHandler(fixedAuthSvc(t), false)
	body := []byte(`{"password":"secret","first_name":"","last_name":"Lee"}`)
	req := httptest.NewRequest(http.MethodPost, "/camp-admin/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Login()(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var parsed commonerrors.APIError
	_ = json.Unmarshal(rec.Body.Bytes(), &parsed)
	if parsed.Code != "bad_request" {
		t.Fatalf("got %+v", parsed)
	}
}

func TestHandler_login_unauthorized(t *testing.T) {
	h := NewHandler(fixedAuthSvc(t), false)
	body := []byte(`{"password":"nope","first_name":"Pat","last_name":"Lee"}`)
	req := httptest.NewRequest(http.MethodPost, "/camp-admin/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:9999"
	rec := httptest.NewRecorder()
	h.Login()(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
	var parsed commonerrors.APIError
	_ = json.Unmarshal(rec.Body.Bytes(), &parsed)
	if parsed.Code != "unauthorized" || parsed.Message != "invalid credentials" {
		t.Fatalf("got %+v", parsed)
	}
}

func TestHandler_login_rateLimited(t *testing.T) {
	h := NewHandler(fixedAuthSvc(t), false)
	body := []byte(`{"password":"nope","first_name":"Pat","last_name":"Lee"}`)
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/camp-admin/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "10.0.0.1:1"
		rec := httptest.NewRecorder()
		h.Login()(rec, req)
	}
	req := httptest.NewRequest(http.MethodPost, "/camp-admin/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "10.0.0.1:1"
	rec := httptest.NewRecorder()
	h.Login()(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var parsed commonerrors.APIError
	_ = json.Unmarshal(rec.Body.Bytes(), &parsed)
	if parsed.Code != "too_many_requests" {
		t.Fatalf("got %+v", parsed)
	}
}

func TestHandler_logout_clearsCookie(t *testing.T) {
	h := NewHandler(fixedAuthSvc(t), true)
	req := httptest.NewRequest(http.MethodPost, "/camp-admin/logout", nil)
	rec := httptest.NewRecorder()
	h.Logout()(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d", rec.Code)
	}
	var cleared *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == authsvc.SessionCookieName {
			cleared = c
			break
		}
	}
	if cleared == nil || cleared.Value != "" || cleared.MaxAge != -1 {
		t.Fatalf("cookie: %+v", cleared)
	}
}

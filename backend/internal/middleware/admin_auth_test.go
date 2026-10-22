package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pagasacentre/backend/internal/auth"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
)

func TestRequireAdmin_unauthorizedJSON(t *testing.T) {
	svc := auth.NewService(auth.Config{
		Password: "p", SessionSecret: []byte("k"),
	}, nil)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next should not run")
	})
	req := httptest.NewRequest(http.MethodGet, "/camp-admin/x", nil)
	rec := httptest.NewRecorder()
	RequireAdmin(svc, next).ServeHTTP(rec, req)
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

func TestRequireAdmin_setsActorOnContext(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	svc := auth.NewService(auth.Config{
		Password: "secret", SessionSecret: []byte("middleware-secret"),
	}, nil, func() time.Time { return now })
	sess, err := svc.Login(t.Context(), auth.LoginInput{
		Password: "secret", FirstName: "Pat", LastName: "Lee", ClientKey: "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	var gotActor string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotActor = auth.ActorFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/camp-admin/x", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sess.Token})
	rec := httptest.NewRecorder()
	RequireAdmin(svc, next).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if gotActor != "Pat Lee" {
		t.Fatalf("actor = %q", gotActor)
	}
}

package router

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	authapi "pagasacentre/backend/internal/api/auth"
	"pagasacentre/backend/internal/auth"
)

type stubPinger struct{ err error }

func (s stubPinger) Ping(context.Context) error { return s.err }

func TestHealth(t *testing.T) {
	ok := New(Config{Commit: "abc", DB: stubPinger{}})
	down := New(Config{Commit: "abc", DB: stubPinger{err: context.DeadlineExceeded}})

	okRes := httptest.NewRecorder()
	ok.ServeHTTP(okRes, httptest.NewRequest(http.MethodGet, "/health", nil))
	if okRes.Code != http.StatusOK {
		t.Fatalf("healthy status = %d", okRes.Code)
	}

	downRes := httptest.NewRecorder()
	down.ServeHTTP(downRes, httptest.NewRequest(http.MethodGet, "/health", nil))
	if downRes.Code != http.StatusServiceUnavailable {
		body, _ := io.ReadAll(downRes.Body)
		t.Fatalf("unhealthy status = %d body %s", downRes.Code, body)
	}
}

func TestCampAdminRoutes(t *testing.T) {
	authSvc := auth.NewService(auth.Config{SessionSecret: []byte("test-secret")}, nil)
	h := New(Config{
		AuthSvc:     authSvc,
		AuthHandler: authapi.NewHandler(authSvc, false),
	})
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)

	cases := []struct {
		path       string
		wantStatus int
	}{
		{"/camp-admin/session", http.StatusUnauthorized},
		{"/admin/session", http.StatusNotFound},
		{"/camp-admin/stream", http.StatusUnauthorized},
		{"/admin/stream", http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			res, err := http.Get(ts.URL + tc.path)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != tc.wantStatus {
				t.Fatalf("GET %s: status = %d, want %d", tc.path, res.StatusCode, tc.wantStatus)
			}
		})
	}
}

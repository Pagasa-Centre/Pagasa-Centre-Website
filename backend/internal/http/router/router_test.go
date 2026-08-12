package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCampAdminRoutes(t *testing.T) {
	h := New(Config{})
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

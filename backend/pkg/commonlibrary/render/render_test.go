package render_test

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
	"pagasacentre/backend/pkg/commonlibrary/render"
)

func TestHandleServiceErrorResponse_sanitisesInternalAPIError(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(log.Writer()) })

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/x", nil)
	err := commonerrors.Internal("secret")

	render.HandleServiceErrorResponse(w, r, "op", err, func(error) (int, string) {
		return http.StatusInternalServerError, "ignored"
	})

	if strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("body leaked: %s", w.Body.String())
	}
	var parsed commonerrors.APIError
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(parsed.Message, "(ref ") {
		t.Fatalf("message %q", parsed.Message)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected one log line, got %q", buf.String())
	}
}

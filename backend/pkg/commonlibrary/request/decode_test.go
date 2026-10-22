package request_test

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
	"pagasacentre/backend/pkg/commonlibrary/request"
)

type decodeFixture struct {
	Age int `json:"age"`
}

func TestDecode_invalidJSONMessage(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(log.Writer()) })

	req := httptest.NewRequest(http.MethodPost, "/api/registrations", strings.NewReader(`{"age":"x"}`))
	var v decodeFixture
	err := request.Decode(req, &v)
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Message != "Invalid JSON body" {
		t.Fatalf("message %q", apiErr.Message)
	}
	if strings.Contains(apiErr.Message, "Go struct") {
		t.Fatal("leaked parser text in message")
	}
	if !strings.Contains(buf.String(), "cannot unmarshal") {
		t.Fatalf("log missing parser detail: %q", buf.String())
	}
}

func TestDecode_emptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/x", strings.NewReader(""))
	var v decodeFixture
	err := request.Decode(req, &v)
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) || apiErr.Message != "Request body is empty" {
		t.Fatalf("got %v", err)
	}
}

func TestDecode_bodyTooLarge(t *testing.T) {
	payload := []byte(`{"age":0,"pad":"` + strings.Repeat("x", 1<<20) + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/x", io.NopCloser(bytes.NewReader(payload)))
	var v decodeFixture
	err := request.Decode(req, &v)
	var apiErr commonerrors.APIError
	if !errors.As(err, &apiErr) || apiErr.Message != "Request body is too large" {
		t.Fatalf("got %v", err)
	}
}

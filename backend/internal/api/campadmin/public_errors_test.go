package campadmin

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
)

func TestPublicErrors_mixed(t *testing.T) {
	r := httptest.NewRequest("POST", "/camp-admin/registrations/invoice-bulk", nil)
	errs := map[string]error{
		"g1": commonerrors.BadRequest("deposit must be paid first", nil),
		"g2": commonerrors.Internal("pq: x"),
		"g3": errors.New("boom"),
	}
	out := publicErrors(r, errs)

	if !strings.HasPrefix(out["g1"], "bad_request:") {
		t.Fatalf("g1 %q", out["g1"])
	}
	if strings.Contains(out["g2"], "pq") || strings.Contains(out["g3"], "boom") {
		t.Fatalf("leaked internals: %+v", out)
	}
	if !strings.Contains(out["g2"], "(ref ") || !strings.Contains(out["g3"], "(ref ") {
		t.Fatalf("expected ref in sanitised messages: %+v", out)
	}
}

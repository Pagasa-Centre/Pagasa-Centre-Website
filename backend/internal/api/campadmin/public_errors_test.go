package campadmin

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

func TestPublicErrors_mixed(t *testing.T) {
	r := httptest.NewRequest("POST", "/camp-admin/registrations/invoice-bulk", nil)
	errs := map[string]error{
		"g1": apperr.Invalid("deposit must be paid first"),
		"g0": commonerrors.BadRequest("deposit must be paid first", nil),
		"g2": commonerrors.Internal("pq: x"),
		"g3": errors.New("boom"),
	}
	out := publicErrors(r, errs)

	if !strings.HasPrefix(out["g1"], "bad_request:") || !strings.HasPrefix(out["g0"], "bad_request:") {
		t.Fatalf("g1 %q g0 %q", out["g1"], out["g0"])
	}
	if strings.Contains(out["g2"], "pq") || strings.Contains(out["g3"], "boom") {
		t.Fatalf("leaked internals: %+v", out)
	}
	if !strings.Contains(out["g2"], "(ref ") || !strings.Contains(out["g3"], "(ref ") {
		t.Fatalf("expected ref in sanitised messages: %+v", out)
	}
}

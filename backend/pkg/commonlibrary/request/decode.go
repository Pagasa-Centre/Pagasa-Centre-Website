package request

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
)

const maxBodyBytes = 1 << 20 // 1 MiB

// Decode decodes the request body into v, with a 1 MiB cap, returning a
// user-friendly APIError on parse failures.
func Decode(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		if err == io.EOF {
			return commonerrors.BadRequest("Request body is empty", nil)
		}
		return commonerrors.BadRequest(fmt.Sprintf("Invalid JSON body: %s", err.Error()), nil)
	}
	if dec.More() {
		return commonerrors.BadRequest("Request body must contain a single JSON object", nil)
	}
	return nil
}

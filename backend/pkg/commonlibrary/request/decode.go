package request

import (
	"encoding/json"
	"errors"
	"io"
	"log"
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
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			log.Printf("%s %s: invalid JSON body: %v", r.Method, r.URL.Path, err)
			return commonerrors.BadRequest("Request body is too large", nil)
		}
		log.Printf("%s %s: invalid JSON body: %v", r.Method, r.URL.Path, err)
		return commonerrors.BadRequest("Invalid JSON body", nil)
	}
	if dec.More() {
		return commonerrors.BadRequest("Request body must contain a single JSON object", nil)
	}
	return nil
}

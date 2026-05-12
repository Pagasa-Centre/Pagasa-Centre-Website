package httpx

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const maxBodyBytes = 1 << 20 // 1 MiB

// WriteJSON serializes v as JSON with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

// DecodeJSON decodes the request body into v, with a 1 MiB cap, returning a
// user-friendly APIError on parse failures.
func DecodeJSON(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		if err == io.EOF {
			return BadRequest("Request body is empty", nil)
		}
		return BadRequest(fmt.Sprintf("Invalid JSON body: %s", err.Error()), nil)
	}
	if dec.More() {
		return BadRequest("Request body must contain a single JSON object", nil)
	}
	return nil
}

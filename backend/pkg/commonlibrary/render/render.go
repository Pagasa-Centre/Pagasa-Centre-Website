package render

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
)

// Json serializes v as JSON with the given status code.
func Json(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

// ErrorMapFn maps a service error to an HTTP status and user-facing message.
type ErrorMapFn func(err error) (status int, message string)

// HandleServiceErrorResponse logs the error and writes an APIError JSON body.
// The frozen {code, message, fields?} shape is preserved for the frontend.
func HandleServiceErrorResponse(w http.ResponseWriter, r *http.Request, op string, err error, mapFn ErrorMapFn) {
	status, msg := mapFn(err)
	if status >= http.StatusInternalServerError {
		log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
	}
	var apiErr commonerrors.APIError
	if errors.As(err, &apiErr) {
		Json(w, status, apiErr)
		return
	}
	Json(w, status, commonerrors.APIError{
		Code:    statusToCode(status),
		Message: msg,
	})
}

func statusToCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	default:
		return "internal_error"
	}
}

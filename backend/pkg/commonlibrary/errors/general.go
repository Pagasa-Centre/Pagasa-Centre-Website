package errors

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

const genericInternalMessage = "Something went wrong. Please try again."

// APIError is the canonical error type returned by handlers. Services return it
// directly; handlers translate Code to HTTP status via StatusFor.
type APIError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
	ref     string            // set only on sanitised errors; never serialised
}

func (e APIError) Error() string { return e.Code + ": " + e.Message }

func BadRequest(message string, fields map[string]string) APIError {
	return APIError{Code: "bad_request", Message: message, Fields: fields}
}

func ValidationFailed(fields map[string]string) APIError {
	return APIError{Code: "validation_failed", Message: "Validation failed", Fields: fields}
}

func NotFound(message string) APIError {
	return APIError{Code: "not_found", Message: message}
}

func Conflict(code, message string, fields map[string]string) APIError {
	return APIError{Code: code, Message: message, Fields: fields}
}

func Internal(message string) APIError {
	return APIError{Code: "internal_error", Message: message}
}

func newRef() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "000000"
	}
	return hex.EncodeToString(b)
}

// Public returns the client-safe form of err. ref is non-empty only when err
// was replaced; callers must log the original err with that ref.
func Public(err error) (pub APIError, ref string) {
	var apiErr APIError
	if errors.As(err, &apiErr) {
		if apiErr.ref != "" {
			return apiErr, ""
		}
		if apiErr.Code != "internal_error" {
			return apiErr, ""
		}
	}
	ref = newRef()
	pub = APIError{
		Code:    "internal_error",
		Message: genericInternalMessage + " (ref " + ref + ")",
		ref:     ref,
	}
	return pub, ref
}

// StatusFor maps an APIError code (or wrapped APIError) to an HTTP status.
func StatusFor(err error) int {
	var apiErr APIError
	if !errors.As(err, &apiErr) {
		return http.StatusInternalServerError
	}
	switch apiErr.Code {
	case "bad_request", "validation_failed":
		return http.StatusBadRequest
	case "not_found":
		return http.StatusNotFound
	case "accommodation_sold_out", "conflict", "registrations_closed":
		return http.StatusConflict
	case "unauthorized":
		return http.StatusUnauthorized
	case "forbidden":
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

// WriteError writes an APIError as JSON. internal_error and non-APIError values
// are sanitised to a generic message with a ref; the full cause is logged once.
func WriteError(w http.ResponseWriter, err error) {
	pub, ref := Public(err)
	if ref != "" {
		log.Printf("internal error ref=%s: %v", ref, err)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(StatusFor(pub))
	_ = json.NewEncoder(w).Encode(pub)
}

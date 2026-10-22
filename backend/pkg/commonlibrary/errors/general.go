package errors

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

const genericInternalMessage = "Something went wrong. Please try again."

// APIError is the canonical error type returned by handlers. Handlers and
// FromError produce it; services return domain errors instead.
type APIError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
	ref     string            // set only on sanitised errors; never serialised
	status  int               // HTTP status from FromError; never serialised
}

func (e APIError) Error() string { return e.Code + ": " + e.Message }

func BadRequest(message string, fields map[string]string) APIError {
	return APIError{Code: "bad_request", Message: message, Fields: fields, status: http.StatusBadRequest}
}

func ValidationFailed(fields map[string]string) APIError {
	return APIError{Code: "validation_failed", Message: "Validation failed", Fields: fields, status: http.StatusBadRequest}
}

func NotFound(message string) APIError {
	return APIError{Code: "not_found", Message: message, status: http.StatusNotFound}
}

func Conflict(code, message string, fields map[string]string) APIError {
	return APIError{Code: code, Message: message, Fields: fields, status: http.StatusConflict}
}

func Internal(message string) APIError {
	return APIError{Code: "internal_error", Message: message, status: http.StatusInternalServerError}
}

func newRef() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "000000"
	}
	return hex.EncodeToString(b)
}

// FromError converts any error into an APIError. It is the single domain-to-HTTP mapping.
func FromError(err error) APIError {
	if err == nil {
		return Internal("unknown error")
	}
	var apiErr APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}
	var de *apperr.Error
	if errors.As(err, &de) {
		return domainToAPI(de)
	}
	return Internal(err.Error())
}

func domainToAPI(de *apperr.Error) APIError {
	switch de.Kind {
	case apperr.KindInvalid:
		code := de.Code
		if code == "" {
			code = "bad_request"
		}
		return APIError{Code: code, Message: de.Message, Fields: de.Fields, status: http.StatusBadRequest}
	case apperr.KindNotFound:
		return APIError{Code: "not_found", Message: de.Message, status: http.StatusNotFound}
	case apperr.KindConflict:
		code := de.Code
		if code == "" {
			code = "conflict"
		}
		return APIError{Code: code, Message: de.Message, Fields: de.Fields, status: http.StatusConflict}
	case apperr.KindValidation:
		return APIError{
			Code:    "validation_failed",
			Message: de.Message,
			Fields:  de.Fields,
			status:  http.StatusBadRequest,
		}
	default:
		return Internal(de.Message)
	}
}

// Public returns the client-safe form of err. ref is non-empty only when err
// was replaced; callers must log the original err with that ref.
func Public(err error) (pub APIError, ref string) {
	api := FromError(err)
	if api.ref != "" {
		return api, ""
	}
	if api.Code != "internal_error" {
		return api, ""
	}
	ref = newRef()
	pub = APIError{
		Code:    "internal_error",
		Message: genericInternalMessage + " (ref " + ref + ")",
		ref:     ref,
		status:  http.StatusInternalServerError,
	}
	return pub, ref
}

// StatusFor maps an error to an HTTP status.
func StatusFor(err error) int {
	api := FromError(err)
	if api.status != 0 {
		return api.status
	}
	switch api.Code {
	case "bad_request", "validation_failed", "invalid_free_code", "missing_identifier":
		return http.StatusBadRequest
	case "not_found":
		return http.StatusNotFound
	case "accommodation_sold_out", "conflict", "registrations_closed", "stale_state", "accommodation_unavailable":
		return http.StatusConflict
	case "unauthorized":
		return http.StatusUnauthorized
	case "forbidden":
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

// WriteError writes an APIError as JSON. internal_error and non-domain errors
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

package errors

import (
	"encoding/json"
	"errors"
	"net/http"
)

// APIError is the canonical error type returned by handlers. Services return it
// directly; handlers translate Code to HTTP status via StatusFor.
type APIError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
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

// WriteError writes an APIError as JSON. If err is not an APIError, it is wrapped
// as an internal_error with its string value as the message.
func WriteError(w http.ResponseWriter, err error) {
	var apiErr APIError
	if !errors.As(err, &apiErr) {
		apiErr = Internal(err.Error())
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(StatusFor(apiErr))
	_ = json.NewEncoder(w).Encode(apiErr)
}

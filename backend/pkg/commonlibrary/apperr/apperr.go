package apperr

import "fmt"

type Kind int

const (
	KindInternal Kind = iota // zero value; never constructed directly, always sanitised
	KindInvalid              // business rule rejected the request
	KindNotFound
	KindConflict
	KindValidation   // per-field input errors
	KindUnauthorized // auth rejected
	KindRateLimited  // too many attempts
)

// Error is a transport-neutral application error returned by services.
type Error struct {
	Kind    Kind
	Code    string // optional machine code, e.g. "stale_state"; empty uses the kind default
	Message string
	Fields  map[string]string
}

func (e *Error) Error() string { return e.Message }

func Invalid(msg string) *Error {
	return &Error{Kind: KindInvalid, Message: msg}
}

func Invalidf(format string, args ...any) *Error {
	return &Error{Kind: KindInvalid, Message: fmt.Sprintf(format, args...)}
}

func InvalidCode(code, msg string) *Error {
	return &Error{Kind: KindInvalid, Code: code, Message: msg}
}

func NotFound(msg string) *Error {
	return &Error{Kind: KindNotFound, Message: msg}
}

func Conflict(code, msg string) *Error {
	return &Error{Kind: KindConflict, Code: code, Message: msg}
}

func Validation(fields map[string]string) *Error {
	return &Error{
		Kind:    KindValidation,
		Message: "Validation failed",
		Fields:  fields,
	}
}

func Unauthorized(msg string) *Error {
	return &Error{Kind: KindUnauthorized, Message: msg}
}

func RateLimited(msg string) *Error {
	return &Error{Kind: KindRateLimited, Message: msg}
}

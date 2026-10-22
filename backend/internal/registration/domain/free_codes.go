package domain

import "errors"

var (
	ErrFreeCodeInvalid      = errors.New("free code invalid")
	ErrFreeCodeNotRevocable = errors.New("free code not revocable")
)

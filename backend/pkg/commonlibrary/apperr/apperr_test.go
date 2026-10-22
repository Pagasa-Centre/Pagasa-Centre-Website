package apperr_test

import (
	"errors"
	"fmt"
	"testing"

	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

func TestConstructors(t *testing.T) {
	if e := apperr.Invalid("x"); e.Kind != apperr.KindInvalid || e.Message != "x" || e.Code != "" {
		t.Fatalf("Invalid: %+v", e)
	}
	if e := apperr.Invalidf("n=%d", 1); e.Kind != apperr.KindInvalid || e.Message != "n=1" {
		t.Fatalf("Invalidf: %+v", e)
	}
	if e := apperr.InvalidCode("c", "m"); e.Kind != apperr.KindInvalid || e.Code != "c" || e.Message != "m" {
		t.Fatalf("InvalidCode: %+v", e)
	}
	if e := apperr.NotFound("gone"); e.Kind != apperr.KindNotFound || e.Message != "gone" {
		t.Fatalf("NotFound: %+v", e)
	}
	if e := apperr.Conflict("stale_state", "stale"); e.Kind != apperr.KindConflict || e.Code != "stale_state" {
		t.Fatalf("Conflict: %+v", e)
	}
	if e := apperr.Validation(map[string]string{"a": "b"}); e.Kind != apperr.KindValidation || e.Fields["a"] != "b" {
		t.Fatalf("Validation: %+v", e)
	}
}

func TestErrorsIsThroughWrap(t *testing.T) {
	sent := apperr.NotFound("group not found")
	wrapped := fmt.Errorf("step: %w", sent)
	if !errors.Is(wrapped, sent) {
		t.Fatal("errors.Is should find sentinel through wrap")
	}
	var de *apperr.Error
	if !errors.As(wrapped, &de) || de != sent {
		t.Fatalf("errors.As should unwrap to sentinel, got %v", de)
	}
}

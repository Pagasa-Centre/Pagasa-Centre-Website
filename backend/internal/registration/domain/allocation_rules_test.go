package domain

import (
	"errors"
	"testing"

	"pagasacentre/backend/pkg/commonlibrary/apperr"
)

func TestAllocationTarget_unknownCamper(t *testing.T) {
	_, _, err := AllocationTarget(nil, "missing", "tent")
	assertInvalidMsg(t, err, "unknown camper_id missing")
}

func TestAllocationTarget_blankCode(t *testing.T) {
	campers := []Camper{{ID: "c1", AttendanceType: AttendanceFullWeek}}
	_, _, err := AllocationTarget(campers, "c1", " ")
	var de *apperr.Error
	if !errors.As(err, &de) || de.Kind != apperr.KindValidation {
		t.Fatalf("want validation, got %v", err)
	}
	if de.Fields["c1"] != "allocated_accommodation_code is required for full-week campers" {
		t.Fatalf("fields %+v", de.Fields)
	}
}

func TestCheckAllCampersAllocated(t *testing.T) {
	campers := []Camper{
		{ID: "c1", AttendanceType: AttendanceFullWeek, FirstName: "A", LastName: "B"},
	}
	err := CheckAllCampersAllocated(campers, nil)
	var ve *apperr.Error
	if !errors.As(err, &ve) || ve.Kind != apperr.KindValidation {
		t.Fatalf("want validation, got %v", err)
	}
	if ve.Fields["campers"] != "full-week camper A B is not allocated" {
		t.Fatalf("fields %+v", ve.Fields)
	}
}

package campadmin

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"pagasacentre/backend/internal/adminlog"
	admindto "pagasacentre/backend/internal/api/campadmin/dto"
	"pagasacentre/backend/internal/api/campadmin/dto/mapper"
	regmapper "pagasacentre/backend/internal/api/registration/dto/mapper"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/testhelper"
	"pagasacentre/backend/pkg/commonlibrary/request"
)

func TestGolden_registrationsList_full(t *testing.T) {
	g := testhelper.SampleGroupFull()
	c := testhelper.SampleCamperFull()
	testhelper.AssertGoldenJSON(t, "registrations_list_full.json", map[string]any{
		"groups": []admindto.GroupResponse{mapper.GroupToResponse(g, []domain.Camper{c})},
	})
}

func TestGolden_registrationsList_emptyGroups(t *testing.T) {
	testhelper.AssertGoldenJSON(t, "registrations_list_empty.json", map[string]any{"groups": []admindto.GroupResponse{}})
}

func TestGolden_registrationsList_nilCampers(t *testing.T) {
	g := testhelper.SampleGroupFull()
	testhelper.AssertGoldenJSON(t, "registrations_list_nil_campers.json", map[string]any{
		"groups": []admindto.GroupResponse{mapper.GroupToResponse(g, nil)},
	})
}

func TestGolden_campConfig(t *testing.T) {
	testhelper.AssertGoldenJSON(t, "camp_config.json", mapper.CampConfigToResponse(testhelper.SampleCampConfig()))
}

func TestGolden_accommodationTypes_full(t *testing.T) {
	testhelper.AssertGoldenJSON(t, "accommodation_types_full.json", map[string]any{
		"accommodations": mapper.AccommodationTypesToResponse([]domain.AccommodationType{testhelper.SampleAccommodationType()}),
	})
}

func TestGolden_accommodationTypes_nil(t *testing.T) {
	testhelper.AssertGoldenJSON(t, "accommodation_types_nil.json", map[string]any{"accommodations": mapper.AccommodationTypesToResponse(nil)})
}

func TestGolden_accommodationUnits_full(t *testing.T) {
	testhelper.AssertGoldenJSON(t, "accommodation_units_full.json", map[string]any{
		"units": mapper.AccommodationUnitsToResponse([]domain.AccommodationUnit{testhelper.SampleAccommodationUnit()}),
	})
}

func TestGolden_accommodationUnits_nil(t *testing.T) {
	testhelper.AssertGoldenJSON(t, "accommodation_units_nil.json", map[string]any{"units": mapper.AccommodationUnitsToResponse(nil)})
}

func TestGolden_freeCodes_full(t *testing.T) {
	testhelper.AssertGoldenJSON(t, "free_codes_full.json", map[string]any{
		"codes": mapper.FreeCodesToResponse([]domain.FreeCode{testhelper.SampleFreeCode()}),
	})
}

func TestGolden_freeCodes_empty(t *testing.T) {
	testhelper.AssertGoldenJSON(t, "free_codes_empty.json", map[string]any{"codes": mapper.FreeCodesToResponse(nil)})
}

func TestGolden_events_full(t *testing.T) {
	ev := sampleAdminEvent()
	testhelper.AssertGoldenJSON(t, "events_full.json", map[string]any{"events": mapper.EventsToResponse([]adminlog.Event{ev})})
}

func TestGolden_events_empty(t *testing.T) {
	testhelper.AssertGoldenJSON(t, "events_empty.json", map[string]any{"events": mapper.EventsToResponse(nil)})
}

func TestGolden_editCamperResponse(t *testing.T) {
	testhelper.AssertGoldenJSON(t, "edit_camper_response.json", mapper.EditCamperToResponse(sampleEditCamperSummary()))
}

func TestGolden_addCamperResponse(t *testing.T) {
	testhelper.AssertGoldenJSON(t, "add_camper_response.json", mapper.AddCamperToResponse(sampleAddCamperSummary()))
}

func TestGolden_sseEvent(t *testing.T) {
	testhelper.AssertGoldenMarshal(t, "sse_event.json", mapper.EventToResponse(sampleAdminEvent()))
}

func decodeRequestGolden[T any](t *testing.T, name string, want T, decodeInto *T) {
	t.Helper()
	testhelper.AssertGoldenMarshal(t, name, want)
	body, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if err := request.Decode(r, decodeInto); err != nil {
		t.Fatal(err)
	}
}

func TestGolden_versionedBody_request(t *testing.T) {
	v := 2
	body := admindto.VersionedBody{ExpectedVersion: testhelper.IntP(v)}
	var got admindto.VersionedBody
	decodeRequestGolden(t, "request_versioned_body.json", body, &got)
	if got.ExpectedVersion == nil || *got.ExpectedVersion != v {
		t.Fatalf("got %+v", got)
	}
}

func TestGolden_bulkInvoice_request(t *testing.T) {
	body := admindto.BulkInvoiceRequest{GroupIDs: []string{"g1", "g2"}}
	var got admindto.BulkInvoiceRequest
	decodeRequestGolden(t, "request_bulk_invoice.json", body, &got)
}

func TestGolden_allocate_request(t *testing.T) {
	v := 1
	body := admindto.AllocateRequest{
		Campers: []admindto.AllocateCamperRequest{{
			CamperID:                   "c1",
			AllocatedAccommodationCode: "tent",
			AllocatedUnitCode:          "tent-1",
			BilledPriceRef:             "price_x",
		}},
		ExpectedVersion: testhelper.IntP(v),
	}
	var got admindto.AllocateRequest
	decodeRequestGolden(t, "request_allocate.json", body, &got)
}

func TestGolden_extendDue_request(t *testing.T) {
	v := 3
	body := admindto.ExtendDueRequest{DueAt: "2026-08-01T00:00:00Z", ExpectedVersion: testhelper.IntP(v)}
	var got admindto.ExtendDueRequest
	decodeRequestGolden(t, "request_extend_due.json", body, &got)
}

func TestGolden_editCamper_request(t *testing.T) {
	v := 1
	body := admindto.EditCamperRequest{
		ExpectedVersion:            testhelper.IntP(v),
		FirstName:                  "Alex",
		LastName:                   "Camper",
		Gender:                     domain.GenderMale,
		Age:                        25,
		CellLeaderName:             "Leader",
		IsCellLeader:               false,
		ShirtSize:                  "adult_m",
		DietaryRequirements:        "none",
		AccommodationFirstChoice:   "tent",
		AccommodationSecondChoice:  "caravan",
		AllocatedAccommodationCode: "tent",
		AllocatedUnitCode:          "tent-1",
	}
	var got admindto.EditCamperRequest
	decodeRequestGolden(t, "request_edit_camper.json", body, &got)
}

func TestGolden_addCamper_request(t *testing.T) {
	v := 1
	body := admindto.AddCamperRequest{
		ExpectedVersion: testhelper.IntP(v),
		Camper:          regmapper.CamperToRequest(testhelper.SampleSubmitRequest().Campers[0]),
	}
	var got admindto.AddCamperRequest
	decodeRequestGolden(t, "request_add_camper.json", body, &got)
}

func TestGolden_convertDayVisitor_request(t *testing.T) {
	body := admindto.ConvertToDayVisitorRequest{
		ExpectedVersion: testhelper.IntP(1),
		Days:            []string{"mon", "tue"},
		TshirtOption:    domain.TshirtOptionTshirtOnly,
		ShirtSize:       "adult_m",
		NeedsCatering:   testhelper.BoolP(true),
	}
	var got admindto.ConvertToDayVisitorRequest
	decodeRequestGolden(t, "request_convert_day_visitor.json", body, &got)
}

func TestGolden_updateDayPass_request(t *testing.T) {
	body := admindto.UpdateDayPassCamperRequest{
		ExpectedVersion: testhelper.IntP(1),
		TshirtOption:    domain.TshirtOptionTeamActivities,
		ShirtSize:       "adult_m",
		NeedsCatering:   testhelper.BoolP(false),
		Dietary:         testhelper.StrP("vegan"),
	}
	var got admindto.UpdateDayPassCamperRequest
	decodeRequestGolden(t, "request_update_day_pass.json", body, &got)
}

func TestGolden_updateCoach_request(t *testing.T) {
	body := admindto.UpdateCamperCoachRequest{ExpectedVersion: testhelper.IntP(1), NeedsCoach: true}
	var got admindto.UpdateCamperCoachRequest
	decodeRequestGolden(t, "request_update_coach.json", body, &got)
}

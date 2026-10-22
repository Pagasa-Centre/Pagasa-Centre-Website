package mapper

import (
	"testing"

	"pagasacentre/backend/internal/adminlog"
	"pagasacentre/backend/internal/billing/camperedit"
	"pagasacentre/backend/internal/registration/domain"
	"pagasacentre/backend/internal/testhelper"
)

func TestGolden_groupResponse_full(t *testing.T) {
	g := testhelper.SampleGroupFull()
	c := testhelper.SampleCamperFull()
	testhelper.AssertGoldenJSON(t, "group_response_full.json", GroupToResponse(g, []domain.Camper{c}))
}

func TestGolden_groupResponse_nilCampers(t *testing.T) {
	g := testhelper.SampleGroupFull()
	testhelper.AssertGoldenJSON(t, "group_response_nil_campers.json", GroupToResponse(g, nil))
}

func TestGolden_editCamperResponse(t *testing.T) {
	sum := camperedit.EditCamperSummary{
		CamperName: "Alex Camper", PreviousName: "Old Name", InvoiceVoided: true, Repriced: true,
	}
	testhelper.AssertGoldenJSON(t, "edit_camper_response.json", EditCamperToResponse(sum))
}

func TestGolden_addCamperResponse(t *testing.T) {
	sum := camperedit.AddCamperSummary{
		CamperID: "cmp-new", CamperName: "New Camper", DepositOwedPence: 5000,
		InvoiceVoided: true, NeedsAllocation: true,
	}
	testhelper.AssertGoldenJSON(t, "add_camper_response.json", AddCamperToResponse(sum))
}

func TestGolden_eventResponse(t *testing.T) {
	ev := adminlog.Event{
		ID: 42, CreatedAt: testhelper.FixedTime, ActorName: "Pat Admin",
		Action: adminlog.ActionCamperAdded, GroupID: testhelper.StrP("grp-1"),
		Summary: "Added Alex Camper to the booking", Metadata: []byte(`{"camper_id":"cmp-1"}`),
	}
	testhelper.AssertGoldenMarshal(t, "sse_event.json", EventToResponse(ev))
}

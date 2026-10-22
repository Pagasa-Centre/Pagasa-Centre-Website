package campadmin

import (
	"pagasacentre/backend/internal/adminlog"
	"pagasacentre/backend/internal/billing/camperedit"
	"pagasacentre/backend/internal/testhelper"
)

func sampleAdminEvent() adminlog.Event {
	return adminlog.Event{
		ID:        42,
		CreatedAt: testhelper.FixedTime,
		ActorName: "Pat Admin",
		Action:    adminlog.ActionCamperAdded,
		GroupID:   testhelper.StrP("grp-1"),
		Summary:   "Added Alex Camper to the booking",
		Metadata:  []byte(`{"camper_id":"cmp-1"}`),
	}
}

func sampleEditCamperSummary() camperedit.EditCamperSummary {
	return camperedit.EditCamperSummary{
		CamperName: "Alex Camper", PreviousName: "Old Name", InvoiceVoided: true, Repriced: true,
	}
}

func sampleAddCamperSummary() camperedit.AddCamperSummary {
	return camperedit.AddCamperSummary{
		CamperID: "cmp-new", CamperName: "New Camper", DepositOwedPence: 5000,
		InvoiceVoided: true, NeedsAllocation: true,
	}
}

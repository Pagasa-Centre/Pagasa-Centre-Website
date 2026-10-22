package campadmin

import (
	"pagasacentre/backend/internal/billing/camperedit"
	"pagasacentre/backend/internal/billing/invoicing"
)

func sampleRemoveCamperSummary() camperedit.RemoveCamperSummary {
	return camperedit.RemoveCamperSummary{CamperName: "Alex Camper", InvoiceVoided: true}
}

func sampleConvertSummary() camperedit.ConvertSummary {
	return camperedit.ConvertSummary{CamperName: "Alex Camper", InvoiceVoided: true, DepositCreditPence: 500}
}

func sampleUpdateDayPassSummary() camperedit.UpdateDayPassSummary {
	return camperedit.UpdateDayPassSummary{CamperName: "Alex Camper"}
}

func sampleUpdateCamperCoachSummary() camperedit.UpdateCamperCoachSummary {
	return camperedit.UpdateCamperCoachSummary{
		CamperName: "Alex Camper", NeedsCoach: true, InvoiceVoided: true,
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

func sampleDeleteSummary() invoicing.DeleteSummary {
	return invoicing.DeleteSummary{
		ContactName: "Jane Doe", ContactEmail: "jane@example.com", BalanceRefunded: true,
		InvoiceVoided: true, AmountPence: 8000, RetainedPence: 2000,
	}
}

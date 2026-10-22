package campadmin

import (
	"pagasacentre/backend/internal/billing/camperedit"
	"pagasacentre/backend/internal/billing/invoicing"
)

type camperRemovedMeta struct {
	CamperName    string `json:"CamperName"`
	InvoiceVoided bool   `json:"InvoiceVoided"`
}

type camperConvertedMeta struct {
	CamperName         string `json:"CamperName"`
	InvoiceVoided      bool   `json:"InvoiceVoided"`
	DepositCreditPence int    `json:"DepositCreditPence"`
}

type dayPassUpdatedMeta struct {
	CamperName string `json:"CamperName"`
}

type camperCoachMeta struct {
	CamperName    string `json:"camper_name"`
	NeedsCoach    bool   `json:"needs_coach"`
	InvoiceVoided bool   `json:"invoice_voided,omitempty"`
}

type camperEditedMeta struct {
	CamperName    string `json:"camper_name"`
	PreviousName  string `json:"previous_name"`
	InvoiceVoided bool   `json:"invoice_voided"`
	Repriced      bool   `json:"repriced"`
}

type camperAddedMeta struct {
	CamperID         string `json:"camper_id"`
	CamperName       string `json:"camper_name"`
	DepositOwedPence int    `json:"deposit_owed_pence"`
	InvoiceVoided    bool   `json:"invoice_voided"`
	NeedsAllocation  bool   `json:"needs_allocation"`
}

type registrationDeletedMeta struct {
	ContactName     string `json:"ContactName"`
	ContactEmail    string `json:"ContactEmail"`
	BalanceRefunded bool   `json:"BalanceRefunded"`
	InvoiceVoided   bool   `json:"InvoiceVoided"`
	AmountPence     int64  `json:"AmountPence"`
	RetainedPence   int64  `json:"RetainedPence"`
}

func metaRemoveCamper(s camperedit.RemoveCamperSummary) camperRemovedMeta {
	return camperRemovedMeta{CamperName: s.CamperName, InvoiceVoided: s.InvoiceVoided}
}

func metaConvertCamper(s camperedit.ConvertSummary) camperConvertedMeta {
	return camperConvertedMeta{
		CamperName: s.CamperName, InvoiceVoided: s.InvoiceVoided, DepositCreditPence: s.DepositCreditPence,
	}
}

func metaDayPassUpdated(s camperedit.UpdateDayPassSummary) dayPassUpdatedMeta {
	return dayPassUpdatedMeta{CamperName: s.CamperName}
}

func metaCamperCoach(s camperedit.UpdateCamperCoachSummary) camperCoachMeta {
	return camperCoachMeta{CamperName: s.CamperName, NeedsCoach: s.NeedsCoach, InvoiceVoided: s.InvoiceVoided}
}

func metaCamperEdited(s camperedit.EditCamperSummary) camperEditedMeta {
	return camperEditedMeta{
		CamperName: s.CamperName, PreviousName: s.PreviousName,
		InvoiceVoided: s.InvoiceVoided, Repriced: s.Repriced,
	}
}

func metaCamperAdded(s camperedit.AddCamperSummary) camperAddedMeta {
	return camperAddedMeta{
		CamperID: s.CamperID, CamperName: s.CamperName, DepositOwedPence: s.DepositOwedPence,
		InvoiceVoided: s.InvoiceVoided, NeedsAllocation: s.NeedsAllocation,
	}
}

func metaRegistrationDeleted(s invoicing.DeleteSummary) registrationDeletedMeta {
	return registrationDeletedMeta{
		ContactName: s.ContactName, ContactEmail: s.ContactEmail, BalanceRefunded: s.BalanceRefunded,
		InvoiceVoided: s.InvoiceVoided, AmountPence: s.AmountPence, RetainedPence: s.RetainedPence,
	}
}

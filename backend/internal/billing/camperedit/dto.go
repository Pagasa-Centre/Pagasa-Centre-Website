package camperedit

import "pagasacentre/backend/internal/registration/domain"

type EditCamperRequest struct {
	FirstName      string
	LastName       string
	Gender         string
	Age            int
	CellLeaderName string
	IsCellLeader   bool

	ShirtSize                  string
	DietaryRequirements        string
	AccommodationFirstChoice   string
	AccommodationSecondChoice  string
	AllocatedAccommodationCode string
	AllocatedUnitCode          string
}

type AddCamperRequest struct {
	Camper domain.NewCamper
}

type EditCamperSummary struct {
	CamperName    string
	PreviousName  string
	InvoiceVoided bool
	Repriced      bool
}

type AddCamperSummary struct {
	CamperID         string
	CamperName       string
	DepositOwedPence int
	InvoiceVoided    bool
	NeedsAllocation  bool
}

type RemoveCamperSummary struct {
	CamperName    string
	InvoiceVoided bool
}

type ConvertToDayVisitorRequest struct {
	Days          []string
	TshirtOption  string
	ShirtSize     string
	NeedsCatering *bool
}

type ConvertSummary struct {
	CamperName         string
	InvoiceVoided      bool
	DepositCreditPence int
}

type UpdateDayPassCamperRequest struct {
	TshirtOption  string
	ShirtSize     string
	NeedsCatering *bool
	Dietary       *string
}

type UpdateDayPassSummary struct {
	CamperName string
}

type UpdateCamperCoachRequest struct {
	NeedsCoach bool
}

type UpdateCamperCoachSummary struct {
	CamperName    string
	NeedsCoach    bool
	InvoiceVoided bool
	NoOp          bool
}

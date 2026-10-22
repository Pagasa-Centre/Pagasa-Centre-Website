package camperedit

import "pagasacentre/backend/internal/registration/domain"

type EditCamperRequest struct {
	ExpectedVersion *int `json:"expected_version,omitempty"`

	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	Gender         string `json:"gender"`
	Age            int    `json:"age"`
	CellLeaderName string `json:"cell_leader_name"`
	IsCellLeader   bool   `json:"is_cell_leader"`

	ShirtSize                  string `json:"shirt_size"`
	DietaryRequirements        string `json:"dietary_requirements"`
	AccommodationFirstChoice   string `json:"accommodation_first_choice"`
	AccommodationSecondChoice  string `json:"accommodation_second_choice"`
	AllocatedAccommodationCode string `json:"allocated_accommodation_code"`
	AllocatedUnitCode          string `json:"allocated_unit_code"`
}

type AddCamperRequest struct {
	ExpectedVersion *int             `json:"expected_version,omitempty"`
	Camper          domain.CamperDTO `json:"camper"`
}

type EditCamperSummary struct {
	CamperName    string `json:"camper_name"`
	PreviousName  string `json:"previous_name"`
	InvoiceVoided bool   `json:"invoice_voided"`
	Repriced      bool   `json:"repriced"`
}

type AddCamperSummary struct {
	CamperID         string `json:"camper_id"`
	CamperName       string `json:"camper_name"`
	DepositOwedPence int    `json:"deposit_owed_pence"`
	InvoiceVoided    bool   `json:"invoice_voided"`
	NeedsAllocation  bool   `json:"needs_allocation"`
}

type RemoveCamperSummary struct {
	CamperName    string
	InvoiceVoided bool
}

type ConvertToDayVisitorRequest struct {
	ExpectedVersion *int     `json:"expected_version,omitempty"`
	Days            []string `json:"days"`
	TshirtOption    string   `json:"tshirt_option"`
	ShirtSize       string   `json:"shirt_size"`
	NeedsCatering   *bool    `json:"needs_catering"`
}

type ConvertSummary struct {
	CamperName         string
	InvoiceVoided      bool
	DepositCreditPence int
}

type UpdateDayPassCamperRequest struct {
	ExpectedVersion *int    `json:"expected_version,omitempty"`
	TshirtOption    string  `json:"tshirt_option"`
	ShirtSize       string  `json:"shirt_size"`
	NeedsCatering   *bool   `json:"needs_catering"`
	Dietary         *string `json:"dietary_requirements"`
}

type UpdateDayPassSummary struct {
	CamperName string
}

type UpdateCamperCoachRequest struct {
	ExpectedVersion *int `json:"expected_version,omitempty"`
	NeedsCoach      bool `json:"needs_coach"`
}

type UpdateCamperCoachSummary struct {
	CamperName    string `json:"camper_name"`
	NeedsCoach    bool   `json:"needs_coach"`
	InvoiceVoided bool   `json:"invoice_voided,omitempty"`
	NoOp          bool   `json:"-"`
}

package dto

import regdto "pagasacentre/backend/internal/api/registration/dto"

type VersionedBody struct {
	ExpectedVersion *int `json:"expected_version,omitempty"`
}

type BulkInvoiceRequest struct {
	GroupIDs []string `json:"group_ids"`
}

type AllocateCamperRequest struct {
	CamperID                   string `json:"camper_id"`
	AllocatedAccommodationCode string `json:"allocated_accommodation_code"`
	AllocatedUnitCode          string `json:"allocated_unit_code,omitempty"`
	BilledStripePriceID        string `json:"billed_stripe_price_id,omitempty"`
}

type AllocateRequest struct {
	Campers         []AllocateCamperRequest `json:"campers"`
	ExpectedVersion *int                    `json:"expected_version,omitempty"`
}

type ExtendDueRequest struct {
	DueAt           string `json:"due_at"`
	ExpectedVersion *int   `json:"expected_version,omitempty"`
}

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
	ExpectedVersion *int                 `json:"expected_version,omitempty"`
	Camper          regdto.CamperRequest `json:"camper"`
}

type ConvertToDayVisitorRequest struct {
	ExpectedVersion *int     `json:"expected_version,omitempty"`
	Days            []string `json:"days"`
	TshirtOption    string   `json:"tshirt_option"`
	ShirtSize       string   `json:"shirt_size"`
	NeedsCatering   *bool    `json:"needs_catering"`
}

type UpdateDayPassCamperRequest struct {
	ExpectedVersion *int    `json:"expected_version,omitempty"`
	TshirtOption    string  `json:"tshirt_option"`
	ShirtSize       string  `json:"shirt_size"`
	NeedsCatering   *bool   `json:"needs_catering"`
	Dietary         *string `json:"dietary_requirements"`
}

type UpdateCamperCoachRequest struct {
	ExpectedVersion *int `json:"expected_version,omitempty"`
	NeedsCoach      bool `json:"needs_coach"`
}

package allocation

// AllocateCamper is one camper placement from the admin dashboard.
type AllocateCamper struct {
	CamperID                   string `json:"camper_id"`
	AllocatedAccommodationCode string `json:"allocated_accommodation_code"`
	AllocatedUnitCode          string `json:"allocated_unit_code,omitempty"`
	BilledStripePriceID        string `json:"billed_stripe_price_id,omitempty"`
}

// AllocateRequest is PUT /camp-admin/registrations/{groupID}/allocation.
type AllocateRequest struct {
	Campers         []AllocateCamper `json:"campers"`
	ExpectedVersion *int             `json:"expected_version,omitempty"`
}

package registration

type SubmitRequest struct {
	Contact ContactDTO  `json:"contact"`
	Campers []CamperDTO `json:"campers"`
}

type ContactDTO struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
}

type CamperDTO struct {
	FirstName      string        `json:"first_name"`
	LastName       string        `json:"last_name"`
	Gender         string        `json:"gender"`
	Age            int           `json:"age"`
	CellLeaderName string        `json:"cell_leader_name"`
	IsCellLeader   bool          `json:"is_cell_leader"`
	IsMainContact  bool          `json:"is_main_contact"`
	Attendance     AttendanceDTO `json:"attendance"`
}

type AttendanceDTO struct {
	Type string `json:"type"` // full_week | day_pass

	// Full-week fields
	ShirtSize                 string `json:"shirt_size,omitempty"`
	DietaryRequirements       string `json:"dietary_requirements,omitempty"`
	NeedsCoach                *bool  `json:"needs_coach,omitempty"`
	AccommodationFirstChoice  string `json:"accommodation_first_choice,omitempty"`
	AccommodationSecondChoice string `json:"accommodation_second_choice,omitempty"`
	RoommateRequests          string `json:"roommate_requests,omitempty"`

	// Day-pass fields
	Days          []string `json:"days,omitempty"`
	TshirtOption  string   `json:"tshirt_option,omitempty"`
	NeedsCatering *bool    `json:"needs_catering,omitempty"`
}

type SubmitResponse struct {
	GroupID          string `json:"group_id"`
	CheckoutURL      string `json:"checkout_url"`
	TotalAmountPence int    `json:"total_amount_pence"`
	HasMinor         bool   `json:"has_minor"`
	ConsentFormURL   string `json:"consent_form_url,omitempty"`
}

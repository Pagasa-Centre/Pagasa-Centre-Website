package dto

type SubmitRequest struct {
	Contact  ContactRequest  `json:"contact"`
	Campers  []CamperRequest `json:"campers"`
	FreeCode string          `json:"free_code,omitempty"`
}

type ContactRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
}

type CamperRequest struct {
	FirstName      string            `json:"first_name"`
	LastName       string            `json:"last_name"`
	Gender         string            `json:"gender"`
	Age            int               `json:"age"`
	CellLeaderName string            `json:"cell_leader_name"`
	IsCellLeader   bool              `json:"is_cell_leader"`
	IsMainContact  bool              `json:"is_main_contact"`
	Attendance     AttendanceRequest `json:"attendance"`
}

type AttendanceRequest struct {
	Type string `json:"type"`

	ShirtSize                 string `json:"shirt_size,omitempty"`
	DietaryRequirements       string `json:"dietary_requirements,omitempty"`
	NeedsCoach                *bool  `json:"needs_coach,omitempty"`
	AccommodationFirstChoice  string `json:"accommodation_first_choice,omitempty"`
	AccommodationSecondChoice string `json:"accommodation_second_choice,omitempty"`
	RoommateRequests          string `json:"roommate_requests,omitempty"`

	Days          []string `json:"days,omitempty"`
	TshirtOption  string   `json:"tshirt_option,omitempty"`
	NeedsCatering *bool    `json:"needs_catering,omitempty"`
}

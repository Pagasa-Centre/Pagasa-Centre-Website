package dto

type OptionResponse struct {
	Code                     string `json:"code"`
	DisplayName              string `json:"display_name"`
	Notes                    string `json:"notes,omitempty"`
	AvailableForRegistration bool   `json:"available_for_registration"`
}

type OptionsResponse struct {
	Accommodations []OptionResponse `json:"accommodations"`
}

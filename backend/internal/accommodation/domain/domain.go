package domain

type Type struct {
	Code        string
	DisplayName string
	SortOrder   int
	Notes       string
}

type Option struct {
	Code                     string `json:"code"`
	DisplayName              string `json:"display_name"`
	Notes                    string `json:"notes,omitempty"`
	AvailableForRegistration bool   `json:"available_for_registration"`
}

package domain

type Type struct {
	Code        string
	DisplayName string
	SortOrder   int
	Notes       string
}

type Option struct {
	Code                     string
	DisplayName              string
	Notes                    string
	AvailableForRegistration bool
}

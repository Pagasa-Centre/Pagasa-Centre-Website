package accommodation

type Type struct {
	Code        string
	DisplayName string
	Capacity    *int
	SortOrder   int
	Notes       string
}

type Availability struct {
	Code        string `json:"code"`
	DisplayName string `json:"display_name"`
	Capacity    *int   `json:"capacity"`
	Taken       int    `json:"taken"`
	Remaining   *int   `json:"remaining"`
	Notes       string `json:"notes,omitempty"`
}

// IsUnlimited returns true if this accommodation has no capacity cap (e.g. tent).
func (a Availability) IsUnlimited() bool { return a.Capacity == nil }

// HasRoomFor reports whether n more campers can be accommodated.
func (a Availability) HasRoomFor(n int) bool {
	if a.IsUnlimited() {
		return true
	}
	return a.Taken+n <= *a.Capacity
}

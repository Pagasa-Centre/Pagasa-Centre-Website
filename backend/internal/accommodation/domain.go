package accommodation

// Type is the canonical record for an accommodation option, mirroring a row
// in the accommodation_types table.
type Type struct {
	Code        string
	DisplayName string
	SortOrder   int
	Notes       string
}

// Option is what we expose to the public API. As of v2 it's identical to Type
// minus SortOrder (which only matters for ordering, not the consumer). Capacity
// and availability tracking were dropped — the committee allocates placements
// manually after registrations close.
type Option struct {
	Code        string `json:"code"`
	DisplayName string `json:"display_name"`
	Notes       string `json:"notes,omitempty"`
}

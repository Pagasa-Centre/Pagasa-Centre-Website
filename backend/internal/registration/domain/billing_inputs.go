package domain

// CamperAllocation is one camper's White Team placement.
type CamperAllocation struct {
	CamperID                   string
	AllocatedAccommodationCode string
	AllocatedUnitCode          string
	BilledPriceRef             string
}

// CamperDetails is the full set of identity and stay fields an admin can rewrite
// on an existing camper.
type CamperDetails struct {
	FirstName      string
	LastName       string
	Gender         string
	Age            int
	CellLeaderName string
	IsCellLeader   bool

	ShirtSize                  *string
	DietaryRequirements        *string
	AccommodationFirstChoice   *string
	AccommodationSecondChoice  *string
	AllocatedAccommodationCode *string
	AllocatedUnitCode          *string
	BilledPriceRef             *string
}

// DayPassFields holds day-visitor details applied during a full-week conversion.
type DayPassFields struct {
	Days          []string
	TshirtOption  string
	ShirtSize     *string
	NeedsCatering bool
	Dietary       *string
}

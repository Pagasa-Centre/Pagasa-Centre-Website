package allocation

type AllocateCamper struct {
	CamperID                   string
	AllocatedAccommodationCode string
	AllocatedUnitCode          string
	BilledStripePriceID        string
}

type AllocateRequest struct {
	Campers []AllocateCamper
}

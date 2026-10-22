package allocation

type AllocateCamper struct {
	CamperID                   string
	AllocatedAccommodationCode string
	AllocatedUnitCode          string
	BilledPriceRef             string
}

type AllocateRequest struct {
	Campers []AllocateCamper
}

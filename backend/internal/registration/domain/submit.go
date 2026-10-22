package domain

type Submission struct {
	Contact  Contact
	Campers  []NewCamper
	FreeCode string
}

type Contact struct {
	FirstName string
	LastName  string
	Email     string
	Phone     string
}

type NewCamper struct {
	FirstName      string
	LastName       string
	Gender         string
	Age            int
	CellLeaderName string
	IsCellLeader   bool
	IsMainContact  bool
	Attendance     Attendance
}

type Attendance struct {
	Type string

	ShirtSize                 string
	DietaryRequirements       string
	NeedsCoach                *bool
	AccommodationFirstChoice  string
	AccommodationSecondChoice string
	RoommateRequests          string

	Days          []string
	TshirtOption  string
	NeedsCatering *bool
}

type SubmitResult struct {
	GroupID          string
	CheckoutURL      string
	TotalAmountPence int
	HasMinor         bool
	ConsentFormURL   string
}

type RegistrationSummary struct {
	GroupID          string
	PaymentStatus    PaymentStatus
	TotalAmountPence int
	Currency         string
	ContactEmail     string
	Campers          []SummaryCamper
}

type SummaryCamper struct {
	FirstName string
	LastName  string
}

package domain

import "time"

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

func (s Submission) HasMinor() bool {
	for _, c := range s.Campers {
		if c.Age < 18 {
			return true
		}
	}
	return false
}

func (s Submission) DepositPayingCount() int {
	n := 0
	for _, c := range s.Campers {
		if c.Attendance.Type == AttendanceFullWeek && c.Age >= MinDepositAge {
			n++
		}
	}
	return n
}

func (s Submission) CoachEligibleCount() int64 {
	var n int64
	for _, c := range s.Campers {
		if c.Attendance.Type != AttendanceFullWeek {
			continue
		}
		if c.CoachEligible() {
			n++
		}
	}
	return n
}

func (s Submission) ToGroup(groupID string, status PaymentStatus, totalPence int, currency string,
	submittedAt time.Time, paidAt *time.Time) (Group, []Camper) {
	g := Group{
		ID:               groupID,
		ContactFirstName: s.Contact.FirstName,
		ContactLastName:  s.Contact.LastName,
		ContactEmail:     s.Contact.Email,
		ContactPhone:     s.Contact.Phone,
		PaymentStatus:    status,
		TotalAmountPence: totalPence,
		Currency:         currency,
		CreatedAt:        submittedAt,
		PaidAt:           paidAt,
	}
	campers := make([]Camper, 0, len(s.Campers))
	for _, c := range s.Campers {
		camper := Camper{
			GroupID:        groupID,
			IsMainContact:  c.IsMainContact,
			FirstName:      c.FirstName,
			LastName:       c.LastName,
			Gender:         c.Gender,
			Age:            c.Age,
			CellLeaderName: c.CellLeaderName,
			IsCellLeader:   c.IsCellLeader,
			AttendanceType: c.Attendance.Type,
		}
		switch c.Attendance.Type {
		case AttendanceFullWeek:
			camper.ShirtSize = ptrIfNotEmpty(c.Attendance.ShirtSize)
			camper.DietaryRequirements = ptrIfNotEmpty(c.Attendance.DietaryRequirements)
			camper.NeedsCoach = c.Attendance.NeedsCoach
			camper.AccommodationFirstChoice = ptrIfNotEmpty(c.Attendance.AccommodationFirstChoice)
			camper.AccommodationSecondChoice = ptrIfNotEmpty(c.Attendance.AccommodationSecondChoice)
			camper.RoommateRequests = ptrIfNotEmpty(c.Attendance.RoommateRequests)
		case AttendanceDayPass:
			camper.ShirtSize = ptrIfNotEmpty(c.Attendance.ShirtSize)
			camper.DietaryRequirements = ptrIfNotEmpty(c.Attendance.DietaryRequirements)
			camper.DayPassDays = c.Attendance.Days
			camper.DayPassTshirtOption = ptrIfNotEmpty(c.Attendance.TshirtOption)
			camper.DayPassNeedsCatering = c.Attendance.NeedsCatering
		}
		campers = append(campers, camper)
	}
	return g, campers
}

func ptrIfNotEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

package registration

import "time"

const (
	AttendanceFullWeek = "full_week"
	AttendanceDayPass  = "day_pass"

	GenderMale   = "male"
	GenderFemale = "female"

	TshirtOptionTeamActivities = "team_activities"
	TshirtOptionTshirtOnly     = "tshirt_only"
	TshirtOptionNone           = "none"

	PaymentPending        = "pending"
	PaymentPaid           = "paid"
	PaymentFailed         = "failed"
	PaymentFailedCapacity = "failed_capacity"
	PaymentRefunded       = "refunded"
	PaymentCancelled      = "cancelled"
)

// ValidDayPassDays enumerates the allowed day_pass_days entries.
var ValidDayPassDays = map[string]struct{}{
	"mon": {}, "tue": {}, "wed": {}, "thu": {}, "fri": {},
}

// PriceCode is the lookup key into the prices table used when computing a
// group total.
const (
	PriceFullWeekAdult = "full_week_adult"
	PriceFullWeekChild = "full_week_child"
	PriceDayPass       = "day_pass"
	PriceTshirtOnly    = "tshirt_only"
)

// Group is the persisted form of a registration_groups row.
type Group struct {
	ID                    string
	ContactFirstName      string
	ContactLastName       string
	ContactEmail          string
	ContactPhone          string
	PaymentStatus         string
	StripeSessionID       *string
	StripePaymentIntentID *string
	TotalAmountPence      int
	Currency              string
	CreatedAt             time.Time
	PaidAt                *time.Time
}

// Camper is the persisted form of a registrations row.
type Camper struct {
	ID                   string
	GroupID              string
	IsMainContact        bool
	FirstName            string
	LastName             string
	Gender               string
	Age                  int
	CellLeaderName       string
	IsCellLeader         bool
	AttendanceType       string
	ShirtSize            *string
	DietaryRequirements  *string
	NeedsCoach           *bool
	AccommodationCode    *string
	DayPassDays          []string
	DayPassTshirtOption  *string
	DayPassNeedsCatering *bool
	CreatedAt            time.Time
}

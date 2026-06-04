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

	BillingNone        = "none"
	BillingAllocated   = "allocated"
	BillingInvoiced    = "invoiced"
	BillingBalancePaid = "balance_paid"
	BillingReleased    = "released"
)

// ValidDayPassDays enumerates the allowed day_pass_days entries.
var ValidDayPassDays = map[string]struct{}{
	"mon": {}, "tue": {}, "wed": {}, "thu": {}, "fri": {},
}

// PriceCode is the lookup key into the prices table used when computing a
// group total. As of v2, the only priced line item is the per-full-week-camper
// deposit; day-pass campers contribute 0.
const (
	PriceDeposit = "deposit"
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
	StripeCustomerID      *string
	StripeInvoiceID       *string
	BillingStatus         string
	InvoiceDueAt          *time.Time
	BalancePaidAt         *time.Time
}

// Camper is the persisted form of a registrations row.
type Camper struct {
	ID                        string
	GroupID                   string
	IsMainContact             bool
	FirstName                 string
	LastName                  string
	Gender                    string
	Age                       int
	CellLeaderName            string
	IsCellLeader              bool
	AttendanceType            string
	ShirtSize                 *string
	DietaryRequirements       *string
	NeedsCoach                *bool
	AccommodationFirstChoice  *string
	AccommodationSecondChoice *string
	RoommateRequests          *string
	DayPassDays               []string
	DayPassTshirtOption       *string
	DayPassNeedsCatering       *bool
	AllocatedAccommodationCode *string
	BilledStripePriceID        *string
	CreatedAt                  time.Time
}

// AccommodationType mirrors accommodation_types for billing lookups.
type AccommodationType struct {
	Code          string  `json:"code"`
	DisplayName   string  `json:"display_name"`
	StripePriceID *string `json:"stripe_price_id,omitempty"`
}

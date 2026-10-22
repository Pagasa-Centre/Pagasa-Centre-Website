package domain

import "time"

const (
	AttendanceFullWeek = "full_week"
	AttendanceDayPass  = "day_pass"

	GenderMale   = "male"
	GenderFemale = "female"

	TshirtOptionTeamActivities = "team_activities"
	TshirtOptionTshirtOnly     = "tshirt_only"
	TshirtOptionNone           = "none"
)

// ValidDayPassDays enumerates the allowed day_pass_days entries.
var ValidDayPassDays = map[string]struct{}{
	"mon": {}, "tue": {}, "wed": {}, "thu": {}, "fri": {},
}

const (
	PriceDeposit = "deposit"

	PaymentModeDeposit = "deposit"
	PaymentModeFull    = "full"
)

// Group is the persisted form of a registration_groups row.
type Group struct {
	ID                       string
	ContactFirstName         string
	ContactLastName          string
	ContactEmail             string
	ContactPhone             string
	PaymentStatus            PaymentStatus
	CheckoutSessionRef       *string
	DepositPaymentRef        *string
	TotalAmountPence         int
	Currency                 string
	CreatedAt                time.Time
	PaidAt                   *time.Time
	BillingCustomerRef       *string
	BalanceInvoiceRef        *string
	BillingStatus            BillingStatus
	InvoiceDueAt             *time.Time
	BalancePaidAt            *time.Time
	Version                  int
	LastAction               *string
	LastActionBy             *string
	LastActionAt             *time.Time
	IsFree                   bool
	PaidInFullAtRegistration bool

	CoachIncludedInBalance bool
	CoachInvoiceRef        *string
	CoachInvoiceDueAt      *time.Time
	CoachFeePaidAt         *time.Time
	CoachFeeWaivedAt       *time.Time
}

// FreeCode is a row from free_codes for admin listing.
type FreeCode struct {
	ID            string
	Code          string
	CreatedAt     time.Time
	CreatedBy     string
	Note          *string
	UsedAt        *time.Time
	UsedByGroupID *string
	RevokedAt     *time.Time
}

// Camper is the persisted form of a registrations row.
type Camper struct {
	ID                         string
	GroupID                    string
	IsMainContact              bool
	FirstName                  string
	LastName                   string
	Gender                     string
	Age                        int
	CellLeaderName             string
	IsCellLeader               bool
	AttendanceType             string
	ShirtSize                  *string
	DietaryRequirements        *string
	NeedsCoach                 *bool
	AccommodationFirstChoice   *string
	AccommodationSecondChoice  *string
	RoommateRequests           *string
	DayPassDays                []string
	DayPassTshirtOption        *string
	DayPassNeedsCatering       *bool
	AllocatedAccommodationCode *string
	AllocatedUnitCode          *string
	BilledPriceRef             *string
	DepositCreditPence         int
	DepositOwedPence           int
	DepositPaidPence           int
	CreatedAt                  time.Time
}

// ListFilterBilling narrows admin listings by balance billing status.
type ListFilterBilling struct {
	PaymentStatus PaymentStatus
	BillingStatus BillingStatus
}

// AccommodationUnit is one physical unit within a tier (e.g. "Caravan 5").
type AccommodationUnit struct {
	Code              string
	AccommodationCode string
	DisplayName       string
	Capacity          int
	SortOrder         int
}

// AccommodationType mirrors accommodation_types for billing lookups.
type AccommodationType struct {
	Code                     string
	DisplayName              string
	Capacity                 *int
	PriceRef                 *string
	AvailableForRegistration bool
}

package dto

import (
	"encoding/json"
	"time"

	"pagasacentre/backend/internal/registration/domain"
)

type GroupResponse struct {
	ID                       string               `json:"id"`
	ContactFirstName         string               `json:"contact_first_name"`
	ContactLastName          string               `json:"contact_last_name"`
	ContactEmail             string               `json:"contact_email"`
	ContactPhone             string               `json:"contact_phone"`
	PaymentStatus            domain.PaymentStatus `json:"payment_status"`
	StripeSessionID          *string              `json:"stripe_session_id,omitempty"`
	StripePaymentIntentID    *string              `json:"stripe_payment_intent_id,omitempty"`
	TotalAmountPence         int                  `json:"total_amount_pence"`
	Currency                 string               `json:"currency"`
	CreatedAt                time.Time            `json:"created_at"`
	PaidAt                   *time.Time           `json:"paid_at,omitempty"`
	StripeCustomerID         *string              `json:"stripe_customer_id,omitempty"`
	StripeInvoiceID          *string              `json:"stripe_invoice_id,omitempty"`
	BillingStatus            domain.BillingStatus `json:"billing_status"`
	InvoiceDueAt             *time.Time           `json:"invoice_due_at,omitempty"`
	BalancePaidAt            *time.Time           `json:"balance_paid_at,omitempty"`
	Version                  int                  `json:"version"`
	LastAction               *string              `json:"last_action,omitempty"`
	LastActionBy             *string              `json:"last_action_by,omitempty"`
	LastActionAt             *time.Time           `json:"last_action_at,omitempty"`
	IsFree                   bool                 `json:"is_free"`
	PaidInFullAtRegistration bool                 `json:"paid_in_full_at_registration"`
	CoachIncludedInBalance   bool                 `json:"coach_included_in_balance"`
	StripeCoachInvoiceID     *string              `json:"stripe_coach_invoice_id,omitempty"`
	CoachInvoiceDueAt        *time.Time           `json:"coach_invoice_due_at,omitempty"`
	CoachFeePaidAt           *time.Time           `json:"coach_fee_paid_at,omitempty"`
	CoachFeeWaivedAt         *time.Time           `json:"coach_fee_waived_at,omitempty"`
	Campers                  []CamperResponse     `json:"campers"`
}

type CamperResponse struct {
	ID                         string    `json:"id"`
	GroupID                    string    `json:"group_id"`
	IsMainContact              bool      `json:"is_main_contact"`
	FirstName                  string    `json:"first_name"`
	LastName                   string    `json:"last_name"`
	Gender                     string    `json:"gender"`
	Age                        int       `json:"age"`
	CellLeaderName             string    `json:"cell_leader_name"`
	IsCellLeader               bool      `json:"is_cell_leader"`
	AttendanceType             string    `json:"attendance_type"`
	ShirtSize                  *string   `json:"shirt_size,omitempty"`
	DietaryRequirements        *string   `json:"dietary_requirements,omitempty"`
	NeedsCoach                 *bool     `json:"needs_coach,omitempty"`
	AccommodationFirstChoice   *string   `json:"accommodation_first_choice,omitempty"`
	AccommodationSecondChoice  *string   `json:"accommodation_second_choice,omitempty"`
	RoommateRequests           *string   `json:"roommate_requests,omitempty"`
	DayPassDays                []string  `json:"day_pass_days,omitempty"`
	DayPassTshirtOption        *string   `json:"day_pass_tshirt_option,omitempty"`
	DayPassNeedsCatering       *bool     `json:"day_pass_needs_catering,omitempty"`
	AllocatedAccommodationCode *string   `json:"allocated_accommodation_code,omitempty"`
	AllocatedUnitCode          *string   `json:"allocated_unit_code,omitempty"`
	BilledStripePriceID        *string   `json:"billed_stripe_price_id,omitempty"`
	DepositCreditPence         int       `json:"deposit_credit_pence"`
	DepositOwedPence           int       `json:"deposit_owed_pence"`
	DepositPaidPence           int       `json:"deposit_paid_pence"`
	CreatedAt                  time.Time `json:"created_at"`
}

type FreeCodeResponse struct {
	ID            string     `json:"id"`
	Code          string     `json:"code"`
	CreatedAt     time.Time  `json:"created_at"`
	CreatedBy     string     `json:"created_by"`
	Note          *string    `json:"note,omitempty"`
	UsedAt        *time.Time `json:"used_at,omitempty"`
	UsedByGroupID *string    `json:"used_by_group_id,omitempty"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
}

type AccommodationTypeResponse struct {
	Code                     string  `json:"code"`
	DisplayName              string  `json:"display_name"`
	Capacity                 *int    `json:"capacity,omitempty"`
	StripePriceID            *string `json:"stripe_price_id,omitempty"`
	AvailableForRegistration bool    `json:"available_for_registration"`
}

type AccommodationUnitResponse struct {
	Code              string `json:"code"`
	AccommodationCode string `json:"accommodation_code"`
	DisplayName       string `json:"display_name"`
	Capacity          int    `json:"capacity"`
	SortOrder         int    `json:"sort_order"`
}

type EventResponse struct {
	ID        int64           `json:"id"`
	CreatedAt time.Time       `json:"created_at"`
	ActorName string          `json:"actor_name"`
	Action    string          `json:"action"`
	GroupID   *string         `json:"group_id,omitempty"`
	Summary   string          `json:"summary"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

type EditCamperResponse struct {
	CamperName    string `json:"camper_name"`
	PreviousName  string `json:"previous_name"`
	InvoiceVoided bool   `json:"invoice_voided"`
	Repriced      bool   `json:"repriced"`
}

type AddCamperResponse struct {
	CamperID         string `json:"camper_id"`
	CamperName       string `json:"camper_name"`
	DepositOwedPence int    `json:"deposit_owed_pence"`
	InvoiceVoided    bool   `json:"invoice_voided"`
	NeedsAllocation  bool   `json:"needs_allocation"`
}

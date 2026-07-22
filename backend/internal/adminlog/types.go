package adminlog

import (
	"encoding/json"
	"time"
)

// Event is broadcast over SSE when something changes in the admin dashboard.
type Event struct {
	ID        int64           `json:"id"`
	CreatedAt time.Time       `json:"created_at"`
	ActorName string          `json:"actor_name"`
	Action    string          `json:"action"`
	GroupID   *string         `json:"group_id,omitempty"`
	Summary   string          `json:"summary"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

// Action constants for admin_events.action.
const (
	ActionLogin               = "login"
	ActionAllocate            = "allocate"
	ActionAllocationEdited    = "allocation_edited"
	ActionUnallocate          = "unallocate"
	ActionInvoiceSent         = "invoice_sent"
	ActionInvoiceResent       = "invoice_resent"
	ActionRelease             = "release"
	ActionCancel              = "cancel"
	ActionExtendDue           = "extend_due"
	ActionContactUpdated      = "contact_updated"
	ActionRegistrationsToggle = "registrations_toggle"
	ActionPriceUpdated        = "price_updated"
	ActionSweep               = "sweep"
	ActionBalancePaid         = "balance_paid"
	ActionFreeCodeGenerated   = "free_code_generated"
	ActionFreeCodeRevoked     = "free_code_revoked"
	ActionFreeConfirmed       = "free_confirmed"
	ActionRegistrationDeleted = "registration_deleted"
	ActionCamperRemoved         = "camper_removed"
	ActionCamperConverted       = "camper_converted"
	ActionCamperUpdated         = "camper_updated"
	ActionCoachInvoiceSent      = "coach_invoice_sent"
	ActionCoachFeeWaived        = "coach_fee_waived"
	ActionCoachFeeUnwaived      = "coach_fee_unwaived"

	ActionAccommodationAvailability = "accommodation_availability"
)

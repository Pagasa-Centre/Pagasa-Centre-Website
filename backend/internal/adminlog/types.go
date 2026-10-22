package adminlog

import (
	"encoding/json"
	"time"
)

// Event is broadcast over SSE when something changes in the admin dashboard.
type Event struct {
	ID        int64
	CreatedAt time.Time
	ActorName string
	Action    string
	GroupID   *string
	Summary   string
	Metadata  json.RawMessage
}

// Action constants for admin_events.action.
const (
	ActionLogin                = "login"
	ActionAllocate             = "allocate"
	ActionAllocationEdited     = "allocation_edited"
	ActionUnallocate           = "unallocate"
	ActionInvoiceSent          = "invoice_sent"
	ActionInvoiceResent        = "invoice_resent"
	ActionRelease              = "release"
	ActionCancel               = "cancel"
	ActionExtendDue            = "extend_due"
	ActionContactUpdated       = "contact_updated"
	ActionPaymentStatusUpdated = "payment_status_updated"
	ActionRegistrationsToggle  = "registrations_toggle"
	ActionPriceUpdated         = "price_updated"
	ActionSweep                = "sweep"
	ActionBalancePaid          = "balance_paid"
	ActionFreeCodeGenerated    = "free_code_generated"
	ActionFreeCodeRevoked      = "free_code_revoked"
	ActionFreeConfirmed        = "free_confirmed"
	ActionRegistrationDeleted  = "registration_deleted"
	ActionCamperRemoved        = "camper_removed"
	ActionCamperConverted      = "camper_converted"
	ActionCamperUpdated        = "camper_updated"
	ActionCamperCoachUpdated   = "camper_coach_updated"
	ActionCamperEdited         = "camper_edited"
	ActionCamperAdded          = "camper_added"
	ActionMainContactMoved     = "main_contact_moved"
	ActionCamperDepositWaived  = "camper_deposit_waived"

	ActionCoachInvoiceSent = "coach_invoice_sent"
	ActionCoachFeeWaived   = "coach_fee_waived"
	ActionCoachFeeUnwaived = "coach_fee_unwaived"
	ActionSheetResynced    = "sheet_resynced"

	ActionPaymentModeToggle         = "payment_mode_toggle"
	ActionAccommodationAvailability = "accommodation_availability"
)

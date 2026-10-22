package domain

import "pagasacentre/backend/pkg/commonlibrary/apperr"

var (
	ErrCoachInBalance       = apperr.Invalid("coach fee is included in the balance invoice for this group")
	ErrSponsoredNoCoach     = apperr.Invalid("church-sponsored registrations are not charged for the coach")
	ErrGroupNotFound        = apperr.NotFound("group not found")
	ErrCamperNotFound       = apperr.NotFound("camper not found")
	ErrRegistrationNotFound = apperr.NotFound("registration not found")
	ErrDepositUnpaid        = apperr.Invalid("deposit must be paid first")
	ErrBalancePaid          = apperr.Invalid("balance already paid")
	ErrInvoiceOpen          = apperr.Invalid("invoice is open; use release instead")
	ErrInvoiceAlreadySent   = apperr.Invalid("invoice already sent")
)

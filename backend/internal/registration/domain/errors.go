package domain

import "pagasacentre/backend/pkg/commonlibrary/apperr"

var (
	ErrGroupNotFound        = apperr.NotFound("group not found")
	ErrCamperNotFound       = apperr.NotFound("camper not found")
	ErrRegistrationNotFound = apperr.NotFound("registration not found")
	ErrDepositUnpaid        = apperr.Invalid("deposit must be paid first")
	ErrBalancePaid          = apperr.Invalid("balance already paid")
	ErrInvoiceOpen          = apperr.Invalid("invoice is open; use release instead")
	ErrInvoiceAlreadySent   = apperr.Invalid("invoice already sent")
)

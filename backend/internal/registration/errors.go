package registration

import "pagasacentre/backend/pkg/commonlibrary/apperr"

var (
	ErrRegistrationsClosed      = apperr.Conflict("registrations_closed", "Camp registrations are currently closed")
	ErrInvalidFreeCode          = apperr.InvalidCode("invalid_free_code", "That sponsorship code is not valid or has already been used.")
	ErrAccommodationUnavailable = apperr.Conflict("accommodation_unavailable", "One or more selected accommodation types are no longer available. Please refresh and choose again.")
	ErrMissingIdentifier        = apperr.InvalidCode("missing_identifier", "either session_id or group_id is required")
	ErrCodeNotRevocable         = apperr.Invalid("code cannot be revoked (already used or revoked)")
)

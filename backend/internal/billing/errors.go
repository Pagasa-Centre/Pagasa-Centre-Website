package billing

import "pagasacentre/backend/pkg/commonlibrary/apperr"

var (
	ErrCoachPriceNotConfigured = apperr.Invalid("STRIPE_PRICE_COACH is not configured")
)

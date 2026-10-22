package billing

import "pagasacentre/backend/pkg/commonlibrary/apperr"

var (
	ErrCoachInBalance          = apperr.Invalid("coach fee is included in the balance invoice for this group")
	ErrSponsoredNoCoach        = apperr.Invalid("church-sponsored registrations are not charged for the coach")
	ErrCoachPriceNotConfigured = apperr.Invalid("STRIPE_PRICE_COACH is not configured")
)

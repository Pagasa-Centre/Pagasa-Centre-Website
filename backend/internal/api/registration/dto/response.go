package dto

import "pagasacentre/backend/internal/registration/domain"

type SubmitResponse struct {
	GroupID          string `json:"group_id"`
	CheckoutURL      string `json:"checkout_url"`
	TotalAmountPence int    `json:"total_amount_pence"`
	HasMinor         bool   `json:"has_minor"`
	ConsentFormURL   string `json:"consent_form_url,omitempty"`
}

type SummaryResponse struct {
	GroupID          string                  `json:"group_id"`
	PaymentStatus    domain.PaymentStatus    `json:"payment_status"`
	TotalAmountPence int                     `json:"total_amount_pence"`
	Currency         string                  `json:"currency"`
	ContactEmail     string                  `json:"contact_email"`
	Campers          []SummaryCamperResponse `json:"campers"`
}

type SummaryCamperResponse struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type PricingResponse struct {
	Mode                   string                `json:"mode"`
	Currency               string                `json:"currency"`
	DepositAmountPence     int                   `json:"deposit_amount_pence"`
	AccommodationTiers     []PricingTierResponse `json:"accommodation_tiers"`
	ChildUnder3AmountPence int                   `json:"child_under3_amount_pence"`
	DayPassAmountPence     int                   `json:"day_pass_amount_pence"`
	CoachAmountPence       int                   `json:"coach_amount_pence"`
}

type PricingTierResponse struct {
	Code        string `json:"code"`
	DisplayName string `json:"display_name"`
	AmountPence int    `json:"amount_pence"`
}

type ShirtSizeResponse struct {
	Code        string `json:"code"`
	DisplayName string `json:"display_name"`
	Category    string `json:"category"`
}

type ShirtSizesResponse struct {
	Sizes         []ShirtSizeResponse            `json:"sizes"`
	ByCategory    map[string][]ShirtSizeResponse `json:"by_category"`
	NotApplicable string                         `json:"not_applicable"`
}

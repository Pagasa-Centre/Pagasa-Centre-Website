package dto

import (
	"pagasacentre/backend/internal/registration"
	"pagasacentre/backend/internal/registration/domain"
)

type SubmitResponse = domain.SubmitResponse
type SummaryResponse = domain.SummaryResponse
type PricingResponse = registration.PricingSnapshot

type ShirtSizesResponse struct {
	Sizes         []domain.ShirtSize        `json:"sizes"`
	ByCategory    map[string][]domain.ShirtSize `json:"by_category"`
	NotApplicable string                    `json:"not_applicable"`
}

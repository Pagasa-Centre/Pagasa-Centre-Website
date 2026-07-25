package mapper

import (
	"pagasacentre/backend/internal/api/registration/dto"
	"pagasacentre/backend/internal/registration"
	"pagasacentre/backend/internal/registration/domain"
)

func RequestToDomain(req dto.SubmitRequest) domain.SubmitRequest {
	return domain.SubmitRequest(req)
}

func SubmitToResponse(resp *domain.SubmitResponse) dto.SubmitResponse {
	if resp == nil {
		return dto.SubmitResponse{}
	}
	return dto.SubmitResponse(*resp)
}

func SummaryToResponse(resp *domain.SummaryResponse) dto.SummaryResponse {
	if resp == nil {
		return dto.SummaryResponse{}
	}
	return dto.SummaryResponse(*resp)
}

func PricingToResponse(snap *registration.PricingSnapshot) dto.PricingResponse {
	if snap == nil {
		return dto.PricingResponse{}
	}
	return dto.PricingResponse(*snap)
}

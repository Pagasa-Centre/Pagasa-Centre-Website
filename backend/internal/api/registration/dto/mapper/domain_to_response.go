package mapper

import (
	"pagasacentre/backend/internal/api/registration/dto"
	"pagasacentre/backend/internal/registration"
	"pagasacentre/backend/internal/registration/domain"
)

func SubmitToResponse(resp *domain.SubmitResult) dto.SubmitResponse {
	if resp == nil {
		return dto.SubmitResponse{}
	}
	return dto.SubmitResponse{
		GroupID:          resp.GroupID,
		CheckoutURL:      resp.CheckoutURL,
		TotalAmountPence: resp.TotalAmountPence,
		HasMinor:         resp.HasMinor,
		ConsentFormURL:   resp.ConsentFormURL,
	}
}

func SummaryToResponse(resp *domain.RegistrationSummary) dto.SummaryResponse {
	if resp == nil {
		return dto.SummaryResponse{}
	}
	campers := make([]dto.SummaryCamperResponse, len(resp.Campers))
	for i, c := range resp.Campers {
		campers[i] = dto.SummaryCamperResponse{FirstName: c.FirstName, LastName: c.LastName}
	}
	return dto.SummaryResponse{
		GroupID:          resp.GroupID,
		PaymentStatus:    resp.PaymentStatus,
		TotalAmountPence: resp.TotalAmountPence,
		Currency:         resp.Currency,
		ContactEmail:     resp.ContactEmail,
		Campers:          campers,
	}
}

func PricingToResponse(snap *registration.PricingSnapshot) dto.PricingResponse {
	if snap == nil {
		return dto.PricingResponse{}
	}
	var outTiers []dto.PricingTierResponse
	if snap.AccommodationTiers != nil {
		outTiers = make([]dto.PricingTierResponse, len(snap.AccommodationTiers))
		for i, t := range snap.AccommodationTiers {
			outTiers[i] = dto.PricingTierResponse{
				Code:        t.Code,
				DisplayName: t.DisplayName,
				AmountPence: t.AmountPence,
			}
		}
	}
	return dto.PricingResponse{
		Mode:                   snap.Mode,
		Currency:               snap.Currency,
		DepositAmountPence:     snap.DepositAmountPence,
		AccommodationTiers:     outTiers,
		ChildUnder3AmountPence: snap.ChildUnder3AmountPence,
		DayPassAmountPence:     snap.DayPassAmountPence,
		CoachAmountPence:       snap.CoachAmountPence,
	}
}

func ShirtSizesToResponse(sizes []domain.ShirtSize, grouped map[string][]domain.ShirtSize, notApplicable string) dto.ShirtSizesResponse {
	outSizes := make([]dto.ShirtSizeResponse, len(sizes))
	for i, s := range sizes {
		outSizes[i] = dto.ShirtSizeResponse{Code: s.Code, DisplayName: s.DisplayName, Category: s.Category}
	}
	byCat := make(map[string][]dto.ShirtSizeResponse, len(grouped))
	for cat, list := range grouped {
		mapped := make([]dto.ShirtSizeResponse, len(list))
		for i, s := range list {
			mapped[i] = dto.ShirtSizeResponse{Code: s.Code, DisplayName: s.DisplayName, Category: s.Category}
		}
		byCat[cat] = mapped
	}
	return dto.ShirtSizesResponse{
		Sizes:         outSizes,
		ByCategory:    byCat,
		NotApplicable: notApplicable,
	}
}

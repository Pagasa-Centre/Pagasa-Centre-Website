package mapper

import (
	"pagasacentre/backend/internal/api/camp/dto"
	"pagasacentre/backend/internal/camp/domain"
)

func ConfigToResponse(cfg domain.Config) dto.ConfigResponse {
	return dto.ConfigResponse{
		Name:                    cfg.Name,
		LocationName:            cfg.LocationName,
		LocationAddr:            cfg.LocationAddr,
		WebsiteURL:              cfg.WebsiteURL,
		StartDate:               cfg.StartDate,
		EndDate:                 cfg.EndDate,
		RegistrationsOpen:       cfg.RegistrationsOpen,
		RegistrationPaymentMode: cfg.RegistrationPaymentMode,
	}
}

func PricesToResponse(prices []domain.Price) dto.PricesResponse {
	if prices == nil {
		prices = []domain.Price{}
	}
	out := make([]dto.PriceResponse, len(prices))
	for i, p := range prices {
		out[i] = dto.PriceResponse{
			Code:        p.Code,
			DisplayName: p.DisplayName,
			AmountPence: p.AmountPence,
			Currency:    p.Currency,
		}
	}
	return dto.PricesResponse{Prices: out}
}

package mapper

import (
	"pagasacentre/backend/internal/api/camp/dto"
	"pagasacentre/backend/internal/camp/domain"
)

func ConfigToResponse(cfg domain.Config) dto.ConfigResponse {
	return cfg
}

func PricesToResponse(prices []domain.Price) dto.PricesResponse {
	if prices == nil {
		prices = []domain.Price{}
	}
	return dto.PricesResponse{Prices: prices}
}

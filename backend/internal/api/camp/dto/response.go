package dto

import "pagasacentre/backend/internal/camp/domain"

type ConfigResponse = domain.Config

type PriceResponse = domain.Price

type PricesResponse struct {
	Prices []domain.Price `json:"prices"`
}

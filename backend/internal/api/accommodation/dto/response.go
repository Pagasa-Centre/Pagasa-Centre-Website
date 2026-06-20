package dto

import "pagasacentre/backend/internal/accommodation/domain"

type OptionsResponse struct {
	Accommodations []domain.Option `json:"accommodations"`
}

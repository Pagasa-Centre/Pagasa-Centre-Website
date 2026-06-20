package mapper

import (
	"pagasacentre/backend/internal/api/accommodation/dto"
	"pagasacentre/backend/internal/accommodation/domain"
)

func OptionsToResponse(list []domain.Option) dto.OptionsResponse {
	if list == nil {
		list = []domain.Option{}
	}
	return dto.OptionsResponse{Accommodations: list}
}

package mapper

import (
	"pagasacentre/backend/internal/accommodation/domain"
	"pagasacentre/backend/internal/api/accommodation/dto"
)

func OptionsToResponse(list []domain.Option) dto.OptionsResponse {
	if list == nil {
		list = []domain.Option{}
	}
	out := make([]dto.OptionResponse, len(list))
	for i, o := range list {
		out[i] = dto.OptionResponse{
			Code:                     o.Code,
			DisplayName:              o.DisplayName,
			Notes:                    o.Notes,
			AvailableForRegistration: o.AvailableForRegistration,
		}
	}
	return dto.OptionsResponse{Accommodations: out}
}

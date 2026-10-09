package mapper

import (
	"pagasacentre/backend/internal/api/sermon/dto"
	"pagasacentre/backend/internal/sermon/domain"
)

func SermonToResponse(s domain.Sermon) dto.SermonResponse {
	return dto.SermonResponse{
		VideoID: s.VideoID,
		Title:   s.Title,
	}
}

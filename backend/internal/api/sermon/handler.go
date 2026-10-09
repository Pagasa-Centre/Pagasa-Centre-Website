package sermon

import (
	"context"
	"errors"
	"net/http"

	"pagasacentre/backend/internal/api/sermon/dto/mapper"
	sermonsvc "pagasacentre/backend/internal/sermon"
	"pagasacentre/backend/internal/sermon/domain"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
	"pagasacentre/backend/pkg/commonlibrary/render"
)

type latestService interface {
	Latest(ctx context.Context) (*domain.Sermon, error)
}

type Handler struct {
	service latestService
}

func NewHandler(service latestService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetLatest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := h.service.Latest(r.Context())
		if err != nil {
			if errors.Is(err, sermonsvc.ErrNotFound) {
				commonerrors.WriteError(w, commonerrors.NotFound("No latest sermon available"))
				return
			}
			commonerrors.WriteError(w, commonerrors.Internal(err.Error()))
			return
		}
		render.Json(w, http.StatusOK, mapper.SermonToResponse(*s))
	}
}

package accommodation

import (
	"net/http"

	"pagasacentre/backend/internal/api/accommodation/dto/mapper"
	accomsvc "pagasacentre/backend/internal/accommodation"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
	"pagasacentre/backend/pkg/commonlibrary/render"
)

type Handler struct {
	service *accomsvc.Service
}

func NewHandler(service *accomsvc.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ListAccommodations() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := h.service.ListOptions(r.Context())
		if err != nil {
			commonerrors.WriteError(w, commonerrors.Internal(err.Error()))
			return
		}
		render.Json(w, http.StatusOK, mapper.OptionsToResponse(list))
	}
}

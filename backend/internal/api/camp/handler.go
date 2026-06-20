package camp

import (
	"net/http"

	"pagasacentre/backend/internal/api/camp/dto/mapper"
	campsvc "pagasacentre/backend/internal/camp"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
	"pagasacentre/backend/pkg/commonlibrary/render"
)

type Handler struct {
	service *campsvc.Service
}

func NewHandler(service *campsvc.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetCamp() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := h.service.GetConfig(r.Context())
		if err != nil {
			commonerrors.WriteError(w, commonerrors.Internal(err.Error()))
			return
		}
		render.Json(w, http.StatusOK, mapper.ConfigToResponse(cfg))
	}
}

func (h *Handler) ListPrices() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		prices, err := h.service.ListPrices(r.Context())
		if err != nil {
			commonerrors.WriteError(w, commonerrors.Internal(err.Error()))
			return
		}
		render.Json(w, http.StatusOK, mapper.PricesToResponse(prices))
	}
}

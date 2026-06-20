package registration

import (
	"net/http"

	"pagasacentre/backend/internal/api/registration/dto"
	"pagasacentre/backend/internal/api/registration/dto/mapper"
	regsvc "pagasacentre/backend/internal/registration"
	"pagasacentre/backend/internal/registration/domain"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
	"pagasacentre/backend/pkg/commonlibrary/render"
	"pagasacentre/backend/pkg/commonlibrary/request"
)

type Handler struct {
	service *regsvc.Service
}

func NewHandler(service *regsvc.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Submit() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body dto.SubmitRequest
		if err := request.Decode(r, &body); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		resp, err := h.service.Submit(r.Context(), mapper.RequestToDomain(body))
		if err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		render.Json(w, http.StatusOK, mapper.SubmitToResponse(resp))
	}
}

func (h *Handler) ListShirtSizes() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sizes := regsvc.ListShirtSizes()
		grouped := map[string][]domain.ShirtSize{"adult": {}, "child": {}}
		for _, s := range sizes {
			grouped[s.Category] = append(grouped[s.Category], s)
		}
		render.Json(w, http.StatusOK, dto.ShirtSizesResponse{
			Sizes:         sizes,
			ByCategory:    grouped,
			NotApplicable: domain.ShirtSizeNotApplicable,
		})
	}
}

func (h *Handler) Summary() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.URL.Query().Get("session_id")
		groupID := r.URL.Query().Get("group_id")
		summary, err := h.service.Summary(r.Context(), sessionID, groupID)
		if err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		if summary == nil {
			commonerrors.WriteError(w, commonerrors.APIError{
				Code:    "not_found",
				Message: "no registration found for the supplied identifier",
			})
			return
		}
		render.Json(w, http.StatusOK, mapper.SummaryToResponse(summary))
	}
}

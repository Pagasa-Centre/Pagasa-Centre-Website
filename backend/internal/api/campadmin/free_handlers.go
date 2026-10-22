package campadmin

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"pagasacentre/backend/internal/admin"
	"pagasacentre/backend/internal/billing"
	usecase "pagasacentre/backend/internal/campadmin"
	"pagasacentre/backend/internal/middleware"
	"pagasacentre/backend/internal/registration"
	"pagasacentre/backend/internal/registration/domain"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
	"pagasacentre/backend/pkg/commonlibrary/render"
	"pagasacentre/backend/pkg/commonlibrary/request"
)

func postGenerateFreeCode(auth middleware.AuthConfig, svc *usecase.Service) http.HandlerFunc {
	type body struct {
		Password string `json:"password"`
		Note     string `json:"note"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if auth.FreeCodePassword == "" {
			commonerrors.WriteError(w, commonerrors.APIError{
				Code:    "forbidden",
				Message: "sponsorship codes are not configured on this server",
			})
			return
		}
		var b body
		if err := request.Decode(r, &b); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		if !auth.FreeCodePasswordMatches(b.Password) {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		actor := admin.ActorFrom(r.Context())
		code, err := svc.GenerateFreeCode(r.Context(), actor, b.Note)
		if err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		render.Json(w, http.StatusOK, map[string]string{"code": code})
	}
}

func getFreeCodes(svc *registration.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		codes, err := svc.ListFreeCodes(r.Context())
		if err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		if codes == nil {
			codes = []domain.FreeCode{}
		}
		render.Json(w, http.StatusOK, map[string]any{"codes": codes})
	}
}

func postRevokeFreeCode(svc *usecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := svc.RevokeFreeCode(r.Context(), admin.ActorFrom(r.Context()), id); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postConfirmFree(svc *usecase.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = request.Decode(r, &body)
		groupID := chi.URLParam(r, "groupID")
		actor := admin.ActorFrom(r.Context())
		if err := svc.ConfirmFree(r.Context(), groupID, actor, billing.ExpectedVersion(body.ExpectedVersion)); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

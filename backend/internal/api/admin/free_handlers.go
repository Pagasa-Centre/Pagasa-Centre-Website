package admin

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"pagasacentre/backend/internal/admin"
	"pagasacentre/backend/internal/adminlog"
	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/middleware"
	"pagasacentre/backend/internal/registration"
	"pagasacentre/backend/internal/registration/domain"
	regstorage "pagasacentre/backend/internal/registration/storage"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
	"pagasacentre/backend/pkg/commonlibrary/render"
	"pagasacentre/backend/pkg/commonlibrary/request"
)

func postGenerateFreeCode(auth middleware.AuthConfig, svc *registration.Service, rec *adminlog.Recorder) http.HandlerFunc {
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
		admin.Audit(rec, r, adminlog.ActionFreeCodeGenerated, nil,
			actor+" generated sponsorship code "+code, map[string]string{"code": code})
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

func postRevokeFreeCode(svc *registration.Service, rec *adminlog.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := svc.RevokeFreeCode(r.Context(), id); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		admin.Audit(rec, r, adminlog.ActionFreeCodeRevoked, nil,
			admin.ActorFrom(r.Context())+" revoked sponsorship code "+id, map[string]string{"code_id": id})
		w.WriteHeader(http.StatusNoContent)
	}
}

func postConfirmFree(billSvc *billing.Service, rec *adminlog.Recorder, regRepo *regstorage.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body billing.VersionedBody
		_ = request.Decode(r, &body)
		groupID := chi.URLParam(r, "groupID")
		actor := admin.ActorFrom(r.Context())
		if err := billSvc.ConfirmFree(r.Context(), groupID, actor, billing.ExpectedVersion(body.ExpectedVersion)); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		g, _ := regRepo.FindGroupByID(r.Context(), groupID)
		gid := groupID
		admin.Audit(rec, r, adminlog.ActionFreeConfirmed, &gid,
			"Confirmed sponsorship for "+admin.GroupSummary(g), nil)
		w.WriteHeader(http.StatusNoContent)
	}
}

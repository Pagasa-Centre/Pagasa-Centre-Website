// Package admin exposes management endpoints for church staff.
//
// TODO(auth): these routes are intentionally unauthenticated for v1. Bolt a
// single middleware in front of admin.Mount before any public deployment.
package admin

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"pagasacentre/backend/internal/accommodation"
	"pagasacentre/backend/internal/camp"
	"pagasacentre/backend/internal/httpx"
	"pagasacentre/backend/internal/registration"
)

// Mount wires admin routes onto r.
func Mount(
	r chi.Router,
	regRepo *registration.Repository,
	accRepo *accommodation.Repository,
	campRepo *camp.Repository,
) {
	r.Get("/registrations", listRegistrationsJSON(regRepo))
	r.Get("/registrations.csv", listRegistrationsCSV(regRepo))
	r.Patch("/registrations/{groupID}", patchRegistration(regRepo))
	r.Put("/accommodations/{code}", putAccommodation(accRepo))
	r.Put("/prices/{code}", putPrice(campRepo))
}

type groupView struct {
	registration.Group
	Campers []registration.Camper `json:"campers"`
}

func listRegistrationsJSON(repo *registration.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		groups, err := repo.List(ctx, registration.ListFilter{
			PaymentStatus: r.URL.Query().Get("status"),
		})
		if err != nil {
			httpx.WriteError(w, httpx.Internal(err.Error()))
			return
		}
		if len(groups) == 0 {
			httpx.WriteJSON(w, http.StatusOK, map[string]any{"groups": []groupView{}})
			return
		}
		ids := make([]string, len(groups))
		for i, g := range groups {
			ids[i] = g.ID
		}
		campers, err := repo.CampersForGroups(ctx, ids)
		if err != nil {
			httpx.WriteError(w, httpx.Internal(err.Error()))
			return
		}
		byGroup := map[string][]registration.Camper{}
		for _, c := range campers {
			byGroup[c.GroupID] = append(byGroup[c.GroupID], c)
		}
		views := make([]groupView, len(groups))
		for i, g := range groups {
			views[i] = groupView{Group: g, Campers: byGroup[g.ID]}
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"groups": views})
	}
}

func listRegistrationsCSV(repo *registration.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		groups, err := repo.List(ctx, registration.ListFilter{
			PaymentStatus: r.URL.Query().Get("status"),
		})
		if err != nil {
			httpx.WriteError(w, httpx.Internal(err.Error()))
			return
		}
		ids := make([]string, len(groups))
		for i, g := range groups {
			ids[i] = g.ID
		}
		campers, err := repo.CampersForGroups(ctx, ids)
		if err != nil {
			httpx.WriteError(w, httpx.Internal(err.Error()))
			return
		}
		byGroup := map[string][]registration.Camper{}
		for _, c := range campers {
			byGroup[c.GroupID] = append(byGroup[c.GroupID], c)
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		filename := "registrations-" + time.Now().UTC().Format("20060102-150405") + ".csv"
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
		if err := WriteCSV(w, groups, byGroup); err != nil {
			httpx.WriteError(w, httpx.Internal(err.Error()))
		}
	}
}

func patchRegistration(repo *registration.Repository) http.HandlerFunc {
	type body struct {
		PaymentStatus string `json:"payment_status"`
	}
	allowed := map[string]struct{}{
		registration.PaymentPending: {}, registration.PaymentPaid: {},
		registration.PaymentFailed: {}, registration.PaymentFailedCapacity: {},
		registration.PaymentRefunded: {}, registration.PaymentCancelled: {},
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var b body
		if err := httpx.DecodeJSON(r, &b); err != nil {
			httpx.WriteError(w, err)
			return
		}
		if _, ok := allowed[b.PaymentStatus]; !ok {
			httpx.WriteError(w, httpx.BadRequest("invalid payment_status", nil))
			return
		}
		if err := repo.MarkStatus(r.Context(), chi.URLParam(r, "groupID"), b.PaymentStatus); err != nil {
			httpx.WriteError(w, httpx.Internal(err.Error()))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func putAccommodation(repo *accommodation.Repository) http.HandlerFunc {
	type body struct {
		Capacity *int `json:"capacity"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var b body
		if err := httpx.DecodeJSON(r, &b); err != nil {
			httpx.WriteError(w, err)
			return
		}
		if err := repo.UpdateCapacity(r.Context(), chi.URLParam(r, "code"), b.Capacity); err != nil {
			httpx.WriteError(w, httpx.Internal(err.Error()))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func putPrice(repo *camp.Repository) http.HandlerFunc {
	type body struct {
		AmountPence int `json:"amount_pence"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var b body
		if err := httpx.DecodeJSON(r, &b); err != nil {
			httpx.WriteError(w, err)
			return
		}
		if b.AmountPence < 0 {
			httpx.WriteError(w, httpx.BadRequest("amount_pence must be >= 0", nil))
			return
		}
		if err := repo.UpdatePrice(r.Context(), chi.URLParam(r, "code"), b.AmountPence); err != nil {
			httpx.WriteError(w, httpx.Internal(err.Error()))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

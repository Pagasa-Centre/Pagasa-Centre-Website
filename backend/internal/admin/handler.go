// Package admin exposes management endpoints for church staff (White Team).
package admin

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/camp"
	"pagasacentre/backend/internal/httpx"
	"pagasacentre/backend/internal/registration"
)

// Mount wires admin routes onto r. Public login routes are outside the auth
// middleware; everything else requires a valid session cookie.
func Mount(
	r chi.Router,
	auth AuthConfig,
	regRepo *registration.Repository,
	campRepo *camp.Repository,
	billSvc *billing.Service,
) {
	r.Post("/login", handleLogin(auth))
	r.Post("/logout", handleLogout(auth))
	r.Get("/session", handleSession(auth))

	r.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return RequireAdmin(auth, next)
		})

		r.Get("/registrations", listRegistrationsJSON(regRepo))
		r.Get("/registrations.csv", listRegistrationsCSV(regRepo))
		r.Patch("/registrations/{groupID}", patchRegistration(regRepo))
		r.Put("/prices/{code}", putPrice(campRepo))

		r.Get("/accommodations", listAccommodationTypes(regRepo))
		r.Put("/registrations/{groupID}/allocation", putAllocation(billSvc))
		r.Post("/registrations/{groupID}/invoice", postInvoice(billSvc))
		r.Post("/registrations/invoice-bulk", postInvoiceBulk(billSvc))
		r.Post("/registrations/{groupID}/release", postRelease(billSvc))
		r.Post("/registrations/{groupID}/invoice/resend", postResendInvoice(billSvc))
		r.Patch("/registrations/{groupID}/invoice-due", patchInvoiceDue(billSvc))
		r.Post("/billing/sweep", postBillingSweep(billSvc))
	})
}

type groupView struct {
	registration.Group
	Campers []registration.Camper `json:"campers"`
}

func listRegistrationsJSON(repo *registration.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		f := registration.ListFilterBilling{
			PaymentStatus: r.URL.Query().Get("status"),
			BillingStatus: r.URL.Query().Get("billing_status"),
		}
		groups, err := repo.ListWithBilling(ctx, f)
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
		groups, err := repo.ListWithBilling(ctx, registration.ListFilterBilling{
			PaymentStatus: r.URL.Query().Get("status"),
			BillingStatus: r.URL.Query().Get("billing_status"),
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

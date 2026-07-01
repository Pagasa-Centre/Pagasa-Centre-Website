// Package admin exposes management endpoints for church staff (White Team).
package admin

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"pagasacentre/backend/internal/admin"
	"pagasacentre/backend/internal/adminlog"
	"pagasacentre/backend/internal/billing"
	campstorage "pagasacentre/backend/internal/camp/storage"
	"pagasacentre/backend/internal/middleware"
	"pagasacentre/backend/internal/registration"
	"pagasacentre/backend/internal/registration/domain"
	regstorage "pagasacentre/backend/internal/registration/storage"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
	"pagasacentre/backend/pkg/commonlibrary/render"
	"pagasacentre/backend/pkg/commonlibrary/request"
)

// contactService corrects a group's contact details (and optionally re-sends
// the deposit confirmation). Satisfied by *payment.Service.
type contactService interface {
	UpdateGroupContact(ctx context.Context, groupID, firstName, lastName, email, phone string, resendConfirmation bool) error
}

// Mount wires admin routes onto r. Public login routes are outside the auth
// middleware; everything else requires a valid session cookie.
func Mount(
	r chi.Router,
	auth middleware.AuthConfig,
	regRepo *regstorage.Repository,
	campRepo *campstorage.Repository,
	regSvc *registration.Service,
	billSvc *billing.Service,
	contactSvc contactService,
	rec *adminlog.Recorder,
) {
	r.Post("/login", middleware.HandleLogin(auth, rec))
	r.Post("/logout", middleware.HandleLogout(auth))
	r.Get("/session", middleware.HandleSession(auth))

	r.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middleware.RequireAdmin(auth, next)
		})

		r.Get("/registrations", listRegistrationsJSON(regRepo))
		r.Get("/registrations.csv", listRegistrationsCSV(regRepo))
		r.Patch("/registrations/{groupID}", patchRegistration(regRepo))
		r.Patch("/registrations/{groupID}/contact", patchContact(contactSvc, rec))
		r.Put("/prices/{code}", putPrice(campRepo, rec))

		r.Get("/camp-config", getCampConfig(campRepo))
		r.Put("/registrations-open", putRegistrationsOpen(campRepo, rec))

		r.Get("/accommodations", listAccommodationTypes(regRepo))
		r.Get("/accommodation-units", listAccommodationUnits(regRepo))
		r.Get("/events", listEvents(rec))
		r.Put("/registrations/{groupID}/allocation", putAllocation(billSvc, rec, regRepo))
		r.Post("/registrations/{groupID}/unallocate", postUnallocate(billSvc, rec, regRepo))
		r.Post("/registrations/{groupID}/invoice", postInvoice(billSvc, rec, regRepo))
		r.Post("/registrations/invoice-bulk", postInvoiceBulk(billSvc, rec, regRepo))
		r.Post("/registrations/{groupID}/release", postRelease(billSvc, rec, regRepo))
		r.Post("/registrations/{groupID}/delete", deleteRegistration(billSvc, rec))
		r.Post("/registrations/{groupID}/invoice/resend", postResendInvoice(billSvc, rec, regRepo))
		r.Patch("/registrations/{groupID}/invoice-due", patchInvoiceDue(billSvc, rec, regRepo))
		r.Post("/billing/sweep", postBillingSweep(billSvc, rec))

		r.Post("/free-codes", postGenerateFreeCode(auth, regSvc, rec))
		r.Get("/free-codes", getFreeCodes(regSvc))
		r.Post("/free-codes/{id}/revoke", postRevokeFreeCode(regSvc, rec))
		r.Post("/registrations/{groupID}/confirm-free", postConfirmFree(billSvc, rec, regRepo))
	})
}

type groupView struct {
	domain.Group
	Campers []domain.Camper `json:"campers"`
}

func listRegistrationsJSON(repo *regstorage.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		f := domain.ListFilterBilling{
			PaymentStatus: r.URL.Query().Get("status"),
			BillingStatus: r.URL.Query().Get("billing_status"),
		}
		groups, err := repo.ListWithBilling(ctx, f)
		if err != nil {
			commonerrors.WriteError(w, commonerrors.Internal(err.Error()))
			return
		}
		if len(groups) == 0 {
			render.Json(w, http.StatusOK, map[string]any{"groups": []groupView{}})
			return
		}
		ids := make([]string, len(groups))
		for i, g := range groups {
			ids[i] = g.ID
		}
		campers, err := repo.CampersForGroups(ctx, ids)
		if err != nil {
			commonerrors.WriteError(w, commonerrors.Internal(err.Error()))
			return
		}
		byGroup := map[string][]domain.Camper{}
		for _, c := range campers {
			byGroup[c.GroupID] = append(byGroup[c.GroupID], c)
		}
		views := make([]groupView, len(groups))
		for i, g := range groups {
			views[i] = groupView{Group: g, Campers: byGroup[g.ID]}
		}
		render.Json(w, http.StatusOK, map[string]any{"groups": views})
	}
}

func listRegistrationsCSV(repo *regstorage.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		groups, err := repo.ListWithBilling(ctx, domain.ListFilterBilling{
			PaymentStatus: r.URL.Query().Get("status"),
			BillingStatus: r.URL.Query().Get("billing_status"),
		})
		if err != nil {
			commonerrors.WriteError(w, commonerrors.Internal(err.Error()))
			return
		}
		ids := make([]string, len(groups))
		for i, g := range groups {
			ids[i] = g.ID
		}
		campers, err := repo.CampersForGroups(ctx, ids)
		if err != nil {
			commonerrors.WriteError(w, commonerrors.Internal(err.Error()))
			return
		}
		byGroup := map[string][]domain.Camper{}
		for _, c := range campers {
			byGroup[c.GroupID] = append(byGroup[c.GroupID], c)
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		filename := "registrations-" + time.Now().UTC().Format("20060102-150405") + ".csv"
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
		if err := admin.WriteCSV(w, groups, byGroup); err != nil {
			commonerrors.WriteError(w, commonerrors.Internal(err.Error()))
		}
	}
}

func patchRegistration(repo *regstorage.Repository) http.HandlerFunc {
	type body struct {
		PaymentStatus string `json:"payment_status"`
	}
	allowed := map[string]struct{}{
		domain.PaymentPending: {}, domain.PaymentPaid: {},
		domain.PaymentFailed: {}, domain.PaymentFailedCapacity: {},
		domain.PaymentRefunded: {}, domain.PaymentCancelled: {},
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var b body
		if err := request.Decode(r, &b); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		if _, ok := allowed[b.PaymentStatus]; !ok {
			commonerrors.WriteError(w, commonerrors.BadRequest("invalid payment_status", nil))
			return
		}
		if err := repo.MarkStatus(r.Context(), chi.URLParam(r, "groupID"), b.PaymentStatus); err != nil {
			commonerrors.WriteError(w, commonerrors.Internal(err.Error()))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// patchContact corrects a group's contact details and, if requested, re-sends
// the deposit confirmation email to the corrected address. Used when someone
// mistypes their email at registration.
func patchContact(svc contactService, rec *adminlog.Recorder) http.HandlerFunc {
	type body struct {
		FirstName          string `json:"first_name"`
		LastName           string `json:"last_name"`
		Email              string `json:"email"`
		Phone              string `json:"phone"`
		ResendConfirmation bool   `json:"resend_confirmation"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var b body
		if err := request.Decode(r, &b); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		b.FirstName = strings.TrimSpace(b.FirstName)
		b.LastName = strings.TrimSpace(b.LastName)
		b.Email = strings.TrimSpace(b.Email)
		b.Phone = strings.TrimSpace(b.Phone)

		fields := map[string]string{}
		if b.FirstName == "" {
			fields["first_name"] = "is required"
		}
		if b.LastName == "" {
			fields["last_name"] = "is required"
		}
		if !registration.ValidEmail(b.Email) {
			fields["email"] = "must be a valid email"
		}
		if b.Phone == "" {
			fields["phone"] = "is required"
		}
		if len(fields) > 0 {
			commonerrors.WriteError(w, commonerrors.ValidationFailed(fields))
			return
		}

		err := svc.UpdateGroupContact(r.Context(), chi.URLParam(r, "groupID"),
			b.FirstName, b.LastName, b.Email, b.Phone, b.ResendConfirmation)
		if err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		gid := chi.URLParam(r, "groupID")
		summary := "Updated contact details for " + b.FirstName + " " + b.LastName
		if b.ResendConfirmation {
			summary += " (confirmation email re-sent)"
		}
		admin.Audit(rec, r, adminlog.ActionContactUpdated, &gid, summary, nil)
		w.WriteHeader(http.StatusNoContent)
	}
}

func getCampConfig(repo *campstorage.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := repo.GetConfig(r.Context())
		if err != nil {
			commonerrors.WriteError(w, commonerrors.Internal(err.Error()))
			return
		}
		render.Json(w, http.StatusOK, cfg)
	}
}

func putRegistrationsOpen(repo *campstorage.Repository, rec *adminlog.Recorder) http.HandlerFunc {
	type body struct {
		Open *bool `json:"open"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var b body
		if err := request.Decode(r, &b); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		if b.Open == nil {
			commonerrors.WriteError(w, commonerrors.BadRequest("missing 'open' boolean", nil))
			return
		}
		if err := repo.SetRegistrationsOpen(r.Context(), *b.Open); err != nil {
			commonerrors.WriteError(w, commonerrors.Internal(err.Error()))
			return
		}
		state := "closed"
		if *b.Open {
			state = "opened"
		}
		admin.Audit(rec, r, adminlog.ActionRegistrationsToggle, nil,
			admin.ActorFrom(r.Context())+" "+state+" public registration", map[string]bool{"open": *b.Open})
		render.Json(w, http.StatusOK, map[string]bool{"registrations_open": *b.Open})
	}
}

func putPrice(repo *campstorage.Repository, rec *adminlog.Recorder) http.HandlerFunc {
	type body struct {
		AmountPence int `json:"amount_pence"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var b body
		if err := request.Decode(r, &b); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		if b.AmountPence < 0 {
			commonerrors.WriteError(w, commonerrors.BadRequest("amount_pence must be >= 0", nil))
			return
		}
		if err := repo.UpdatePrice(r.Context(), chi.URLParam(r, "code"), b.AmountPence); err != nil {
			commonerrors.WriteError(w, commonerrors.Internal(err.Error()))
			return
		}
		code := chi.URLParam(r, "code")
		admin.Audit(rec, r, adminlog.ActionPriceUpdated, nil,
			admin.ActorFrom(r.Context())+" updated price "+code, map[string]any{"code": code, "amount_pence": b.AmountPence})
		w.WriteHeader(http.StatusNoContent)
	}
}

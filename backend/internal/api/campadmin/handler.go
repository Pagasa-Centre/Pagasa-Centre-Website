// Package campadmin exposes management endpoints for church staff (White Team).
package campadmin

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"pagasacentre/backend/internal/admin"
	"pagasacentre/backend/internal/adminlog"
	"pagasacentre/backend/internal/billing"
	campdomain "pagasacentre/backend/internal/camp/domain"
	"pagasacentre/backend/internal/camp"
	"pagasacentre/backend/internal/middleware"
	"pagasacentre/backend/internal/registration"
	"pagasacentre/backend/internal/registration/domain"
		commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
	"pagasacentre/backend/pkg/commonlibrary/render"
	"pagasacentre/backend/pkg/commonlibrary/request"
)

// contactService corrects a group's contact details (and optionally re-sends
// the deposit confirmation). Satisfied by *payment.Service.
type contactService interface {
	UpdateGroupContact(ctx context.Context, groupID, firstName, lastName, email, phone string, resendConfirmation bool) error
}

type groupFinder interface {
	FindGroup(ctx context.Context, groupID string) (*domain.Group, error)
	CampersForGroup(ctx context.Context, groupID string) ([]domain.Camper, error)
}

type registrationLister interface {
	ListGroupsWithCampers(ctx context.Context, f domain.ListFilterBilling) ([]domain.Group, map[string][]domain.Camper, error)
}

type paymentStatusSetter interface {
	SetPaymentStatus(ctx context.Context, groupID, status string) error
}

type campConfigService interface {
	GetConfig(ctx context.Context) (campdomain.Config, error)
	SetRegistrationsOpen(ctx context.Context, open bool) error
	SetRegistrationPaymentMode(ctx context.Context, mode string) error
	UpdatePrice(ctx context.Context, code string, amountPence int) error
}

// Mount wires admin routes onto r. Public login routes are outside the auth
// middleware; everything else requires a valid session cookie.
func Mount(
	r chi.Router,
	auth middleware.AuthConfig,
	regSvc *registration.Service,
	campSvc *camp.Service,
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

		r.Get("/registrations", listRegistrationsJSON(regSvc))
		r.Get("/registrations.csv", listRegistrationsCSV(regSvc))
		r.Patch("/registrations/{groupID}", patchRegistration(regSvc))
		r.Patch("/registrations/{groupID}/contact", patchContact(contactSvc, rec))
		r.Put("/prices/{code}", putPrice(campSvc, rec))

		r.Get("/camp-config", getCampConfig(campSvc))
		r.Put("/registrations-open", putRegistrationsOpen(campSvc, rec))
		r.Put("/registration-payment-mode", putRegistrationPaymentMode(campSvc, regSvc, rec))

		r.Get("/accommodations", listAccommodationTypes(regSvc))
		r.Put("/accommodations/{code}/availability", putAccommodationAvailability(regSvc, rec))
		r.Get("/accommodation-units", listAccommodationUnits(regSvc))
		r.Get("/events", listEvents(rec))
		r.Put("/registrations/{groupID}/allocation", putAllocation(billSvc, rec, regSvc))
		r.Post("/registrations/{groupID}/unallocate", postUnallocate(billSvc, rec, regSvc))
		r.Post("/registrations/{groupID}/invoice", postInvoice(billSvc, rec, regSvc))
		r.Post("/registrations/invoice-bulk", postInvoiceBulk(billSvc, rec, regSvc))
		r.Post("/registrations/{groupID}/coach-invoice", postCoachInvoice(billSvc, rec, regSvc))
		r.Post("/registrations/coach-invoice-bulk", postCoachInvoiceBulk(billSvc, rec, regSvc))
		r.Post("/registrations/{groupID}/coach-invoice/waive", postWaiveCoachFee(billSvc, rec, regSvc))
		r.Post("/registrations/{groupID}/coach-invoice/unwaive", postUnwaiveCoachFee(billSvc, rec, regSvc))
		r.Post("/registrations/{groupID}/mark-paid", postMarkPaid(billSvc, rec, regSvc))
		r.Post("/registrations/{groupID}/release", postRelease(billSvc, rec, regSvc, false))
		r.Post("/registrations/{groupID}/cancel", postRelease(billSvc, rec, regSvc, true))
		r.Post("/registrations/{groupID}/delete", deleteRegistration(billSvc, rec))
		r.Post("/registrations/{groupID}/campers/{camperID}/delete", deleteCamper(billSvc, rec, regSvc))
		r.Post("/registrations/{groupID}/campers/{camperID}/convert-day-visitor", postConvertDayVisitor(billSvc, rec, regSvc))
		r.Patch("/registrations/{groupID}/campers/{camperID}/day-pass", patchDayPassCamper(billSvc, rec))
		r.Patch("/registrations/{groupID}/campers/{camperID}/coach", patchCamperCoach(billSvc, rec))
		r.Patch("/registrations/{groupID}/campers/{camperID}", patchCamper(billSvc, rec))
		r.Post("/registrations/{groupID}/campers", postAddCamper(billSvc, rec))
		r.Post("/registrations/{groupID}/campers/{camperID}/make-main-contact", postMakeMainContact(billSvc, rec))
		r.Post("/registrations/{groupID}/campers/{camperID}/waive-deposit", postWaiveCamperDeposit(billSvc, rec))
		r.Post("/registrations/{groupID}/invoice/resend", postResendInvoice(billSvc, rec, regSvc))
		r.Post("/registrations/sheet-resync-all", postResyncAllSheets(billSvc, rec))
		r.Patch("/registrations/{groupID}/invoice-due", patchInvoiceDue(billSvc, rec, regSvc))
		r.Post("/billing/sweep", postBillingSweep(billSvc, rec))

		r.Post("/free-codes", postGenerateFreeCode(auth, regSvc, rec))
		r.Get("/free-codes", getFreeCodes(regSvc))
		r.Post("/free-codes/{id}/revoke", postRevokeFreeCode(regSvc, rec))
		r.Post("/registrations/{groupID}/confirm-free", postConfirmFree(billSvc, rec, regSvc))
	})
}

type groupView struct {
	domain.Group
	Campers []domain.Camper `json:"campers"`
}

func listRegistrationsJSON(svc registrationLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		f := domain.ListFilterBilling{
			PaymentStatus: r.URL.Query().Get("status"),
			BillingStatus: r.URL.Query().Get("billing_status"),
		}
		groups, byGroup, err := svc.ListGroupsWithCampers(ctx, f)
		if err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		if len(groups) == 0 {
			render.Json(w, http.StatusOK, map[string]any{"groups": []groupView{}})
			return
		}
		views := make([]groupView, len(groups))
		for i, g := range groups {
			views[i] = groupView{Group: g, Campers: byGroup[g.ID]}
		}
		render.Json(w, http.StatusOK, map[string]any{"groups": views})
	}
}

func listRegistrationsCSV(svc registrationLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		groups, byGroup, err := svc.ListGroupsWithCampers(ctx, domain.ListFilterBilling{
			PaymentStatus: r.URL.Query().Get("status"),
			BillingStatus: r.URL.Query().Get("billing_status"),
		})
		if err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		filename := "registrations-" + time.Now().UTC().Format("20060102-150405") + ".csv"
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
		if err := admin.WriteCSV(w, groups, byGroup); err != nil {
			commonerrors.WriteError(w, err)
		}
	}
}

func patchRegistration(svc paymentStatusSetter) http.HandlerFunc {
	type body struct {
		PaymentStatus string `json:"payment_status"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var b body
		if err := request.Decode(r, &b); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		if err := svc.SetPaymentStatus(r.Context(), chi.URLParam(r, "groupID"), b.PaymentStatus); err != nil {
			commonerrors.WriteError(w, err)
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

func getCampConfig(svc campConfigService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := svc.GetConfig(r.Context())
		if err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		render.Json(w, http.StatusOK, cfg)
	}
}

func putRegistrationsOpen(svc campConfigService, rec *adminlog.Recorder) http.HandlerFunc {
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
		if err := svc.SetRegistrationsOpen(r.Context(), *b.Open); err != nil {
			commonerrors.WriteError(w, err)
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

func putRegistrationPaymentMode(campSvc campConfigService, regSvc *registration.Service, rec *adminlog.Recorder) http.HandlerFunc {
	type body struct {
		Mode *string `json:"mode"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var b body
		if err := request.Decode(r, &b); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		if b.Mode == nil {
			commonerrors.WriteError(w, commonerrors.BadRequest("missing 'mode' string", nil))
			return
		}
		mode := strings.TrimSpace(*b.Mode)
		if mode != domain.PaymentModeDeposit && mode != domain.PaymentModeFull {
			commonerrors.WriteError(w, commonerrors.BadRequest("mode must be 'deposit' or 'full'", nil))
			return
		}
		if mode == domain.PaymentModeFull {
			if err := regSvc.ValidateFullPaymentPricing(r.Context()); err != nil {
				commonerrors.WriteError(w, err)
				return
			}
		}
		if err := campSvc.SetRegistrationPaymentMode(r.Context(), mode); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		label := "deposit mode"
		if mode == domain.PaymentModeFull {
			label = "full payment mode"
		}
		admin.Audit(rec, r, adminlog.ActionPaymentModeToggle, nil,
			admin.ActorFrom(r.Context())+" switched registration to "+label,
			map[string]string{"mode": mode})
		render.Json(w, http.StatusOK, map[string]string{"registration_payment_mode": mode})
	}
}

func putPrice(svc campConfigService, rec *adminlog.Recorder) http.HandlerFunc {
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
		if err := svc.UpdatePrice(r.Context(), chi.URLParam(r, "code"), b.AmountPence); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		code := chi.URLParam(r, "code")
		admin.Audit(rec, r, adminlog.ActionPriceUpdated, nil,
			admin.ActorFrom(r.Context())+" updated price "+code, map[string]any{"code": code, "amount_pence": b.AmountPence})
		w.WriteHeader(http.StatusNoContent)
	}
}

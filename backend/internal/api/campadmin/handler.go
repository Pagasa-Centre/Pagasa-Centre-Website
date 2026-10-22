// Package campadmin exposes management endpoints for church staff (White Team).
package campadmin

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"pagasacentre/backend/internal/admin"
	admindto "pagasacentre/backend/internal/api/campadmin/dto"
	"pagasacentre/backend/internal/api/campadmin/dto/mapper"
	"pagasacentre/backend/internal/camp"
	campdomain "pagasacentre/backend/internal/camp/domain"
	usecase "pagasacentre/backend/internal/campadmin"
	"pagasacentre/backend/internal/middleware"
	"pagasacentre/backend/internal/registration"
	"pagasacentre/backend/internal/registration/domain"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
	"pagasacentre/backend/pkg/commonlibrary/render"
	"pagasacentre/backend/pkg/commonlibrary/request"
)

type registrationLister interface {
	ListGroupsWithCampers(ctx context.Context, f domain.ListFilterBilling) ([]domain.Group, map[string][]domain.Camper, error)
}

type campConfigReader interface {
	GetConfig(ctx context.Context) (campdomain.Config, error)
}

// Mount wires admin routes onto r. Public login routes are outside the auth
// middleware; everything else requires a valid session cookie.
func Mount(
	r chi.Router,
	auth middleware.AuthConfig,
	regSvc *registration.Service,
	campSvc *camp.Service,
	adminSvc *usecase.Service,
) {
	r.Post("/login", middleware.HandleLogin(auth, adminSvc))
	r.Post("/logout", middleware.HandleLogout(auth))
	r.Get("/session", middleware.HandleSession(auth))

	r.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middleware.RequireAdmin(auth, next)
		})

		r.Get("/registrations", listRegistrationsJSON(regSvc))
		r.Get("/registrations.csv", listRegistrationsCSV(regSvc))
		r.Patch("/registrations/{groupID}", patchRegistration(adminSvc))
		r.Patch("/registrations/{groupID}/contact", patchContact(adminSvc))
		r.Put("/prices/{code}", putPrice(adminSvc))

		r.Get("/camp-config", getCampConfig(campSvc))
		r.Put("/registrations-open", putRegistrationsOpen(adminSvc))
		r.Put("/registration-payment-mode", putRegistrationPaymentMode(adminSvc))

		r.Get("/accommodations", listAccommodationTypes(regSvc))
		r.Put("/accommodations/{code}/availability", putAccommodationAvailability(adminSvc))
		r.Get("/accommodation-units", listAccommodationUnits(regSvc))
		r.Get("/events", listEvents(adminSvc))
		r.Put("/registrations/{groupID}/allocation", putAllocation(adminSvc))
		r.Post("/registrations/{groupID}/unallocate", postUnallocate(adminSvc))
		r.Post("/registrations/{groupID}/invoice", postInvoice(adminSvc))
		r.Post("/registrations/invoice-bulk", postInvoiceBulk(adminSvc))
		r.Post("/registrations/{groupID}/coach-invoice", postCoachInvoice(adminSvc))
		r.Post("/registrations/coach-invoice-bulk", postCoachInvoiceBulk(adminSvc))
		r.Post("/registrations/{groupID}/coach-invoice/waive", postWaiveCoachFee(adminSvc))
		r.Post("/registrations/{groupID}/coach-invoice/unwaive", postUnwaiveCoachFee(adminSvc))
		r.Post("/registrations/{groupID}/mark-paid", postMarkPaid(adminSvc))
		r.Post("/registrations/{groupID}/release", postRelease(adminSvc, false))
		r.Post("/registrations/{groupID}/cancel", postRelease(adminSvc, true))
		r.Post("/registrations/{groupID}/delete", deleteRegistration(adminSvc))
		r.Post("/registrations/{groupID}/campers/{camperID}/delete", deleteCamper(adminSvc))
		r.Post("/registrations/{groupID}/campers/{camperID}/convert-day-visitor", postConvertDayVisitor(adminSvc))
		r.Patch("/registrations/{groupID}/campers/{camperID}/day-pass", patchDayPassCamper(adminSvc))
		r.Patch("/registrations/{groupID}/campers/{camperID}/coach", patchCamperCoach(adminSvc))
		r.Patch("/registrations/{groupID}/campers/{camperID}", patchCamper(adminSvc))
		r.Post("/registrations/{groupID}/campers", postAddCamper(adminSvc))
		r.Post("/registrations/{groupID}/campers/{camperID}/make-main-contact", postMakeMainContact(adminSvc))
		r.Post("/registrations/{groupID}/campers/{camperID}/waive-deposit", postWaiveCamperDeposit(adminSvc))
		r.Post("/registrations/{groupID}/invoice/resend", postResendInvoice(adminSvc))
		r.Post("/registrations/sheet-resync-all", postResyncAllSheets(adminSvc))
		r.Patch("/registrations/{groupID}/invoice-due", patchInvoiceDue(adminSvc))
		r.Post("/billing/sweep", postBillingSweep(adminSvc))

		r.Post("/free-codes", postGenerateFreeCode(auth, adminSvc))
		r.Get("/free-codes", getFreeCodes(regSvc))
		r.Post("/free-codes/{id}/revoke", postRevokeFreeCode(adminSvc))
		r.Post("/registrations/{groupID}/confirm-free", postConfirmFree(adminSvc))
	})
}

func listRegistrationsJSON(svc registrationLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		f := domain.ListFilterBilling{
			PaymentStatus: domain.PaymentStatus(r.URL.Query().Get("status")),
			BillingStatus: domain.BillingStatus(r.URL.Query().Get("billing_status")),
		}
		groups, byGroup, err := svc.ListGroupsWithCampers(ctx, f)
		if err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		if len(groups) == 0 {
			render.Json(w, http.StatusOK, map[string]any{"groups": []admindto.GroupResponse{}})
			return
		}
		views := make([]admindto.GroupResponse, len(groups))
		for i, g := range groups {
			views[i] = mapper.GroupToResponse(g, byGroup[g.ID])
		}
		render.Json(w, http.StatusOK, map[string]any{"groups": views})
	}
}

func listRegistrationsCSV(svc registrationLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		groups, byGroup, err := svc.ListGroupsWithCampers(ctx, domain.ListFilterBilling{
			PaymentStatus: domain.PaymentStatus(r.URL.Query().Get("status")),
			BillingStatus: domain.BillingStatus(r.URL.Query().Get("billing_status")),
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

func patchRegistration(svc *usecase.Service) http.HandlerFunc {
	type body struct {
		PaymentStatus string `json:"payment_status"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var b body
		if err := request.Decode(r, &b); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		if err := svc.SetPaymentStatus(r.Context(), chi.URLParam(r, "groupID"), admin.ActorFrom(r.Context()), b.PaymentStatus); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func patchContact(svc *usecase.Service) http.HandlerFunc {
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

		err := svc.UpdateContact(r.Context(), chi.URLParam(r, "groupID"), admin.ActorFrom(r.Context()),
			b.FirstName, b.LastName, b.Email, b.Phone, b.ResendConfirmation)
		if err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func getCampConfig(svc campConfigReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := svc.GetConfig(r.Context())
		if err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		render.Json(w, http.StatusOK, mapper.CampConfigToResponse(cfg))
	}
}

func putRegistrationsOpen(svc *usecase.Service) http.HandlerFunc {
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
		if err := svc.SetRegistrationsOpen(r.Context(), admin.ActorFrom(r.Context()), *b.Open); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		render.Json(w, http.StatusOK, map[string]bool{"registrations_open": *b.Open})
	}
}

func putRegistrationPaymentMode(svc *usecase.Service) http.HandlerFunc {
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
		if err := svc.SetRegistrationPaymentMode(r.Context(), admin.ActorFrom(r.Context()), mode); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		render.Json(w, http.StatusOK, map[string]string{"registration_payment_mode": mode})
	}
}

func putPrice(svc *usecase.Service) http.HandlerFunc {
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
		code := chi.URLParam(r, "code")
		if err := svc.UpdatePrice(r.Context(), admin.ActorFrom(r.Context()), code, b.AmountPence); err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

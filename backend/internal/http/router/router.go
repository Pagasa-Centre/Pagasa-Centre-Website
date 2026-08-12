package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	accomapi "pagasacentre/backend/internal/api/accommodation"
	campadminapi "pagasacentre/backend/internal/api/campadmin"
	campapi "pagasacentre/backend/internal/api/camp"
	consentapi "pagasacentre/backend/internal/api/consent"
	paymentapi "pagasacentre/backend/internal/api/payment"
	regapi "pagasacentre/backend/internal/api/registration"
	"pagasacentre/backend/internal/adminlog"
	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/middleware"
	campstorage "pagasacentre/backend/internal/camp/storage"
	"pagasacentre/backend/internal/payment"
	"pagasacentre/backend/internal/registration"
	regstorage "pagasacentre/backend/internal/registration/storage"
	"pagasacentre/backend/pkg/commonlibrary/render"
)

type Config struct {
	AllowedOrigin string
	Commit        string

	AdminAuth middleware.AuthConfig

	CampHandler           *campapi.Handler
	AccommodationHandler  *accomapi.Handler
	RegistrationHandler   *regapi.Handler
	PaymentHandler        *paymentapi.Handler
	ConsentHandler        *consentapi.Handler

	RegRepo   *regstorage.Repository
	CampRepo  *campstorage.Repository
	RegSvc    *registration.Service
	BillSvc   *billing.Service
	PaySvc    *payment.Service
	AdminRec  *adminlog.Recorder
	AdminHub  *adminlog.Hub
}

func New(cfg Config) http.Handler {
	r := chi.NewRouter()
	middleware.UseDefaults(r, cfg.AllowedOrigin)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		render.Json(w, http.StatusOK, map[string]string{"status": "ok", "commit": cfg.Commit})
	})

	r.Get("/camp-admin/stream", campadminapi.HandleStream(cfg.AdminAuth, cfg.AdminHub))

	r.Route("/api", func(r chi.Router) {
		middleware.WithRequestTimeout(r)
		r.Get("/camp", cfg.CampHandler.GetCamp())
		r.Get("/prices", cfg.CampHandler.ListPrices())
		r.Get("/accommodations", cfg.AccommodationHandler.ListAccommodations())
		r.Post("/registrations", cfg.RegistrationHandler.Submit())
		r.Get("/shirt-sizes", cfg.RegistrationHandler.ListShirtSizes())
		r.Get("/registrations/summary", cfg.RegistrationHandler.Summary())
		r.Get("/registration-pricing", cfg.RegistrationHandler.Pricing())
		r.Post("/payments/webhook", cfg.PaymentHandler.Webhook())
		r.Get("/consent-form", cfg.ConsentHandler.GetConsentForm())
	})

	r.Route("/camp-admin", func(r chi.Router) {
		middleware.WithRequestTimeout(r)
		campadminapi.Mount(r, cfg.AdminAuth, cfg.RegRepo, cfg.CampRepo, cfg.RegSvc, cfg.BillSvc, cfg.PaySvc, cfg.AdminRec)
	})

	return r
}

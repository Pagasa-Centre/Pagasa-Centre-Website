package router

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"pagasacentre/backend/internal/adminlog"
	accomapi "pagasacentre/backend/internal/api/accommodation"
	campapi "pagasacentre/backend/internal/api/camp"
	campadminapi "pagasacentre/backend/internal/api/campadmin"
	consentapi "pagasacentre/backend/internal/api/consent"
	paymentapi "pagasacentre/backend/internal/api/payment"
	regapi "pagasacentre/backend/internal/api/registration"
	sermonapi "pagasacentre/backend/internal/api/sermon"
	"pagasacentre/backend/internal/billing"
	"pagasacentre/backend/internal/camp"
	"pagasacentre/backend/internal/middleware"
	"pagasacentre/backend/internal/payment"
	"pagasacentre/backend/internal/registration"
	"pagasacentre/backend/pkg/commonlibrary/render"
)

// Pinger is the database check behind /health. *pgxpool.Pool satisfies it.
type Pinger interface {
	Ping(ctx context.Context) error
}

type Config struct {
	AllowedOrigin string
	Commit        string
	DB            Pinger

	AdminAuth middleware.AuthConfig

	CampHandler          *campapi.Handler
	AccommodationHandler *accomapi.Handler
	RegistrationHandler  *regapi.Handler
	PaymentHandler       *paymentapi.Handler
	ConsentHandler       *consentapi.Handler
	SermonHandler        *sermonapi.Handler

	CampSvc  *camp.Service
	RegSvc   *registration.Service
	BillSvc  *billing.Service
	PaySvc   *payment.Service
	AdminRec *adminlog.Recorder
	AdminHub *adminlog.Hub
}

func New(cfg Config) http.Handler {
	r := chi.NewRouter()
	middleware.UseDefaults(r, cfg.AllowedOrigin)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		// 503 when the process is up but cannot serve, so a platform probe
		// stops sending traffic. The body stays a status string; the ping
		// error is logged, not returned.
		if cfg.DB == nil {
			render.Json(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "commit": cfg.Commit})
			return
		}
		pingCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := cfg.DB.Ping(pingCtx); err != nil {
			log.Printf("health: db ping: %v", err)
			render.Json(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "commit": cfg.Commit})
			return
		}
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
		r.Get("/sermons/latest", cfg.SermonHandler.GetLatest())
	})

	r.Route("/camp-admin", func(r chi.Router) {
		middleware.WithRequestTimeout(r)
		campadminapi.Mount(r, cfg.AdminAuth, cfg.RegSvc, cfg.CampSvc, cfg.BillSvc, cfg.PaySvc, cfg.AdminRec)
	})

	return r
}

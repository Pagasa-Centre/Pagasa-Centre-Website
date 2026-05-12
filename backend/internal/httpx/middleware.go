package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// UseDefaults installs the standard middleware stack on a chi router.
func UseDefaults(r chi.Router, allowedOrigin string) {
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60_000_000_000)) // 60s

	allowed := []string{allowedOrigin}
	if allowedOrigin == "*" {
		allowed = []string{"*"}
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowed,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "Stripe-Signature"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
}

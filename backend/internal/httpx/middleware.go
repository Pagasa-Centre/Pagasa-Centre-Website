package httpx

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// UseDefaults installs the standard middleware stack on a chi router.
//
// allowedOrigins may be a single origin, a comma-separated list (e.g.
// "https://pagasa-centre-website-dev.up.railway.app,http://localhost:3000"),
// or "*" to allow everything. Whitespace around entries is trimmed and empty
// entries are dropped.
func UseDefaults(r chi.Router, allowedOrigins string) {
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60_000_000_000)) // 60s

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   parseOrigins(allowedOrigins),
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "Stripe-Signature"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
}

func parseOrigins(raw string) []string {
	if raw == "" || raw == "*" {
		return []string{"*"}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
}

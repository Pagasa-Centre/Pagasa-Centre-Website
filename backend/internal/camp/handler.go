package camp

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"pagasacentre/backend/internal/httpx"
)

// Mount registers public read endpoints for camp config and prices.
func Mount(r chi.Router, repo *Repository) {
	r.Get("/camp", func(w http.ResponseWriter, req *http.Request) {
		cfg, err := repo.GetConfig(req.Context())
		if err != nil {
			httpx.WriteError(w, httpx.Internal(err.Error()))
			return
		}
		httpx.WriteJSON(w, http.StatusOK, cfg)
	})

	r.Get("/prices", func(w http.ResponseWriter, req *http.Request) {
		prices, err := repo.ListPrices(req.Context())
		if err != nil {
			httpx.WriteError(w, httpx.Internal(err.Error()))
			return
		}
		if prices == nil {
			prices = []Price{}
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"prices": prices})
	})
}

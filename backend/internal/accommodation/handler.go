package accommodation

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"pagasacentre/backend/internal/httpx"
)

// Mount registers public accommodation routes on r.
func Mount(r chi.Router, svc *Service) {
	r.Get("/accommodations", func(w http.ResponseWriter, req *http.Request) {
		list, err := svc.ListAvailability(req.Context())
		if err != nil {
			httpx.WriteError(w, httpx.Internal(err.Error()))
			return
		}
		if list == nil {
			list = []Availability{}
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"accommodations": list})
	})
}

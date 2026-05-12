package registration

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"pagasacentre/backend/internal/httpx"
)

// Mount registers the POST /registrations route.
func Mount(r chi.Router, svc *Service) {
	r.Post("/registrations", func(w http.ResponseWriter, req *http.Request) {
		var body SubmitRequest
		if err := httpx.DecodeJSON(req, &body); err != nil {
			httpx.WriteError(w, err)
			return
		}
		resp, err := svc.Submit(req.Context(), body)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, resp)
	})
}

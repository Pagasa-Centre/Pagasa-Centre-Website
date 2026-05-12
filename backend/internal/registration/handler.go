package registration

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"pagasacentre/backend/internal/httpx"
)

// Mount registers the registration-related routes.
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

	r.Get("/shirt-sizes", func(w http.ResponseWriter, req *http.Request) {
		sizes := ListShirtSizes()
		grouped := map[string][]ShirtSize{"adult": {}, "child": {}}
		for _, s := range sizes {
			grouped[s.Category] = append(grouped[s.Category], s)
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"sizes":           sizes,
			"by_category":     grouped,
			"not_applicable":  ShirtSizeNotApplicable,
		})
	})
}

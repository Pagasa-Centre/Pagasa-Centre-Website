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

	// Success-page summary lookup. Public, authenticated only by knowing the
	// one-time session_id or group_id — both random opaque tokens the payer
	// received via redirect. Returns minimal data (camper first/last names +
	// status). Never exposes phone numbers, accommodation choices, ages,
	// dietary info etc.
	r.Get("/registrations/summary", func(w http.ResponseWriter, req *http.Request) {
		sessionID := req.URL.Query().Get("session_id")
		groupID := req.URL.Query().Get("group_id")
		summary, err := svc.Summary(req.Context(), sessionID, groupID)
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		if summary == nil {
			httpx.WriteError(w, httpx.APIError{
				Code:    "not_found",
				Message: "no registration found for the supplied identifier",
			})
			return
		}
		httpx.WriteJSON(w, http.StatusOK, summary)
	})
}

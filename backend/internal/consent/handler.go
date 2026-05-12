package consent

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
)

// PDFPath is the location of the parental consent form, resolved relative to
// the process working directory. Exposed so tests can override it.
var PDFPath = filepath.Join("static", "parental-consent-form.pdf")

// Mount registers GET /consent-form.
func Mount(r chi.Router) {
	r.Get("/consent-form", func(w http.ResponseWriter, req *http.Request) {
		if _, err := os.Stat(PDFPath); err != nil {
			http.Error(w, "Consent form not yet available. Please contact the church office.", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="pc-summer-camp-2026-parental-consent.pdf"`)
		http.ServeFile(w, req, PDFPath)
	})
}

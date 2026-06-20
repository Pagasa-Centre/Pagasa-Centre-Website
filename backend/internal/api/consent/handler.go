package consent

import (
	"net/http"
	"os"
	"path/filepath"
)

// PDFPath is the location of the parental consent form, resolved relative to
// the process working directory. Exposed so tests can override it.
var PDFPath = filepath.Join("static", "parental-consent-form.pdf")

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) GetConsentForm() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat(PDFPath); err != nil {
			http.Error(w, "Consent form not yet available. Please contact the church office.", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="pc-summer-camp-2026-parental-consent.pdf"`)
		http.ServeFile(w, r, PDFPath)
	}
}

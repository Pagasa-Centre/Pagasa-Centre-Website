package admin

import (
	"log"
	"net/http"

	"pagasacentre/backend/internal/adminlog"
	"pagasacentre/backend/internal/registration/domain"
)

// Audit records an admin action (exported for api/campadmin).
func Audit(rec *adminlog.Recorder, r *http.Request, action string, groupID *string, summary string, meta any) {
	if rec == nil {
		return
	}
	actor := ActorFrom(r.Context())
	if _, err := rec.Record(r.Context(), actor, action, groupID, summary, meta); err != nil {
		log.Printf("admin audit: %v", err)
	}
}

func GroupSummary(g *domain.Group) string {
	if g == nil {
		return "a registration group"
	}
	return g.ContactFirstName + " " + g.ContactLastName
}

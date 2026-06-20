package admin

import (
	"log"
	"net/http"

	"pagasacentre/backend/internal/adminlog"
	"pagasacentre/backend/internal/registration"
)

func expectedVersion(v *int) int {
	if v == nil {
		return registration.SkipVersionCheck
	}
	return *v
}

func audit(rec *adminlog.Recorder, r *http.Request, action string, groupID *string, summary string, meta any) {
	if rec == nil {
		return
	}
	actor := ActorFrom(r.Context())
	if _, err := rec.Record(r.Context(), actor, action, groupID, summary, meta); err != nil {
		log.Printf("admin audit: %v", err)
	}
}

func groupSummary(g *registration.Group) string {
	if g == nil {
		return "a registration group"
	}
	return g.ContactFirstName + " " + g.ContactLastName
}

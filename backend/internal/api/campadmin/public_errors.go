package campadmin

import (
	"log"
	"net/http"

	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
)

// publicErrors logs each internal failure with its key and ref, and returns
// client-safe "code: message" strings in the same format as before.
func publicErrors(r *http.Request, errs map[string]error) map[string]string {
	out := make(map[string]string, len(errs))
	for id, err := range errs {
		pub, ref := commonerrors.Public(err)
		if ref != "" {
			log.Printf("%s %s: group %s: internal error ref=%s: %v", r.Method, r.URL.Path, id, ref, err)
		}
		out[id] = pub.Error()
	}
	return out
}

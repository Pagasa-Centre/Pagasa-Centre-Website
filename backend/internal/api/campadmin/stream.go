package campadmin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"pagasacentre/backend/internal/adminlog"
	"pagasacentre/backend/internal/api/campadmin/dto/mapper"
	"pagasacentre/backend/internal/auth"
	"pagasacentre/backend/pkg/commonlibrary/apperr"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
	"pagasacentre/backend/pkg/commonlibrary/render"
)

func HandleStream(authSvc *auth.Service, hub *adminlog.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if _, ok := authSvc.VerifyToken(token); !ok {
			commonerrors.WriteError(w, apperr.Unauthorized("unauthorized"))
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		events, unsub := hub.Subscribe()
		defer unsub()

		tick := time.NewTicker(25 * time.Second)
		defer tick.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case ev := <-events:
				data, err := json.Marshal(mapper.EventToResponse(ev))
				if err != nil {
					continue
				}
				if _, err := fmt.Fprintf(w, "event: changed\ndata: %s\n\n", data); err != nil {
					return
				}
				flusher.Flush()
			case <-tick.C:
				if _, err := fmt.Fprintf(w, ": ping\n\n"); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	}
}

type eventLister interface {
	ListEvents(ctx context.Context, limit int, beforeID int64) ([]adminlog.Event, error)
}

func listEvents(lister eventLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 50
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				limit = n
			}
		}
		var before int64
		if v := r.URL.Query().Get("before"); v != "" {
			before, _ = strconv.ParseInt(v, 10, 64)
		}
		events, err := lister.ListEvents(r.Context(), limit, before)
		if err != nil {
			commonerrors.WriteError(w, err)
			return
		}
		render.Json(w, http.StatusOK, map[string]any{"events": mapper.EventsToResponse(events)})
	}
}

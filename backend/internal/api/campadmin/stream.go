package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"pagasacentre/backend/internal/adminlog"
	"pagasacentre/backend/internal/middleware"
	commonerrors "pagasacentre/backend/pkg/commonlibrary/errors"
	"pagasacentre/backend/pkg/commonlibrary/render"
)

func HandleStream(auth middleware.AuthConfig, hub *adminlog.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if _, ok := middleware.VerifySessionToken(auth.SessionSecret, token); !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
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
				data, err := json.Marshal(ev)
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

func listEvents(rec *adminlog.Recorder) http.HandlerFunc {
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
		events, err := rec.List(r.Context(), limit, before)
		if err != nil {
			commonerrors.WriteError(w, commonerrors.Internal(err.Error()))
			return
		}
		if events == nil {
			events = []adminlog.Event{}
		}
		render.Json(w, http.StatusOK, map[string]any{"events": events})
	}
}

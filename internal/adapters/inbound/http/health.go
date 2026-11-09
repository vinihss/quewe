package http

import (
	"context"
	"net/http"
	"time"
)

// ReadyFunc reports whether the process can serve traffic.
//
// Guide rule (I5): /healthz is liveness (process alive) and /readyz is readiness
// (dependencies ready). Both are outside the versioned prefix and do not require
// client authentication.
type ReadyFunc func(ctx context.Context) error

func healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func readyz(ready ReadyFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := ready(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

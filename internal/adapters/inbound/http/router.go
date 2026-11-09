// Package http mounts the inbound HTTP adapter.
//
// Guide rule (I3): the version prefix is registered only once, here.
// Features register only their suffixes on the received ServeMux.
package http

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/vinihss/quewe/services/quewe/internal/adapters/inbound/http/middleware"
)

// Mount wraps the versioned API mux with the public endpoints and middleware.
func Mount(api *http.ServeMux, ready ReadyFunc) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", api))
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /readyz", readyz(ready))
	mux.Handle("GET /metrics", promhttp.Handler())

	return middleware.Chain(mux,
		middleware.RequestID,
		middleware.Recover,
		middleware.AccessLog,
	)
}

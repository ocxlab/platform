// Package runtimehttp provides operational endpoints for the API process.
// Process liveness is not order readiness or payment-provider availability.
package runtimehttp

import "net/http"

func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte("{\"status\":\"ok\",\"service\":\"ocp-api\"}\n"))
	})
	return mux
}

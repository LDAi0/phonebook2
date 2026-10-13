package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// NewRouter wires routes and middleware. ready is called by /readyz.
func NewRouter(log *slog.Logger, svc ContactService, ready func(context.Context) error) http.Handler {
	h := &handler{svc: svc, log: log}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := ready(ctx); err != nil {
			writeError(w, http.StatusServiceUnavailable, "not_ready", "database is unavailable", nil)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	mux.HandleFunc("POST /api/v1/contacts", h.create)
	mux.HandleFunc("GET /api/v1/contacts", h.list)
	mux.HandleFunc("GET /api/v1/contacts/{id}", h.get)
	mux.HandleFunc("PUT /api/v1/contacts/{id}", h.update)
	mux.HandleFunc("DELETE /api/v1/contacts/{id}", h.delete)

	return chain(mux, requestID, logging(log), recoverer(log))
}

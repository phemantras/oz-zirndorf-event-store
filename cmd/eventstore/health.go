package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

const (
	healthPath = "/healthz"
	// adminPath is the subtree the admin interface owns.
	adminPath = "/admin/"
	// publicAPIPath is the subtree the public API v1 owns (KON-1).
	publicAPIPath = "/v1/"
	// healthPingTimeout bounds the database ping so a hanging database
	// yields 503 instead of a stuck health check.
	healthPingTimeout = 2 * time.Second
)

// pinger checks whether the database is reachable. *pgxpool.Pool satisfies it.
type pinger interface {
	Ping(ctx context.Context) error
}

// newHealthHandler answers 200 when the database answers a ping within
// healthPingTimeout and 503 otherwise.
func newHealthHandler(db pinger, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), healthPingTimeout)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			logger.Error("health check failed: database unreachable", "error", err)
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

// routeHandlers are the interfaces the router mounts besides the health
// check.
type routeHandlers struct {
	admin  http.Handler
	public http.Handler
}

// newRouter returns the HTTP routes of the service: the health check, the
// admin interface below adminPath and the public API below publicAPIPath.
func newRouter(db pinger, logger *slog.Logger, handlers routeHandlers) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(http.MethodGet+" "+healthPath, newHealthHandler(db, logger))
	mux.Handle(adminPath, handlers.admin)
	mux.Handle(publicAPIPath, handlers.public)
	return mux
}

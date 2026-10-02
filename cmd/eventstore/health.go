package main

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	healthPath = "/healthz"
	// healthPingTimeout bounds the database ping so a hanging database
	// yields 503 instead of a stuck health check.
	healthPingTimeout = 2 * time.Second
)

// The X-Forwarded-For log on /healthz is temporary: it measures how Railway
// forwards client addresses (ENT-21) and is removed again after the
// measurement, at the latest with Story 1.3.
const (
	forwardedForHeader      = "X-Forwarded-For"
	healthRequestLogMessage = "health check request"
	forwardedForLogKey      = "x_forwarded_for"
	remoteAddrLogKey        = "remote_addr"
	// forwardedForSeparator joins repeated X-Forwarded-For header lines the
	// same way proxies join entries within one line.
	forwardedForSeparator = ", "
)

// pinger checks whether the database is reachable. *pgxpool.Pool satisfies it.
type pinger interface {
	Ping(ctx context.Context) error
}

// newHealthHandler answers 200 when the database answers a ping within
// healthPingTimeout and 503 otherwise. It logs the forwarding headers of
// every request (temporary, see forwardedForHeader).
func newHealthHandler(db pinger, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Info(healthRequestLogMessage,
			forwardedForLogKey, strings.Join(r.Header.Values(forwardedForHeader), forwardedForSeparator),
			remoteAddrLogKey, r.RemoteAddr,
		)

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

// newRouter returns the HTTP routes of the service.
func newRouter(db pinger, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(http.MethodGet+" "+healthPath, newHealthHandler(db, logger))
	return mux
}

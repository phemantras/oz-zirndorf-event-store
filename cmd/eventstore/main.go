// Command eventstore starts the OZ Zirndorf Event Store: it reads the
// configuration from the environment, applies the database migrations and
// only then starts the HTTP server.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // Europe/Berlin must resolve even without system zone data.

	"github.com/phemantras/oz-zirndorf-event-store/internal/adapter/postgres"
)

const (
	exitCodeFailure = 1
	// shutdownTimeout bounds how long running requests may finish after
	// SIGINT or SIGTERM.
	shutdownTimeout = 10 * time.Second
	// readHeaderTimeout protects the server against slow-header clients.
	readHeaderTimeout = 5 * time.Second
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	logger := newLogger(os.Stdout)

	err := run(ctx, logger, os.Getenv)
	stop()
	if err != nil {
		logger.Error("eventstore stopped with error", "error", err)
		os.Exit(exitCodeFailure)
	}
}

// newLogger returns a logger that writes JSON lines to out.
func newLogger(out io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(out, nil))
}

// run wires the service in startup order (configuration, database,
// migrations, HTTP server) and blocks until ctx is cancelled or a step fails.
func run(ctx context.Context, logger *slog.Logger, getenv func(string) string) error {
	cfg, err := loadConfig(getenv)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	applied, err := postgres.Migrate(ctx, pool)
	if err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	logger.Info("database migrations applied", "count", applied)

	listener, err := net.Listen("tcp", cfg.ListenAddress())
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.ListenAddress(), err)
	}
	return serve(ctx, newServer(pool, logger), listener, logger)
}

// newServer returns the HTTP server with all routes and timeouts.
func newServer(db pinger, logger *slog.Logger) *http.Server {
	return &http.Server{
		Handler:           newRouter(db, logger),
		ReadHeaderTimeout: readHeaderTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
}

// serve answers requests on listener until ctx is cancelled, then shuts the
// server down gracefully within shutdownTimeout.
func serve(ctx context.Context, server *http.Server, listener net.Listener, logger *slog.Logger) error {
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	logger.Info("http server listening", "address", listener.Addr().String())

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve http: %w", err)
	case <-ctx.Done():
	}

	logger.Info("shutting down http server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shut down http server: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve http: %w", err)
	}
	return nil
}
